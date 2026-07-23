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
	"os/exec"
	"testing"
)

func TestGerritTargetBranch(t *testing.T) {
	tests := []struct {
		name   string
		remote string // empty means no upstream is configured
		want   string
	}{
		{name: "origin remote", remote: "origin", want: "master"},
		{name: "non-origin remote", remote: "cuelabs", want: "master"},
		{name: "no upstream", remote: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
			t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
			t.Chdir(t.TempDir())
			mustGit := func(args ...string) {
				t.Helper()
				out, err := exec.Command("git", args...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			mustGit("init", "-q", "-b", "work")
			mustGit("-c", "user.name=test", "-c", "user.email=test@example.com",
				"commit", "-q", "--allow-empty", "-m", "commit")
			if tt.remote != "" {
				mustGit("remote", "add", tt.remote, "https://example.com/repo.git")
				mustGit("update-ref", "refs/remotes/"+tt.remote+"/master", "HEAD")
				mustGit("branch", "--set-upstream-to="+tt.remote+"/master")
			}
			if got := gerritTargetBranch(context.Background()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
