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
