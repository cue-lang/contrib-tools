// Copyright 2021 The CUE Authors
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
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	debug = os.Getenv("CUECKOO_DEBUG") != ""
)

// Main runs the cueckoo tool and returns the code for passing to os.Exit.
//
// We follow the same approach here as the cue command (as well as using the
// using the same version of Cobra) for consistency. Panic is used as a
// strategy for early-return from any running command.
func Main() int {
	err := mainErr(context.Background(), os.Args[1:])
	if err != nil {
		if err != errPrintedError {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}

func mainErr(ctx context.Context, args []string) (err error) {
	defer recoverError(&err)
	cmd, err := New(args)
	if err != nil {
		return err
	}
	return cmd.Run(ctx)
}

func New(args []string) (cmd *Command, err error) {
	defer recoverError(&err)

	cmd = newRootCmd()
	rootCmd := cmd.root
	if len(args) == 0 {
		return cmd, nil
	}
	rootCmd.SetArgs(args)
	return
}

func newRootCmd() *Command {
	cmd := &cobra.Command{
		Use:          "cueckoo",
		Short:        "cueckoo is a development tool for working with the CUE project",
		SilenceUsage: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if os.Getenv("_CUECKOO_SELF_UPDATED") != "" {
				return
			}
			curVersion, latest, hasUpdate := checkForUpdate(false)
			if !hasUpdate {
				return
			}
			exe, target, err := installTarget()
			if err != nil {
				debugf("install location check error: %v\n", err)
				return
			}
			if exe != target {
				// `go install` would not overwrite the running binary, so an
				// in-place auto-update isn't safe. Nudge the user instead.
				fmt.Fprintf(os.Stderr, "cueckoo: a newer version %s is available (current: %s)\n"+
					"cueckoo: the running binary %s is not in GOBIN/GOPATH/bin; run `cueckoo version update` or reinstall manually\n",
					latest.Version, curVersion, exe)
				return
			}
			fmt.Fprintf(os.Stderr, "cueckoo: updating %s -> %s ...\n", curVersion, latest.Version)
			if err := installUpdate(latest.Version); err != nil {
				debugf("update install error: %v\n", err)
				return
			}
			// Have the newly-installed binary write the matching
			// guidance file before we re-exec into it. The running
			// process is still the OLD binary so it cannot write
			// the new guidance from its own commonGuidance; shell
			// out to target. Errors are non-fatal — the binary
			// upgrade itself has already succeeded.
			if err := exec.Command(target, "guidance", "--install").Run(); err != nil {
				debugf("guidance install error: %v\n", err)
			}
			if err := reExec(exe); err != nil {
				debugf("re-exec error: %v\n", err)
			}
		},
	}

	c := &Command{Command: cmd, root: cmd}

	subCommands := []*cobra.Command{
		newRuntrybotCmd(c),
		newImportPRCmd(c),
		newUnityCmd(c),
		newReleaselogCmd(c),
		newVersionCmd(c),
		newMCPCmd(c),
		newRewriteCommitMsgCmd(c),
		newGuidanceCmd(c),
	}

	for _, sub := range subCommands {
		cmd.AddCommand(sub)
	}

	return c
}

func debugf(format string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}
