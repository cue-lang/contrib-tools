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
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/build/gerrit"
)

var (
	changeIDRegex = regexp.MustCompile(`(?m)^Change-Id: (.*)$`)
)

type builder func(payload clTriggerPayload) error

type cltrigger struct {
	cmd     *Command
	cfg     *config
	builder builder
}

func newCLTrigger(cmd *Command, cfg *config, b builder) *cltrigger {
	return &cltrigger{
		cmd:     cmd,
		cfg:     cfg,
		builder: b,
	}
}

func (c *cltrigger) run() (err error) {
	var changeIDs []revision
	args := make(map[string]bool)
	for _, a := range c.cmd.Flags().Args() {
		args[a] = true
	}
	if flagChange.Bool(c.cmd) {
		if len(args) == 0 {
			return fmt.Errorf("must provide at least one change number of ID")
		}
		for a := range args {
			changeIDs = append(changeIDs, revision{
				changeID: a,
			})
		}
	} else {
		changeIDs, err = c.deriveChangeIDs(args)
		if err != nil {
			return err
		}
	}
	return c.triggerBuilds(changeIDs)
}

// deriveChangeIDs determines a list of change IDs for the supplied args (if
// there are any). See the trybot docs for an explanation of what is derived.
// Essentially however we try to follow the semantics of git-codereview:
//
// https://pkg.go.dev/golang.org/x/review/git-codereview
func (c *cltrigger) deriveChangeIDs(args map[string]bool) (res []revision, err error) {
	ctx := context.TODO()
	// Work out the branchpoint
	bp, err := run(ctx, "git", "codereview", "branchpoint")
	if err != nil {
		return nil, err
	}

	// Calculate the list of commits that are pending
	pendingCommits, err := resolveCommits(ctx, fmt.Sprintf("%s..HEAD", strings.TrimSpace(bp)))
	if err != nil {
		return nil, err
	}

	if len(pendingCommits) == 0 {
		return nil, fmt.Errorf("no pending commits")
	}
	if args["HEAD"] && len(args) > 1 {
		return nil, fmt.Errorf("HEAD can only be supplied as an argument by itself")
	}
	if !args["HEAD"] && len(pendingCommits) > 1 && len(args) == 0 {
		return nil, fmt.Errorf("must specify commits as arguments or use HEAD for everything")
	}
	addRevision := func(pc commit) error {
		changeID, err := getChangeIDFromCommitMsg(pc.body)
		if err != nil {
			return fmt.Errorf("failed to derive change ID: %v", err)
		}
		// If HEAD is tracking an origin remote branch,
		// make the changeID include the project name and target branch,
		// which will make the changeID string be an unique identifier.
		// See [revision.changeID].
		targetBranch, _ := run(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD@{u}")
		targetBranch = strings.TrimSpace(targetBranch)             // no trailing newline
		targetBranch = strings.TrimPrefix(targetBranch, "origin/") // no remote name prefix
		if targetBranch != "" {
			changeID = url.PathEscape(
				c.cfg.githubOwner + "/" + c.cfg.githubRepo +
					"~" +
					targetBranch +
					"~" +
					changeID)
		}

		res = append(res, revision{
			changeID: changeID,
			revision: pc.hash,
		})
		return nil
	}
	if args["HEAD"] || len(args) == 0 && len(pendingCommits) == 1 {
		// Verify all
		for _, pc := range pendingCommits {
			if err := addRevision(pc); err != nil {
				return nil, err
			}
		}
	} else {
		// To unique the list of commits for which we submit requests
		seen := make(map[string]bool)

		// We verify each of the arguments
	EachArg:
		for h := range args {
			// Resolve the arg and ensure we have a matching pending commit
			// and ensure we have a single one
			commits, err := resolveCommits(ctx, h)
			if err != nil || len(commits) != 1 {
				return nil, fmt.Errorf("failed to resolve revision %q", h)
			}
			commit := commits[0]
			if seen[commit.hash] {
				continue
			}
			seen[commit.hash] = true
			for _, pc := range pendingCommits {
				if commit.hash == pc.hash {
					if err := addRevision(pc); err != nil {
						return nil, err
					}
					continue EachArg
				}
			}
			return nil, fmt.Errorf("commit %v is not a pending commit", h)
		}
	}
	return
}

type commit struct {
	hash string
	body string
}

func resolveCommits(ctx context.Context, args ...string) ([]commit, error) {

	// Log a stream of commits, separated by NUL, with a single space
	// separating the commit hash and commit message body
	cmd := append([]string{"git", "log", "-z", "--pretty=format:%H %B", "--no-patch"}, args...)
	commitStream, err := run(ctx, cmd[0], cmd[1:]...)
	if err != nil {
		return nil, err
	}

	var commits []commit

	// The results are NUL-separated thanks to -z
	commitList := strings.Split(commitStream, "\x00")

	for _, commitBlob := range commitList {
		parts := strings.SplitN(commitBlob, " ", 2)
		commits = append(commits, commit{
			hash: parts[0],
			body: parts[1],
		})
	}

	return commits, nil
}

type revision struct {
	// changeID uniquely identifies a CL, per the documentation at
	// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id.
	//
	// Usually, it will be one of:
	//
	// 1) 12345, a CL number
	// 2) Ideadbeef123, from the Change-Id commit trailer
	// 3) project~branch~Ideadbeef123, in case a Change-Id is ambiguous due to
	//    backport cherry-pick CLs
	//
	// When deriving change IDs, we will always use the third form,
	// as it is the only one which cannot result in ambiguous identifiers.
	// However, the command-line UI accepts the three forms as direct arguments.
	changeID string

	// revision is a commit hash; when empty, we use changeID's latest patchset,
	// also known as its current revision.
	revision string
}

func (c *cltrigger) triggerBuilds(revs []revision) error {
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
			err = c.triggerBuild(rev)
		}()
	}

	wg.Wait()
	if len(errs.errs) > 0 {
		var msgs []string
		for _, e := range errs.errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf(strings.Join(msgs, "\n"))
	}
	return nil
}

func (c *cltrigger) triggerBuild(rev revision) error {
	in, err := c.cfg.gerritClient.GetChange(context.Background(), rev.changeID, gerrit.QueryChangesOpt{
		Fields: []string{"ALL_REVISIONS"},
	})
	if err != nil {
		// Note that this may be a "change not found" error when the changeID is
		// an ambiguous identifier. See [revision.changeID].
		return fmt.Errorf("failed to get current revision information: %v", err)
	}

	commit := rev.revision
	if commit == "" {
		// fall back to the current/latest revision, also a commit hash
		commit = in.CurrentRevision
	}
	revision, ok := in.Revisions[commit]
	if !ok {
		return fmt.Errorf("change %q does not know about revision %q; did you forget to run git codereview mail?", rev.changeID, commit)
	}

	return c.builder(clTriggerPayload{
		// rev.changeID may be in the unique "project~branch~change_id" form,
		// and we can't use that form for the workflow trigger payload
		// as tildes are not allowed in
		// Use the change ID alone, without any tildes.
		ChangeID: in.ChangeID,

		Ref:    revision.Ref,
		Commit: commit,
		Branch: in.Branch,
	})
}

type clTriggerPayload struct {
	ChangeID string `json:"changeID"`
	Ref      string `json:"ref"`
	Commit   string `json:"commit"`
	Branch   string `json:"branch"`
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
