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
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteCommitMsg(t *testing.T) {
	tests := []struct {
		name   string
		orig   string
		newMsg string
		want   string
	}{
		{
			name: "preserves Change-Id and Signed-off-by",
			orig: `cmd/foo: old summary

Old description of the change.

Signed-off-by: Alice <alice@example.com>
Change-Id: I1234567890abcdef1234567890abcdef12345678
`,
			newMsg: "cmd/foo: new summary\n\nNew description.",
			want: `cmd/foo: new summary

New description.

Signed-off-by: Alice <alice@example.com>
Change-Id: I1234567890abcdef1234567890abcdef12345678
`,
		},
		{
			name: "preserves trailers with no body",
			orig: `cmd/foo: old summary

Change-Id: I1234567890abcdef1234567890abcdef12345678
`,
			newMsg: "cmd/foo: new summary",
			want: `cmd/foo: new summary

Change-Id: I1234567890abcdef1234567890abcdef12345678
`,
		},
		{
			name: "no trailers",
			orig: `cmd/foo: old summary

Old description.
`,
			newMsg: "cmd/foo: new summary\n\nNew description.",
			want: `cmd/foo: new summary

New description.
`,
		},
		{
			name:   "preserves trailers when git comment lines are present",
			orig:   "cmd/foo: old summary\n\nOld description.\n\nSigned-off-by: Alice <alice@example.com>\nChange-Id: I1234567890abcdef1234567890abcdef12345678\n\n# Please enter the commit message.\n# Lines starting with '#' will be ignored.\n#\n# Changes to be committed:\n#   modified: foo.go\n",
			newMsg: "cmd/foo: new summary\n\nNew description.",
			want:   "cmd/foo: new summary\n\nNew description.\n\nSigned-off-by: Alice <alice@example.com>\nChange-Id: I1234567890abcdef1234567890abcdef12345678\n",
		},
		{
			name: "message with trailing newlines in new message",
			orig: `cmd/foo: old summary

Change-Id: Iabcdef
`,
			newMsg: "cmd/foo: new summary\n\n\n",
			want: `cmd/foo: new summary

Change-Id: Iabcdef
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
			if err := os.WriteFile(path, []byte(tt.orig), 0666); err != nil {
				t.Fatal(err)
			}
			if err := rewriteCommitMsg(path, tt.newMsg); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestWrapCommitBody(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "summary only",
			in: `
cmd/foo: short summary
`[1:],
			want: `
cmd/foo: short summary
`[1:],
		},
		{
			name: "short body unchanged",
			in: `
cmd/foo: summary

Short body.
`[1:],
			want: `
cmd/foo: summary

Short body.
`[1:],
		},
		{
			name: "long single-line paragraph is wrapped",
			in: `
cmd/foo: summary

This is a fairly long paragraph that should be wrapped because it exceeds the seventy-two column limit set by the guidance.
`[1:],
			want: `
cmd/foo: summary

This is a fairly long paragraph that should be wrapped because it
exceeds the seventy-two column limit set by the guidance.
`[1:],
		},
		{
			name: "pre-wrapped paragraph is re-flowed to same width",
			in: `
cmd/foo: summary

This is a fairly long paragraph
that should be wrapped because
it exceeds the seventy-two column
limit set by the guidance.
`[1:],
			want: `
cmd/foo: summary

This is a fairly long paragraph that should be wrapped because it
exceeds the seventy-two column limit set by the guidance.
`[1:],
		},
		{
			name: "two-line paragraph where first line just exceeds width is re-flowed",
			in: `
cmd/foo: summary

line that just barely goes over 72 chars in its length and then is followed by
almost nothing
`[1:],
			want: `
cmd/foo: summary

line that just barely goes over 72 chars in its length and then is
followed by almost nothing
`[1:],
		},
		{
			name: "URL line preserved verbatim",
			in: `
cmd/foo: summary

See the discussion at https://cuelang.org/issue/1234567890 for context.
More prose follows.
`[1:],
			want: `
cmd/foo: summary

See the discussion at https://cuelang.org/issue/1234567890 for context.
More prose follows.
`[1:],
		},
		{
			name: "Fixes line preserved even when paragraph wraps around it",
			in: `
cmd/foo: summary

A short body.

Fixes cue-lang/cue#4368.
`[1:],
			want: `
cmd/foo: summary

A short body.

Fixes cue-lang/cue#4368.
`[1:],
		},
		{
			name: "long Fixes line not wrapped",
			in: `
cmd/foo: summary

A body.

Fixes cue-lang/some-very-long-repo-name#123456789012345.
`[1:],
			want: `
cmd/foo: summary

A body.

Fixes cue-lang/some-very-long-repo-name#123456789012345.
`[1:],
		},
		{
			name: "multiple paragraphs",
			in: `
cmd/foo: summary

First paragraph that is somewhat long and will need to be wrapped at seventy-two columns.

Second paragraph also reasonably long and likewise requiring a wrap.
`[1:],
			want: `
cmd/foo: summary

First paragraph that is somewhat long and will need to be wrapped at
seventy-two columns.

Second paragraph also reasonably long and likewise requiring a wrap.
`[1:],
		},
		{
			name: "summary line never wrapped even if long",
			in: `
cmd/foo: a deliberately long summary line that exceeds seventy-two columns by quite a margin
`[1:],
			want: `
cmd/foo: a deliberately long summary line that exceeds seventy-two columns by quite a margin
`[1:],
		},
		{
			// TODO: a tab-indented quote should be preserved verbatim;
			// the leading tab is currently stripped and the line reflowed.
			name: "tab-indented quote preserved verbatim",
			in: `
cmd/foo: summary

Consider the command below:

	cue export --out yaml+indentSeq=false foo.cue

That emits compact YAML.
`[1:],
			want: `
cmd/foo: summary

Consider the command below:

cue export --out yaml+indentSeq=false foo.cue

That emits compact YAML.
`[1:],
		},
		{
			// TODO: a four-space-indented quote should be preserved
			// verbatim; the indentation is currently stripped.
			name: "four-space-indented quote preserved verbatim",
			in: `
cmd/foo: summary

Consider the command below:

    cue export --out yaml+indentSeq=false foo.cue

That emits compact YAML.
`[1:],
			want: `
cmd/foo: summary

Consider the command below:

cue export --out yaml+indentSeq=false foo.cue

That emits compact YAML.
`[1:],
		},
		{
			name: "indentation under four spaces is reflowed as prose",
			in: `
cmd/foo: summary

  this short line keeps under four spaces of indent
`[1:],
			want: `
cmd/foo: summary

this short line keeps under four spaces of indent
`[1:],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapCommitBody(tt.in)
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestExtractTrailers(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{
			name: "standard trailers",
			msg:  "summary\n\nbody\n\nSigned-off-by: Alice <a@b>\nChange-Id: Iabcdef\n",
			want: "Signed-off-by: Alice <a@b>\nChange-Id: Iabcdef",
		},
		{
			name: "no trailers",
			msg:  "summary\n\nbody\n",
			want: "",
		},
		{
			name: "trailer with trailing blank lines",
			msg:  "summary\n\nChange-Id: Iabcdef\n\n\n",
			want: "Change-Id: Iabcdef",
		},
		{
			name: "trailers with git comment lines after them",
			msg:  "summary\n\nbody\n\nSigned-off-by: Alice <a@b>\nChange-Id: Iabcdef\n\n# Please enter the commit message.\n# Lines starting with '#' will be ignored.\n#\n# Changes to be committed:\n#   modified: foo.go\n",
			want: "Signed-off-by: Alice <a@b>\nChange-Id: Iabcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTrailers(tt.msg)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
