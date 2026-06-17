// Copyright 2025 The CUE Authors
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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newGuidanceCmd(c *Command) *cobra.Command {
	var hashOnly, install, check bool
	cmd := &cobra.Command{
		Use:   "guidance",
		Short: "Print or manage the common guidance for CUE project repos",
		Long: `Print or manage the common guidance for CUE project repos.

With no flags, the canonical guidance content is printed to stdout
(wrapped in BEGIN/END markers; the BEGIN line includes the current
cueckoo version). This is the same content written to
~/.cache/cueckoo/common-guidance.md and inlined into each repo's
CLAUDE.md via the @-import.

Other forms:

  --hash       print only the hex sha256 of the guidance body
  --install    force-write the guidance to ~/.cache/cueckoo/common-guidance.md
  --check      verify the on-disk guidance matches what this binary
               would write; exit non-zero on drift. Intended for use
               in CI / pre-mail gates (strict byte equality)

Routine refresh (write when missing, refresh on cueckoo upgrade or
any other drift) is the job of "cueckoo version update", which is
what the Claude Code SessionStart hook invokes.
`,
		// Skip the root PersistentPreRun (auto-update check): the
		// guidance subcommand is meta-tooling, and --check in
		// particular needs to see the real on-disk state.
		PersistentPreRun: func(_ *cobra.Command, _ []string) {},
		Args:             cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			modes := 0
			for _, b := range []bool{hashOnly, install, check} {
				if b {
					modes++
				}
			}
			if modes > 1 {
				return fmt.Errorf("--hash, --install and --check are mutually exclusive")
			}
			switch {
			case hashOnly:
				_, err := out.Write([]byte(commonGuidanceHash + "\n"))
				return err
			case install:
				return runGuidanceInstall()
			case check:
				return runGuidanceCheck()
			default:
				_, err := out.Write([]byte(formattedGuidance()))
				return err
			}
		},
	}
	cmd.Flags().BoolVar(&hashOnly, "hash", false, "print only the hex sha256 hash of the guidance")
	cmd.Flags().BoolVar(&install, "install", false, "force-write the guidance to ~/.cache/cueckoo/common-guidance.md")
	cmd.Flags().BoolVar(&check, "check", false, "verify the on-disk guidance file matches; exit non-zero on drift")
	return cmd
}

func runGuidanceInstall() error {
	p, err := defaultGuidancePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(formattedGuidance()), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	fmt.Fprintf(os.Stderr, "cueckoo: wrote guidance to %s\n", p)
	return nil
}

func runGuidanceCheck() error {
	p, err := defaultGuidancePath()
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("reading %s: %w", p, err)
	}
	if string(existing) != formattedGuidance() {
		return fmt.Errorf("guidance file %s does not match the current cueckoo (%s) — run `cueckoo guidance --install` to refresh", p, cueckooVersion)
	}
	return nil
}
