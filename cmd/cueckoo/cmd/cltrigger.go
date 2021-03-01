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
	"golang.org/x/build/gerrit"
)

var (
	changeIDRegex = regexp.MustCompile(`(?m)^Change-Id: (.*)$`)
)

type builder func(payload clTriggerPayload) (github.DispatchRequestOptions, error)

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

// deriveChangeIDs determines a list of change IDs for the supplied args
// (if there are any). See the runtrybot docs for an explanation of what
// is derived. Essentially however we try to follow the semantics of
// git-codereview:
//
// https://pkg.go.dev/golang.org/x/review/git-codereview
//
func (c *cltrigger) deriveChangeIDs(args map[string]bool) (res []revision, err error) {
	// Work out the branchpoint
	var bp, bpStderr bytes.Buffer
	bpCmd := exec.Command("git", "codereview", "branchpoint")
	bpCmd.Stdout = &bp
	bpCmd.Stderr = &bpStderr
	if err := bpCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run [%v]: %v\n%s", strings.Join(bpCmd.Args, " "), err, bpStderr.String())
	}

	// Calculate the list of commits that are pending
	var commitList, clStderr bytes.Buffer
	logCmd := exec.Command("git", "log", "--pretty=format:%H", "--no-patch", fmt.Sprintf("%s..HEAD", bytes.TrimSpace(bp.Bytes())))
	logCmd.Stdout = &commitList
	logCmd.Stderr = &clStderr
	if err := logCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run [%v]: %v\n%s", strings.Join(logCmd.Args, " "), err, clStderr.String())
	}

	var pendingCommits []*object.Commit
	for _, line := range strings.Split(string(commitList.String()), "\n") {
		h := strings.TrimSpace(line)
		commit, err := c.cfg.repo.CommitObject(plumbing.NewHash(h))
		if err != nil {
			return nil, fmt.Errorf("failed to derive commit from %q: %v", h, err)
		}
		pendingCommits = append(pendingCommits, commit)
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
	if args["HEAD"] || len(args) == 0 && len(pendingCommits) == 1 {
		// Verify all

		for _, pc := range pendingCommits {
			changeID, err := getChangeIDFromCommitMsg(pc.Message)
			if err != nil {
				return nil, fmt.Errorf("failed to derive change ID: %v", err)
			}

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
			commit, err := c.cfg.repo.CommitObject(plumbing.NewHash(h))
			if err != nil {
				return nil, fmt.Errorf("failed to derive commit from %q: %v", h, err)
			}
			for _, pc := range pendingCommits {
				if commit.Hash == pc.Hash {
					changeID, err := getChangeIDFromCommitMsg(pc.Message)
					if err != nil {
						return nil, fmt.Errorf("failed to derive change ID: %v", err)
					}

					res = append(res, revision{
						changeID: changeID,
						revision: pc.Hash.String(),
					})
					continue EachArg
				}
			}
			return nil, fmt.Errorf("commit %v is not a pending commit", h)
		}
	}
	return
}

type revision struct {
	changeID string
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
		return fmt.Errorf("failed to get current revision information: %v", err)
	}

	var ref string
	var commit string
	if rev.revision != "" {
		ri, ok := in.Revisions[rev.revision]
		if !ok {
			return fmt.Errorf("change %v does not know about revision %v; did you forget to run git codereview mail?", rev.changeID, rev.revision)
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

	payload, err := c.builder(clTriggerPayload{
		ChangeID: rev.changeID,
		Ref:      ref,
		Commit:   commit,
	})
	if err != nil {
		return err
	}
	return c.cfg.triggerRepositoryDispatch(payload)
}

type clTriggerPayload struct {
	ChangeID string `json:"changeID"`
	Ref      string `json:"ref"`
	Commit   string `json:"commit"`
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
