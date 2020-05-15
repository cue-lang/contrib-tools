// runtrybot triggers GitHub actions builds for Gerrit CLs
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/cue-sh/tools/internal/codereviewcfg"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v31/github"
	"golang.org/x/build/gerrit"
)

var (
	changeIDRegex = regexp.MustCompile(`(?m)^Change-Id: (.*)$`)
)

type runner struct {
	github *github.Client
	gerrit *gerrit.Client
	owner  string
	repo   string
}

func mainerr() (err error) {
	defer handleKnown(&err)

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return flagErr(err.Error())
	}
	r := &runner{}
	r.run()

	return nil
}

func (r *runner) run() {
	rep, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	check(err, "failed to find git repository: %v", err)

	wt, err := rep.Worktree()
	check(err, "failed to get worktree: %v", err)

	cfg, err := codereviewcfg.Config(wt.Filesystem.Root())
	check(err, "failed to load codereview config: %v", err)

	gerritURL := cfg["gerrit"]
	if gerritURL == "" {
		raise("missing Gerrit server in codereview config")
	}
	githubURL := cfg["github"]
	if githubURL == "" {
		raise("missing GitHub server in codereview config")
	}
	gerritServer, err := codereviewcfg.GerritURLToServer(gerritURL)
	check(err, "failed to derived Gerrit server from %v: %v", gerritURL, err)

	r.owner, r.repo, err = codereviewcfg.GithubURLToParts(githubURL)
	check(err, "failed to derive GitHub owner and repo from %v: %v", githubURL, err)

	r.gerrit = gerrit.NewClient(gerritServer, gerrit.NoAuth)

	auth := github.BasicAuthTransport{
		Username: os.Getenv("GITHUB_USER"),
		Password: os.Getenv("GITHUB_PAT"),
	}
	r.github = github.NewClient(auth.Client())

	var changeIDs []revision
	args := make(map[string]bool)
	for _, a := range flagSet.Args() {
		args[a] = true
	}
	if *fChange {
		if len(args) == 0 {
			raise("must provide at least one change number of ID")
		}
		for a := range args {
			changeIDs = append(changeIDs, revision{
				changeID: a,
			})
		}
	} else {
		// Work out the branchpoint
		var bp, bpStderr bytes.Buffer
		bpCmd := exec.Command("git", "codereview", "branchpoint")
		bpCmd.Stdout = &bp
		bpCmd.Stderr = &bpStderr
		err := bpCmd.Run()
		check(err, "failed to run [%v]: %v\n%s", strings.Join(bpCmd.Args, " "), err, bpStderr.String())

		// Calculate the list of commits that are pending
		var commitList, clStderr bytes.Buffer
		logCmd := exec.Command("git", "log", "--pretty=format:%H", "--no-patch", fmt.Sprintf("%s..HEAD", bytes.TrimSpace(bp.Bytes())))
		logCmd.Stdout = &commitList
		logCmd.Stderr = &clStderr
		err = logCmd.Run()
		check(err, "failed to run [%v]: %v\n%s", strings.Join(logCmd.Args, " "), err, clStderr.String())

		var pendingCommits []*object.Commit
		for _, line := range strings.Split(string(commitList.String()), "\n") {
			h := strings.TrimSpace(line)
			commit, err := rep.CommitObject(plumbing.NewHash(h))
			check(err, "failed to derive commit from %q: %v", h, err)
			pendingCommits = append(pendingCommits, commit)
		}

		if len(pendingCommits) == 0 {
			raise("no pending commits")
		}
		if args["HEAD"] && len(args) > 1 {
			raise("HEAD can only be supplied as an argument by itself")
		}
		if !args["HEAD"] && len(pendingCommits) > 1 && len(args) == 0 {
			raise("must specify commits as arguments or use HEAD for everything")
		}
		if args["HEAD"] || len(args) == 0 && len(pendingCommits) == 1 {
			// Verify all

			for _, pc := range pendingCommits {
				changeID, err := getChangeIdFromCommitMsg(pc.Message)
				check(err, "failed to derive change ID: %v", err)

				changeIDs = append(changeIDs, revision{
					changeID: changeID,
					revision: pc.Hash.String(),
				})
			}
		} else {
			// We verify each of the arguments
		EachArg:
			for h := range args {
				// Resolve the arg and ensure we have a matching pending commit
				commit, err := rep.CommitObject(plumbing.NewHash(h))
				check(err, "failed to derive commit from %q: %v", h, err)
				for _, pc := range pendingCommits {
					if commit.Hash == pc.Hash {
						changeID, err := getChangeIdFromCommitMsg(pc.Message)
						check(err, "failed to derive change ID: %v", err)

						changeIDs = append(changeIDs, revision{
							changeID: changeID,
							revision: pc.Hash.String(),
						})
						continue EachArg
					}
				}
				raise("commit %v is not a pending commit", h)
			}
		}
	}

	r.triggerBuilds(changeIDs)
}

type revision struct {
	changeID string
	revision string
}

func (r *runner) triggerBuilds(revs []revision) {
	errs := new(errorList)
	var wg sync.WaitGroup

	for i := range revs {
		rev := revs[i]
		wg.Add(1)
		go func() {
			var err error

			defer wg.Done()
			defer errs.Add(&err)
			defer handleKnown(&err)

			in, err := r.gerrit.GetChange(context.Background(), rev.changeID, gerrit.QueryChangesOpt{
				Fields: []string{"ALL_REVISIONS"},
			})
			check(err, "failed to get current revision information: %v", err)

			var ref string
			var commit string
			if rev.revision != "" {
				ri, ok := in.Revisions[rev.revision]
				if !ok {
					raise("change %v does not know about revision %v; did you forget to run git codereview mail?", rev.changeID, rev.revision)
				}
				ref = ri.Ref
				commit = rev.revision
			} else {
				// find the latest ref
				type revInfoPair struct {
					rev string
					ri  gerrit.RevisionInfo
				}
				var revInfoPairs []revInfoPair
				for rev, ri := range in.Revisions {
					revInfoPairs = append(revInfoPairs, revInfoPair{
						rev: rev,
						ri:  ri,
					})
				}
				sort.Slice(revInfoPairs, func(i, j int) bool {
					return revInfoPairs[i].ri.PatchSetNumber < revInfoPairs[j].ri.PatchSetNumber
				})
				ref = revInfoPairs[len(revInfoPairs)-1].ri.Ref
				commit = revInfoPairs[len(revInfoPairs)-1].rev
			}

			payload, err := buildClientPayload(rev.changeID, ref, commit)
			check(err, "failed to build client payload: %v", err)

			dispatch := github.DispatchRequestOptions{
				EventType:     fmt.Sprintf("Build for %v", ref),
				ClientPayload: payload,
			}
			_, resp, err := r.github.Repositories.Dispatch(context.Background(), r.owner, r.repo, dispatch)
			check(err, "failed to dispatch build event: %v", err)
			if resp.StatusCode/100 != 2 {
				check(fmt.Errorf("dispatch build event did not succeed; status code %v", resp.StatusCode), "")
			}
		}()
	}

	wg.Wait()
	if len(errs.errs) > 0 {
		var msgs []string
		for _, e := range errs.errs {
			msgs = append(msgs, e.Error())
		}
		raise(strings.Join(msgs, "\n"))
	}
}

func buildClientPayload(changeId, ref, commit string) (*json.RawMessage, error) {
	var toEnc = struct {
		ChangeID string `json:"changeID"`
		Ref      string `json:"ref"`
		Commit   string `json:"commit"`
	}{
		ChangeID: changeId,
		Ref:      ref,
		Commit:   commit,
	}
	byts, err := json.Marshal(toEnc)
	rm := json.RawMessage(byts)
	return &rm, err
}

func getChangeIdFromCommitMsg(msg string) (string, error) {
	matches := changeIDRegex.FindAllStringSubmatch(msg, -1)
	if len(matches) != 1 || len(matches[0]) != 2 {
		return "", fmt.Errorf("failed to find Change-Id in commit message")
	}
	return matches[0][1], nil
}

type knownError struct{ error }

func handleKnown(err *error) {
	switch r := recover().(type) {
	case nil:
	case knownError:
		*err = r
	default:
		panic(r)
	}
}

func raise(format string, args ...interface{}) {
	panic(knownError{fmt.Errorf(format, args...)})
}

func check(err error, format string, args ...interface{}) {
	if err != nil {
		if format != "" {
			err = fmt.Errorf(format, args...)
		}
		panic(knownError{err})
	}
}

type errorList struct {
	mu   sync.Mutex
	errs []error
}

func (e *errorList) Add(err *error) {
	if *err == nil {
		return
	}
	e.mu.Lock()
	e.errs = append(e.errs, *err)
	e.mu.Unlock()
}

func (e *errorList) Error() string {
	panic("do not use; inspect list of errors")
}

func (e *errorList) Err() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e
}
