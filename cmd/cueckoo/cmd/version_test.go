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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/goproxytest"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"cueckoo": func() { os.Exit(Main()) },
	})
}

func TestScript(t *testing.T) {
	srv := goproxytest.NewTestServer(t, "testdata/testmod", "")

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("GOPROXY", srv.URL)
			env.Setenv("GOSUMDB", "off")

			// Use per-test directories so tests don't interfere
			// and so that `go install` has a writable module cache.
			env.Setenv("XDG_CACHE_HOME", filepath.Join(env.WorkDir, "cache"))
			env.Setenv("GOPATH", filepath.Join(env.WorkDir, "gopath"))
			env.Setenv("GOMODCACHE", filepath.Join(env.WorkDir, "gomodcache"))
			env.Setenv("GOCACHE", filepath.Join(env.WorkDir, "gocache"))

			// Per-test bindir so `go install` only overwrites a disposable
			// copy of cueckoo, not the shared testscript dispatch binary
			// (which would break later tests and -count=N iterations).
			perTestBin := filepath.Join(env.WorkDir, "bin")
			if err := os.MkdirAll(perTestBin, 0o777); err != nil {
				return err
			}
			path := env.Getenv("PATH")
			sharedBindir, _, _ := strings.Cut(path, string(filepath.ListSeparator))
			if err := copyFile(filepath.Join(sharedBindir, "cueckoo"), filepath.Join(perTestBin, "cueckoo")); err != nil {
				return err
			}
			env.Setenv("PATH", perTestBin+string(filepath.ListSeparator)+path)
			env.Setenv("GOBIN", perTestBin)

			return nil
		},
	})
}

func copyFile(src, dst string) error {
	// Prefer a hardlink so there is no window between writing and exec
	// during which the kernel may return ETXTBSY.
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
