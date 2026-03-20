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
	"os/exec"
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
// trailers using git interpret-trailers, replaces the body with newMessage,
// and writes the result back.
func rewriteCommitMsg(path, newMessage string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading commit message file: %w", err)
	}

	trailers, err := extractTrailers(string(data))
	if err != nil {
		return fmt.Errorf("extracting trailers: %w", err)
	}

	var b strings.Builder
	b.WriteString(strings.TrimRight(newMessage, "\n"))
	b.WriteString("\n")
	if trailers != "" {
		b.WriteString("\n")
		b.WriteString(trailers)
		if !strings.HasSuffix(trailers, "\n") {
			b.WriteString("\n")
		}
	}

	if err := os.WriteFile(path, []byte(b.String()), 0666); err != nil {
		return fmt.Errorf("writing commit message file: %w", err)
	}
	return nil
}

// extractTrailers uses git interpret-trailers --parse to extract trailer
// lines from a commit message. This correctly handles the git trailer
// format including Change-Id, Signed-off-by, etc.
func extractTrailers(msg string) (string, error) {
	cmd := exec.Command("git", "interpret-trailers", "--parse")
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git interpret-trailers: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
