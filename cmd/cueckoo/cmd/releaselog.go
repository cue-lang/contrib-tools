// Copyright 2023 The CUE Authors
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
	"fmt"
	"strings"

	"github.com/google/go-github/v51/github"
	"github.com/spf13/cobra"
)

// newUnityCmd creates a new unity command
func newReleaselogCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releaselog",
		Short: "create a GitHub release log",
		Long: `
Usage of releaselog:

	releaselog RANGE_START RANGE_END

releaselog generates a bullet list of commits similar to the GitHub change log
that is automatically created for a release in a repository that uses pull
requests. Because the CUE repository does not use PRs, the automatic change log
refuses to generate.

RANGE_START and RANGE_END are both required. The arguments are interpreted in a
similar way to:

    git log $RANGE_START..$RANGE_END

Like git log, commits are ordered reverse chronologically.
`,
		RunE: mkRunE(c, releaseLog),
	}
	return cmd
}

func releaseLog(cmd *Command, args []string) error {
	cmd.Flags()

	if len(args) != 2 {
		return fmt.Errorf("expected exactly two args which will be interpreted like git log $1..$2")
	}

	cfg, err := loadConfig(cmd.Context())
	if err != nil {
		return err
	}

	var commits []*github.RepositoryCommit
	opts := &github.ListOptions{
		Page: 1,
	}

	for {
		res, resp, err := cfg.githubClient.Repositories.CompareCommits(cmd.Context(), cfg.githubOwner, cfg.githubRepo, args[0], args[1], opts)
		// Check for any errors
		if err != nil {
			return fmt.Errorf("failed to compare commits: %w", err)
		}

		// Extract the commits
		commits = append(commits, res.Commits...)

		// Break if done
		if resp.LastPage == opts.Page {
			break
		}
		opts.Page++
	}

	for i := len(commits) - 1; i >= 0; i-- {
		v := commits[i]
		msg := v.Commit.GetMessage()
		lines := strings.Split(msg, "\n")
		fmt.Printf("* %s by @%s in %s\n", lines[0], v.GetAuthor().GetLogin(), v.GetSHA())
	}

	return nil
}
