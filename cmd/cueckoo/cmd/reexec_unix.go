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

//go:build unix

package cmd

import (
	"os"
	"syscall"
)

// reExec replaces the current process with a fresh invocation of the given
// binary, and so does not return on success.
//
// See reexec_other.go for the platforms that have no execve to replace the
// process with, where this has to be emulated with a child process.
func reExec(exe string) error {
	os.Setenv(selfUpdatedEnv, "1")
	return syscall.Exec(exe, os.Args, os.Environ())
}
