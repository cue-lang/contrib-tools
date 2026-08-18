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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cue-lang/contrib-tools/internal/codereviewcfg"
)

// guidanceImportLine is the CLAUDE.md @-import line by which a repository
// opts in to the cueckoo common guidance. Its presence is what marks a
// repository as configured to use cueckoo; see the "Configuring a repo to
// use this guidance" section of the common guidance.
const guidanceImportLine = "@~/.cache/cueckoo/common-guidance.md"

// requireCueckooRepo verifies that the current working directory is inside
// a repository that has opted in to cueckoo, and fails closed otherwise.
//
// A repository has opted in when its root CLAUDE.md contains the common
// guidance @-import line. Additionally, when the repository has a
// codereview.cfg, its gerrit entry must name the Gerrit server this binary
// is built to talk to — a repository whose review happens elsewhere must
// not be operated on even if it carries a stale CLAUDE.md marker.
//
// Commands with external effects must call this (directly or via
// loadConfig) before doing anything. Local-only and bootstrap commands
// (version, guidance, rewrite-commit-msg) are exempt: they have no
// cross-repository effects, and the guidance and version commands are
// needed to configure a repository that does not yet carry the marker.
func requireCueckooRepo(ctx context.Context) error {
	gitRoot, err := run(ctx, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("cueckoo only operates within a git repository configured to use it: %v", err)
	}
	root := strings.TrimSpace(gitRoot)

	claudeMD, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		return notConfiguredErr(root, err)
	}
	found := false
	for line := range strings.SplitSeq(string(claudeMD), "\n") {
		if strings.TrimSpace(line) == guidanceImportLine {
			found = true
			break
		}
	}
	if !found {
		return notConfiguredErr(root, fmt.Errorf("CLAUDE.md does not import the cueckoo common guidance"))
	}

	// If the repository declares its review configuration, it must match
	// the Gerrit server this binary talks to.
	if _, err := os.Stat(filepath.Join(root, "codereview.cfg")); err == nil {
		cfg, err := codereviewcfg.Config(root)
		if err != nil {
			return err
		}
		gerritURL := cfg["gerrit"]
		if gerritURL == "" {
			return notConfiguredErr(root, fmt.Errorf("codereview.cfg has no gerrit entry"))
		}
		server, _, err := codereviewcfg.GerritURLToParts(gerritURL)
		if err != nil {
			return err
		}
		if server != gerritBase {
			return notConfiguredErr(root, fmt.Errorf("codereview.cfg gerrit server is %s, not %s", server, gerritBase))
		}
	}
	return nil
}

func notConfiguredErr(root string, cause error) error {
	return fmt.Errorf("repository at %s is not configured to use cueckoo: %v\n"+
		"cueckoo only operates on repositories whose CLAUDE.md imports the cueckoo\n"+
		"common guidance (%s) and whose codereview.cfg,\n"+
		"if any, names %s; see the \"Configuring a repo to use this\n"+
		"guidance\" section of the common guidance (cueckoo guidance)",
		root, cause, guidanceImportLine, gerritBase)
}
