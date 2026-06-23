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
// trailers using git interpret-trailers, replaces the body with newMessage
// (hard-wrapped at commitBodyWidth columns), and writes the result back.
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
	b.WriteString(strings.TrimRight(wrapCommitBody(newMessage), "\n"))
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

// wrapCommitBody hard-wraps the body of a commit message at
// commitBodyWidth columns. The first line (summary) is always
// preserved as-is. Blank lines are preserved as paragraph
// separators. Lines that must not be split — those starting with
// "Fixes ", "Updates ", or "For " (the issue-reference lines), and
// any line containing a URL — are emitted verbatim, even if they
// exceed the wrap width. Indented quote lines — those beginning with a
// tab or with four or more spaces — are likewise treated as
// preformatted blocks (e.g. example commands) and emitted verbatim with
// their leading whitespace intact. All other lines are treated as
// prose: consecutive non-preserve lines are joined into a paragraph and
// re-flowed to commitBodyWidth.
func wrapCommitBody(msg string) string {
	lines := strings.Split(msg, "\n")
	if len(lines) == 0 {
		return msg
	}
	out := make([]string, 0, len(lines))
	out = append(out, lines[0])

	var para []string
	flush := func() {
		if len(para) == 0 {
			return
		}
		out = append(out, wrapTokens(para, commitBodyWidth)...)
		para = para[:0]
	}

	for _, line := range lines[1:] {
		switch {
		case line == "":
			flush()
			out = append(out, "")
		case preserveCommitLine(line):
			flush()
			out = append(out, line)
		default:
			para = append(para, strings.Fields(line)...)
		}
	}
	flush()
	return strings.Join(out, "\n")
}

// preserveCommitLine reports whether line must be emitted verbatim
// by wrapCommitBody (no wrapping, no merging with neighbouring
// lines). See wrapCommitBody.
func preserveCommitLine(line string) bool {
	// Indented quote lines (a leading tab, or four or more spaces) are
	// preformatted blocks such as example commands; keep them verbatim.
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

// wrapTokens greedy-wraps tokens into lines of at most width
// columns. A single token longer than width is emitted on its own
// line and may exceed width.
func wrapTokens(tokens []string, width int) []string {
	if len(tokens) == 0 {
		return nil
	}
	var out []string
	var cur strings.Builder
	for _, tok := range tokens {
		if cur.Len() == 0 {
			cur.WriteString(tok)
			continue
		}
		if cur.Len()+1+len(tok) > width {
			out = append(out, cur.String())
			cur.Reset()
			cur.WriteString(tok)
			continue
		}
		cur.WriteByte(' ')
		cur.WriteString(tok)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
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
