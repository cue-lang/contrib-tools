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
	"unicode/utf8"

	"github.com/spf13/cobra"
)

func newRewriteCommitMsgCmd(c *Command) *cobra.Command {
	var message, messageFile string
	cmd := &cobra.Command{
		Use:   "rewrite-commit-msg [flags] <file>",
		Short: "Rewrite a commit message file, preserving trailers",
		Long: `Rewrite a commit message file, preserving trailers (Change-Id, Signed-off-by, etc.).

This command is designed to be used as a GIT_EDITOR when amending commits
non-interactively. It replaces the message body (everything above the trailers)
with the new message, provided inline via -m or read from a file via -F, while
preserving all existing trailers.

Prefer -F: the GIT_EDITOR value is parsed by the shell, so a message passed
inline via -m breaks as soon as it contains a quote character — as ordinary
prose regularly does.

The new message is inserted verbatim — no reflowing or rewrapping is applied —
so it must already be hard-wrapped at 72 columns. A message with longer lines
is rejected (non-zero exit) without modifying anything, which makes git abort
the amend; rewrap the message and retry. The summary (first) line, lines
containing a URL, "Fixes"/"Updates"/"For" reference lines, and indented quote
lines are exempt and may be arbitrarily long.

Usage as GIT_EDITOR:

    GIT_EDITOR="cueckoo rewrite-commit-msg -F /tmp/msg.txt" \
      git commit --amend

where /tmp/msg.txt contains the new summary and body (no trailers).

The file argument is the path to the commit message file, which is passed
automatically by git when this command is used as GIT_EDITOR.
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case message != "" && messageFile != "":
				return fmt.Errorf("-m and -F are mutually exclusive")
			case message == "" && messageFile == "":
				return fmt.Errorf("one of -m or -F is required")
			}
			if messageFile != "" {
				data, err := os.ReadFile(messageFile)
				if err != nil {
					return fmt.Errorf("reading message file: %w", err)
				}
				message = string(data)
			}
			return rewriteCommitMsg(args[0], message)
		},
	}
	cmd.Flags().StringVarP(&message, "m", "m", "", "new commit message (replaces everything above trailers)")
	cmd.Flags().StringVarP(&messageFile, "F", "F", "", "file containing the new commit message (replaces everything above trailers)")
	return cmd
}

// rewriteCommitMsg reads the commit message file at path, extracts the
// trailers using git interpret-trailers, replaces the body with newMessage
// inserted verbatim, and writes the result back. newMessage must already
// be hard-wrapped at commitBodyWidth columns (see checkCommitBodyWidth);
// otherwise an error is returned and the file is left unmodified.
func rewriteCommitMsg(path, newMessage string) error {
	if err := checkCommitBodyWidth(newMessage); err != nil {
		return err
	}

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

// commitBodyWidth is the column at which commit message bodies are
// hard-wrapped. See the "Commit Messages" section of the cueckoo
// common guidance.
const commitBodyWidth = 72

// checkCommitBodyWidth verifies that every line of msg fits within
// commitBodyWidth columns, and returns an error naming each offending
// line otherwise. It deliberately validates rather than reflows: any
// reformatting smart enough to preserve deliberate structure (lists,
// quoted output, aligned text) amounts to a full formatter, and
// formatting is the caller's job — the guidance already instructs
// authors to hard-wrap commit messages at 72 columns. The summary
// (first) line is not checked, and widthCheckExempt lines may be
// arbitrarily long.
func checkCommitBodyWidth(msg string) error {
	var long []string
	for i, line := range strings.Split(msg, "\n") {
		if i == 0 || widthCheckExempt(line) {
			continue
		}
		if n := utf8.RuneCountInString(line); n > commitBodyWidth {
			long = append(long, fmt.Sprintf("line %d (%d columns): %s", i+1, n, line))
		}
	}
	if long != nil {
		return fmt.Errorf("message is not hard-wrapped at %d columns; rewrap the offending lines and retry:\n%s",
			commitBodyWidth, strings.Join(long, "\n"))
	}
	return nil
}

// widthCheckExempt reports whether line is exempt from the
// commitBodyWidth check performed by checkCommitBodyWidth. Exempt are
// the lines the guidance requires to stay unsplit even beyond 72
// columns: "Fixes"/"Updates"/"For" reference lines and any line
// containing a URL — plus indented quote lines (a leading tab, or four
// or more spaces), which are preformatted blocks such as example
// commands.
func widthCheckExempt(line string) bool {
	if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
		return true
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "Fixes ") ||
		strings.HasPrefix(trimmed, "Updates ") ||
		strings.HasPrefix(trimmed, "For ") {
		return true
	}
	if strings.Contains(line, "://") {
		return true
	}
	return false
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
