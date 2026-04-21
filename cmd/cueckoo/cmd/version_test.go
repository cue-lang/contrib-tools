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

			// Set GOBIN to the directory containing the cueckoo binary
			// so that `go install` overwrites it and reExec picks up the new binary.
			path := env.Getenv("PATH")
			gobin, _, _ := strings.Cut(path, string(filepath.ListSeparator))
			env.Setenv("GOBIN", gobin)

			return nil
		},
	})
}
