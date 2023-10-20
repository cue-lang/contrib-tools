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
	"unicode"

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

	cfg, err := loadConfig(c.Context())
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

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
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

	// Extract the commit hash
	commitHash, err := run(ctx, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to establish commit hash: %w", err)
	}
	// Remove the trailing \n
	commitHash = strings.TrimSpace(commitHash)

	// Set the branch upstream as the first step. If subsequent commands fail
	// (they shouldn't but it can happen) we still need the upstream to have
	// been set.
	if _, err := run(ctx, "git", "branch", "--set-upstream-to", originBaseRef); err != nil {
		return err
	}

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
	log.Printf("rebased and squashed on %s", rebaseMsg)

	// TODO: fix up common commit message issues, especially when squashing, in Go code.

	// Add "Closes #PR as merged." Not that running this command will also end
	// up adding a Change-Id trailer if the user has git commit hooks set for
	// post-commit. This means that the Changed-ID will be visible in the commit
	// message when it comes to final human-edit of the commit message below.
	msg, err := run(ctx, "git", "log", "--pretty=%B", "-1")
	if err != nil {
		return err
	}
	msg, err = addClosesMsg(msg, prNumber, commitHash)
	if err != nil {
		return err
	}
	addClosesCmd := exec.CommandContext(context.Background(), "git", "commit", "--quiet", "--amend", "-F", "-")
	addClosesCmd.Stdin = strings.NewReader(msg)
	addClosesCmd.Stdout = os.Stdout
	addClosesCmd.Stderr = os.Stderr
	if err := addClosesCmd.Run(); err != nil {
		return err
	}

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
	return nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		if err, _ := err.(*exec.ExitError); err != nil {
			// Cmd.Output populates ExitError.Stderr.
			return "", fmt.Errorf("failed to run %q: %v:\n%s", cmd.Args, err, err.Stderr)
		}
		return "", fmt.Errorf("failed to run %q: %v", cmd.Args, err)
	}
	return string(out), err
}

// addClosesMsg adds the message to "Closes #pr as merged." to the commit message
// msg.  It respects trailers and leaves a newline at the end of the message.
// Like git it respects the last block of trailers.
//
// There is probably a nice package somewhere for parsing the git commit
// message into the constituent parts: pre-trailers, and trailers. For now
// we brute force it based on the logic described in "man
// git-interpret-trailers". If there are trailers we want to insert "Closes
// #PR as merged." as the last clear line before the trailers. If there are
// no trailers, it should be the final line in the commit message.
func addClosesMsg(msg string, pr int, commitHash string) (string, error) {
	// TODO: handle carriage returns?

	// Drop any trailing space. We will add back a \n at the end
	msg = strings.TrimRightFunc(msg, unicode.IsSpace)

	// Find the trailers if there are any. Note that this will include a
	// trailing \n which we will need to remove.
	trailersCmd := exec.Command("git", "interpret-trailers", "--only-trailers", "--only-input", "--unfold")
	trailersCmd.Stdin = strings.NewReader(msg)
	trailersCmd.Stderr = os.Stderr
	out, err := trailersCmd.Output()
	if err != nil {
		return "", err
	}
	trailersStr := strings.TrimSuffix(string(out), "\n")

	// Remove the trailers suffix and trim any trailing space
	msg = strings.TrimSuffix(msg, trailersStr)
	msg = strings.TrimRightFunc(msg, unicode.IsSpace)

	// Prepare the closes message
	closes := fmt.Sprintf("Closes #%d as merged as of commit %v.", pr, commitHash)

	// Add the closes message
	msg += "\n\n" + closes

	// Add the trailers back if there were any
	if trailersStr != "" {
		msg += "\n\n" + trailersStr
	}

	// Add the trailing \n
	msg += "\n"

	return msg, nil
}
