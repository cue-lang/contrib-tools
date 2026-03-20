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
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newRewriteCommitMsgCmd(c *Command) *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "rewrite-commit-msg [flags] <file>",
		Short: "Rewrite a commit message file, preserving trailers",
		Long: `Rewrite a commit message file, preserving trailers (Change-Id, Signed-off-by, etc.).

This command is designed to be used as a GIT_EDITOR when amending commits
non-interactively. It replaces the message body (everything above the trailers)
with the new message provided via -m, while preserving all existing trailers.

Usage as GIT_EDITOR:

    GIT_EDITOR="cueckoo rewrite-commit-msg -m 'pkg/foo: new summary

    New description of the change.'" git commit --amend

The file argument is the path to the commit message file, which is passed
automatically by git when this command is used as GIT_EDITOR.
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if message == "" {
				return fmt.Errorf("the -m flag is required")
			}
			return rewriteCommitMsg(args[0], message)
		},
	}
	cmd.Flags().StringVarP(&message, "m", "m", "", "new commit message (replaces everything above trailers)")
	return cmd
}

// rewriteCommitMsg reads the commit message file at path, extracts the
// trailers, replaces the body with newMessage, and writes the result back.
func rewriteCommitMsg(path, newMessage string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading commit message file: %w", err)
	}

	trailers := extractTrailers(string(data))

	var b strings.Builder
	b.WriteString(strings.TrimRight(newMessage, "\n"))
	b.WriteString("\n")
	if len(trailers) > 0 {
		b.WriteString("\n")
		for _, t := range trailers {
			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	if err := os.WriteFile(path, []byte(b.String()), 0666); err != nil {
		return fmt.Errorf("writing commit message file: %w", err)
	}
	return nil
}

// extractTrailers extracts trailer lines from a commit message. Trailers
// are key-value pairs at the end of the message in the form "Key: Value",
// separated from the body by a blank line. This matches the git trailer
// convention used by Change-Id, Signed-off-by, etc.
func extractTrailers(msg string) []string {
	lines := strings.Split(msg, "\n")

	// Walk backwards from the end, skipping trailing empty lines,
	// then collect consecutive trailer lines.
	i := len(lines) - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}

	var trailers []string
	for i >= 0 {
		line := lines[i]
		if !isTrailerLine(line) {
			break
		}
		trailers = append(trailers, line)
		i--
	}

	// Reverse to restore original order.
	for l, r := 0, len(trailers)-1; l < r; l, r = l+1, r-1 {
		trailers[l], trailers[r] = trailers[r], trailers[l]
	}

	return trailers
}

// isTrailerLine checks if a line looks like a git trailer (Key: Value).
func isTrailerLine(line string) bool {
	key, _, ok := strings.Cut(line, ": ")
	if !ok {
		return false
	}
	// Key must be non-empty, contain no spaces, and start with a letter.
	if key == "" || strings.ContainsAny(key, " \t") {
		return false
	}
	if key[0] < 'A' || (key[0] > 'Z' && key[0] < 'a') || key[0] > 'z' {
		return false
	}
	return true
}
