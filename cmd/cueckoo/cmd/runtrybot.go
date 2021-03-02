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
	"fmt"

	"github.com/google/go-github/v31/github"
	"github.com/spf13/cobra"
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

	runtrybot [--change] [ARGS...]

When run with no arguments, runtrybot derives a revision and change ID for each
pending commit in the current branch. If multiple pending commits are found,
you must either specify which commits to run, or specify HEAD to run the
trybots for all of them.

If the --change flag is provide, then the list of arguments is interpreted as
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
	cfg, err := loadConfig(targetGitHub)
	if err != nil {
		return err
	}
	r := newCLTrigger(cmd, cfg, buildRunTryBotPayload)
	return r.run()
}

func buildRunTryBotPayload(payload clTriggerPayload) (github.DispatchRequestOptions, error) {
	msg := fmt.Sprintf("trybot run for %v", payload.Ref)
	return buildDispatchPayload(msg, eventTypeRuntrybot, payload)
}
