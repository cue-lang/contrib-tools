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
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	flagUpdate flagName = "update"
)

// newImportPRCmd creates a new importpr command
func newImportPRCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "importpr",
		Short: "Import GitHub PRs to Gerrit",
		RunE:  mkRunE(c, importPRDef),
	}
	cmd.Flags().Bool(string(flagUpdate), false, "rebase against the tip of the target branch")
	return cmd
}

func importPRDef(c *Command, args []string) error {
	log.SetPrefix("[importpr] ")
	log.SetFlags(0) // no timestamps, as they aren't very useful

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if len(args) != 1 {
		return fmt.Errorf("expected a single PR number")
	}

	prNumber, err := strconv.Atoi(args[0])

	if err != nil || prNumber <= 0 {
		return fmt.Errorf("%q is not a valid number", prNumber)
	}

	log.Printf("using github remote URL %q", cfg.githubURL)

	branchName := fmt.Sprintf("importpr-%d", prNumber)

	// TODO: Note that mainErr's ctx is not wired here.
	// We should wire it up and use it for e.g. FetchContext.
	// For now, use a hard-coded timeout of 10s.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pr, _, err := cfg.githubClient.PullRequests.Get(context.Background(), cfg.githubOwner, cfg.githubRepo, prNumber)
	if err != nil {
		return fmt.Errorf("could not get github PR: %v", err)
	}
	baseRef := pr.GetBase().GetRef()
	if baseRef == "" {
		return fmt.Errorf("PR seems to have an empty base branch?")
	}

	// If the branch already exists, refuse to continue.
	if out, err := run(ctx,
		"git", "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branchName),
	); err == nil {
		return fmt.Errorf("branch %q already exists; delete it to start over", branchName)
	} else if len(out) == 0 {
		// An error without output means the branch does not exist.
	} else {
		return err // something else went wrong
	}

	// TODO: note that we assume that the upstream github remote is "origin".
	// We need to use a remote name in --set-upstream-to, so githubURL isn't enough.
	// If others have git setups where the remotes are named differently,
	// we can figure out a way to remove this assumption.
	originBaseRef := "origin/" + baseRef

	// Fetch the PR HEAD and place it in a new branch, then switch to it.
	if _, err := run(ctx,
		"git", "fetch", "--quiet", cfg.githubURL,
		fmt.Sprintf("pull/%d/head:%s", prNumber, branchName),
	); err != nil {
		return err
	}
	if _, err := run(ctx, "git", "switch", "--quiet", branchName); err != nil {
		return err
	}
	log.Printf("fetched PR into branch %q", branchName)

	// Fetch the latest baseRef in order that we can rebase against it.
	//
	// In the default case we do not try to incorporate new commits from the
	// target branch. That is, we simply use the rebase in order to squash the
	// commits in the PR. The rebase happens against the merge-base with respect
	// to baseRef.
	//
	// When the --update flag is passed, we perform the same rebase (to squash
	// commits) but against the tip of the target branch instead of the merge
	// base.
	if _, err := run(ctx, "git", "fetch", "--quiet", cfg.githubURL, baseRef); err != nil {
		return err
	}
	rebaseMsg := "tip of target branch"
	rebasePoint := "FETCH_HEAD"
	if !flagUpdate.Bool(c) {
		// We need to work out the mergebase
		out, err := run(ctx, "git", "merge-base", originBaseRef, branchName)
		if err != nil {
			return fmt.Errorf("failed to determine merge base %w", err)
		}
		rebaseMsg = "existing merge-base"
		rebasePoint = strings.TrimSpace(out)
	}
	if _, err := run(ctx, "git",
		"-c", "core.editor=cat",
		"-c", `sequence.editor=sed -i -e '2,$s/^pick/squash/'`,
		"rebase", "--interactive", rebasePoint,
	); err != nil {
		return err
	}
	if _, err := run(ctx, "git", "branch", "--set-upstream-to", originBaseRef); err != nil {
		return err
	}
	log.Printf("rebased and squashed on %s", rebaseMsg)

	// TODO: fix up common commit message issues, especially when squashing, in Go code.
	// TODO: automate adding "Closes #PR as merged.".

	// TODO: add a header (Change-Id or GitOrigin-RevId? see
	// https://cue-review.googlesource.com/c/cue/+/9781) to ensure that we don't
	// import the same PR multiple times into different CLs.

	// Amend the squashed commit message manually.
	// More often than not, we'll want to tweak commit messages to follow
	// https://github.com/cue-lang/cue/blob/HEAD/doc/contribute.md#good-commit-messages.
	// Moreover, if we squashed commits, a human needs to merge or discard their messages.
	// Note that we forward stdin/out/err for terminal editors like vim.
	// TODO: also add the PR title and description above the commit messages if
	// we squashed, because some people put the info there.
	log.Printf("opening editor to fix up commit message...")
	editCmd := exec.CommandContext(context.Background(), "git", "commit", "--quiet", "--amend")
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	if err := editCmd.Run(); err != nil {
		return err
	}

	log.Printf("When you're happy with the commit, run: git-codereview mail")
	log.Printf("Remember to ensure that the commit contains:")
	log.Printf("\tFixes #N. (if it fixes an open issue)")
	log.Printf("\tCloses #%d as merged.", prNumber)
	return nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, out)
	}
	return string(out), err
}
