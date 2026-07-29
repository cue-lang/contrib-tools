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

//go:build !unix

package cmd

import (
	"errors"
	"os"
	"os/exec"
)

// reExec runs a fresh invocation of the given binary and exits with its
// status, and so does not return on success.
//
// Platforms without execve cannot replace the running process image, so the
// closest equivalent is to run the new binary as a child, forward its
// streams, and adopt its exit code.
//
// Windows is the case that matters here, and it cannot be made to do better.
// Go's syscall.Exec for GOOS=windows is a stub that unconditionally returns
// EWINDOWS (see syscall/exec_windows.go in the standard library), because the
// platform has no process-replacement primitive to build on. Even the C
// runtime's _exec family, which borrows the POSIX names, is documented as
// loading a new process via CreateProcess rather than overlaying the caller:
//
//	https://learn.microsoft.com/en-us/cpp/c-runtime-library/exec-wexec-functions
//
// We wait for the child rather than exiting as soon as it has started.
// Exiting early is what the CRT _exec functions effectively do, and it is
// wrong for a command line tool: the shell would regain the prompt while the
// new process was still writing to the console, and the child's exit status
// would be lost.
func reExec(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), selfUpdatedEnv+"=1")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		// The child never ran, so let the caller carry on with the
		// already-loaded (old) binary rather than exiting.
		return err
	}
	os.Exit(0)
	panic("unreachable")
}
