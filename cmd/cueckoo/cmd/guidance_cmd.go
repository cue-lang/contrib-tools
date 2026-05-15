// Copyright 2026 The CUE Authors
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

	"github.com/spf13/cobra"
)

func newGuidanceCmd(c *Command) *cobra.Command {
	var hashOnly, sessionStart bool
	cmd := &cobra.Command{
		Use:   "guidance",
		Short: "Print the common guidance for CUE project repos",
		Long: `Print the common guidance for CUE project repos.

The default output matches what the cueckoo MCP server's guidance tool
returns: the guidance body wrapped in BEGIN/END markers (the BEGIN line
includes the current guidance hash).

With --hash, only the hex hash of the current guidance is printed.

With --session-start, the current hash and an explicit load/reload
protocol are printed. This form is intended for use in Claude Code
SessionStart hooks that inject the current state into Claude's context
on every new or resumed session.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			switch {
			case hashOnly && sessionStart:
				return fmt.Errorf("--hash and --session-start are mutually exclusive")
			case hashOnly:
				_, err := out.Write([]byte(commonGuidanceHash + "\n"))
				return err
			case sessionStart:
				_, err := out.Write([]byte(sessionStartPrompt()))
				return err
			default:
				_, err := out.Write([]byte(formattedGuidance()))
				return err
			}
		},
	}
	cmd.Flags().BoolVar(&hashOnly, "hash", false, "print only the hex sha256 hash of the guidance")
	cmd.Flags().BoolVar(&sessionStart, "session-start", false, "print the current hash plus a load/reload protocol for a Claude Code SessionStart hook")
	return cmd
}
