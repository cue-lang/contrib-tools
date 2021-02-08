// Copyright 2021 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v31/github"
	"github.com/spf13/cobra"
	"golang.org/x/build/gerrit"
)

const (
	flagChange flagName = "change"
)

// newRuntrybotCmd creates a new runtrybot command
func newRuntrybotCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtrybot",
		Short: "run the CUE trybot on the given CL",
		Long: `
Usage of runtrybot:

	runtrybot [-change] [ARGS...]

When run with no arguments, runtrybot derives a revision and change ID for each
pending commit in the current branch. If multiple pending commits are found,
you must either specify which commits to run, or specify HEAD to run the
trybots for all of them.

If the -change flag is provide, then the list of arguments is interpreted as
change numbers or IDs, and the latest revision from each of those changes is
assumed.

runtrybot requires GITHUB_USER and GITHUB_PAT environment variables to be set
with your GitHub username and personal acccess token respectively. The personal
access token only requires "public_repo" scope.
`,
		RunE: mkRunE(c, runtrybotDef),
	}
	cmd.Flags().Bool(string(flagChange), false, "interpret arguments as change numbers or IDs")
	return cmd
}

func runtrybotDef(cmd *Command, args []string) error {
	cfg := loadConfig()
	r := &runtrybot{
		cmd: cmd,
		cfg: cfg,
	}
	r.run()

	return nil
}

var (
	changeIDRegex = regexp.MustCompile(`(?m)^Change-Id: (.*)$`)
)

type runtrybot struct {
	cmd *Command
	cfg *config
}

func (r *runtrybot) run() {
	var changeIDs []revision
	args := make(map[string]bool)
	for _, a := range r.cmd.Flags().Args() {
		args[a] = true
	}
	if flagChange.Bool(r.cmd) {
		if len(args) == 0 {
			raise("must provide at least one change number of ID")
		}
		for a := range args {
			changeIDs = append(changeIDs, revision{
				changeID: a,
			})
		}
	} else {
		changeIDs = r.deriveChangeIDs(args)
	}
	r.triggerBuilds(changeIDs)
}

// deriveChangeIDs determines a list of change IDs for the supplied args
// (if there are any). See the runtrybot docs for an explanation of what
// is derived. Essentially however we try to follow the semantics of
// git-codereview:
//
// https://pkg.go.dev/golang.org/x/review/git-codereview
//
func (r *runtrybot) deriveChangeIDs(args map[string]bool) (res []revision) {
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
		commit, err := r.cfg.repo.CommitObject(plumbing.NewHash(h))
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
			changeID, err := getChangeIDFromCommitMsg(pc.Message)
			check(err, "failed to derive change ID: %v", err)

			res = append(res, revision{
				changeID: changeID,
				revision: pc.Hash.String(),
			})
		}
	} else {
		// We verify each of the arguments
	EachArg:
		for h := range args {
			// Resolve the arg and ensure we have a matching pending commit
			commit, err := r.cfg.repo.CommitObject(plumbing.NewHash(h))
			check(err, "failed to derive commit from %q: %v", h, err)
			for _, pc := range pendingCommits {
				if commit.Hash == pc.Hash {
					changeID, err := getChangeIDFromCommitMsg(pc.Message)
					check(err, "failed to derive change ID: %v", err)

					res = append(res, revision{
						changeID: changeID,
						revision: pc.Hash.String(),
					})
					continue EachArg
				}
			}
			raise("commit %v is not a pending commit", h)
		}
	}
	return
}

type revision struct {
	changeID string
	revision string
}

func (r *runtrybot) triggerBuilds(revs []revision) {
	errs := new(errorList)
	var wg sync.WaitGroup

	for i := range revs {
		rev := revs[i]
		wg.Add(1)
		go func() {
			var err error

			defer wg.Done()
			defer errs.Add(&err)
			defer recoverError(&err)

			r.triggerBuild(rev)
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

func (r *runtrybot) triggerBuild(rev revision) {
	in, err := r.cfg.gerritClient.GetChange(context.Background(), rev.changeID, gerrit.QueryChangesOpt{
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

	msg := fmt.Sprintf("trybot run for %v", ref)
	payload, err := buildRuntrybotPayload(msg, runtrybotPayload{
		ChangeID: rev.changeID,
		Ref:      ref,
		Commit:   commit,
	})
	errcheck(err)
	err = r.cfg.triggerRepositoryDispatch(payload)
	errcheck(err)
}

type runtrybotPayload struct {
	ChangeID string `json:"changeID"`
	Ref      string `json:"ref"`
	Commit   string `json:"commit"`
}

func buildRuntrybotPayload(msg string, payload runtrybotPayload) (github.DispatchRequestOptions, error) {
	return buildDispatchPayload(msg, eventTypeRuntrybot, payload)
}

func getChangeIDFromCommitMsg(msg string) (string, error) {
	matches := changeIDRegex.FindAllStringSubmatch(msg, -1)
	if len(matches) != 1 || len(matches[0]) != 2 {
		return "", fmt.Errorf("failed to find Change-Id in commit message")
	}
	return matches[0][1], nil
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
