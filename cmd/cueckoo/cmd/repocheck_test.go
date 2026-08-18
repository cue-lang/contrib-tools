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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireCueckooRepo(t *testing.T) {
	claudeMDWithImport := "# Some repo\n\n" + guidanceImportLine + "\n\n## More\n"
	tests := []struct {
		name    string
		gitInit bool
		files   map[string]string
		subdir  string // run from this subdirectory of the repo
		wantErr string // empty means the check must pass
	}{{
		name:    "not a git repository",
		gitInit: false,
		wantErr: "only operates within a git repository",
	}, {
		name:    "no CLAUDE.md",
		gitInit: true,
		wantErr: "not configured to use cueckoo",
	}, {
		name:    "CLAUDE.md without guidance import",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md": "# Some repo\n\nNo import here.\n",
		},
		wantErr: "does not import the cueckoo common guidance",
	}, {
		name:    "guidance import mentioned in prose only",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md": "# Some repo\n\nsee @~/.cache/cueckoo/common-guidance.md for details\n",
		},
		wantErr: "does not import the cueckoo common guidance",
	}, {
		name:    "CLAUDE.md with guidance import",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md": claudeMDWithImport,
		},
	}, {
		name:    "guidance import with surrounding whitespace",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md": "  " + guidanceImportLine + "  \n",
		},
	}, {
		name:    "run from a subdirectory",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md": claudeMDWithImport,
		},
		subdir: "sub/dir",
	}, {
		name:    "codereview.cfg with matching gerrit server",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md":      claudeMDWithImport,
			"codereview.cfg": "gerrit: https://cue.gerrithub.io/a/cue-lang/example\ngithub: https://github.com/cue-lang/example\n",
		},
	}, {
		name:    "codereview.cfg with other gerrit server",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md":      claudeMDWithImport,
			"codereview.cfg": "gerrit: https://other.example.com/a/other-org/example\n",
		},
		wantErr: "codereview.cfg gerrit server is https://other.example.com",
	}, {
		name:    "codereview.cfg without gerrit entry",
		gitInit: true,
		files: map[string]string{
			"CLAUDE.md":      claudeMDWithImport,
			"codereview.cfg": "github: https://github.com/cue-lang/example\n",
		},
		wantErr: "codereview.cfg has no gerrit entry",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
			t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
			dir := t.TempDir()
			// Ensure git does not resolve to an enclosing repository
			// in the not-a-git-repository case.
			t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
			if tt.gitInit {
				out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput()
				if err != nil {
					t.Fatalf("git init: %v\n%s", err, out)
				}
			}
			for name, contents := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			if tt.subdir != "" {
				dir = filepath.Join(dir, tt.subdir)
				if err := os.MkdirAll(dir, 0o777); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			err := requireCueckooRepo(context.Background())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}
