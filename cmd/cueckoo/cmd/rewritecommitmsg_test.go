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
	"strings"
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
			name: "numbered list inserted verbatim",
			orig: `cmd/foo: old summary

Old description.

Change-Id: Iabcdef
`,
			newMsg: `cmd/foo: new summary

The steps:

1. First item of the list, with a continuation line that is
   indented to line up under the item text.
2. Second item.
`,
			want: `cmd/foo: new summary

The steps:

1. First item of the list, with a continuation line that is
   indented to line up under the item text.
2. Second item.

Change-Id: Iabcdef
`,
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

func TestCheckCommitBodyWidth(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr []string // substrings; empty means the check must pass
	}{
		{
			name: "wrapped body passes",
			in: `
cmd/foo: summary

A body wrapped at seventy-two columns.
`[1:],
		},
		{
			name: "numbered list left alone",
			in: `
cmd/foo: summary

The check:

1. First item of the list, with a continuation line that is
   indented to line up under the item text.
2. Second item.
`[1:],
		},
		{
			name: "overlong prose line is an error naming the line",
			in: `
cmd/foo: summary

This line is deliberately made much too long so that it exceeds the seventy-two column limit.
`[1:],
			wantErr: []string{"line 3 (93 columns)"},
		},
		{
			name: "all overlong lines are reported",
			in: `
cmd/foo: summary

This first line is deliberately made much too long so that it exceeds the limit.
Short line.
This third line is also deliberately made much too long so that it exceeds it too.
`[1:],
			wantErr: []string{"line 3 (80 columns)", "line 5 (82 columns)"},
		},
		{
			name: "summary line may exceed the limit",
			in: `
cmd/foo: a deliberately long summary line that exceeds seventy-two columns by quite a margin
`[1:],
		},
		{
			name: "URL line may exceed the limit",
			in: `
cmd/foo: summary

See https://cuelang.org/issue/1234 which has a very long trailing discussion title after it.
`[1:],
		},
		{
			name: "Fixes line may exceed the limit",
			in: `
cmd/foo: summary

Fixes cue-lang/some-very-long-repo-name#123456789012345 together with more trailing text.
`[1:],
		},
		{
			name: "tab-indented quote may exceed the limit",
			in: `
cmd/foo: summary

	cue export --out yaml+indentSeq=false a-very-long-file-name-that-goes-on-and-on.cue
`[1:],
		},
		{
			name: "four-space-indented quote may exceed the limit",
			in: `
cmd/foo: summary

    cue export --out yaml+indentSeq=false a-very-long-file-name-that-goes-on-and-on.cue
`[1:],
		},
		{
			name: "width is measured in runes not bytes",
			in: `
cmd/foo: summary

Dieses Stück enthält Umlaute — ä, ö, ü — und bleibt unter der Grenze.
`[1:],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkCommitBodyWidth(tt.in)
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if got := err.Error(); !strings.Contains(got, want) {
					t.Fatalf("error %q does not contain %q", got, want)
				}
			}
		})
	}
}

func TestRewriteCommitMsgRejectsOverlongLines(t *testing.T) {
	orig := "cmd/foo: old summary\n\nOld description.\n\nChange-Id: Iabcdef\n"
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(orig), 0666); err != nil {
		t.Fatal(err)
	}
	newMsg := "cmd/foo: new summary\n\nA replacement body line that is deliberately made much too long to pass the check.\n"
	err := rewriteCommitMsg(path, newMsg)
	if err == nil {
		t.Fatal("expected an error for an overlong line, got nil")
	}
	if !strings.Contains(err.Error(), "not hard-wrapped") {
		t.Fatalf("unexpected error: %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != orig {
		t.Errorf("commit message file was modified on error:\ngot:\n%s\nwant:\n%s", got, orig)
	}
}

func TestRewriteCommitMsgFileFlag(t *testing.T) {
	dir := t.TempDir()
	editMsg := filepath.Join(dir, "COMMIT_EDITMSG")
	orig := "cmd/foo: old summary\n\nOld description.\n\nChange-Id: Iabcdef\n"
	if err := os.WriteFile(editMsg, []byte(orig), 0666); err != nil {
		t.Fatal(err)
	}
	msgFile := filepath.Join(dir, "msg.txt")
	// A message that could not survive shell quoting as an inline
	// -m argument in GIT_EDITOR.
	newMsg := "cmd/foo: new summary\n\nIt's got an apostrophe and \"quotes\".\n"
	if err := os.WriteFile(msgFile, []byte(newMsg), 0666); err != nil {
		t.Fatal(err)
	}

	cmd := newRewriteCommitMsgCmd(nil)
	cmd.SetArgs([]string{"-F", msgFile, editMsg})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(editMsg)
	if err != nil {
		t.Fatal(err)
	}
	want := "cmd/foo: new summary\n\nIt's got an apostrophe and \"quotes\".\n\nChange-Id: Iabcdef\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteCommitMsgFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "-m and -F are mutually exclusive",
			args:    []string{"-m", "x", "-F", "y", "unused"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "one of -m or -F is required",
			args:    []string{"unused"},
			wantErr: "one of -m or -F is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRewriteCommitMsgCmd(nil)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
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
