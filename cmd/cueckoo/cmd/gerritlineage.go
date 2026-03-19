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
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// fetchGerritLineage resolves a change identifier to a Change-Id, then
// searches local git reflogs for all commits that carry that Change-Id
// in their trailers. It scopes the search to branches sharing the same
// upstream target, matching Gerrit's view that a review is defined by
// Target Branch + Change-Id.
func fetchGerritLineage(change string) (string, error) {
	rc, err := resolveChange(change)
	if err != nil {
		return "", err
	}
	changeID, err := rc.ChangeID()
	if err != nil {
		return "", err
	}

	// Determine the upstream target for the current branch.
	target, err := upstreamTarget()
	if err != nil {
		return "", err
	}

	// Find all local branches that share this upstream target.
	branches, err := branchesTrackingTarget(target)
	if err != nil {
		return "", err
	}
	if len(branches) == 0 {
		return "", fmt.Errorf("no local branches tracking %s", target)
	}

	// Scan the reflogs of all matching branches for commits with
	// the given Change-Id.
	entries, err := scanReflogsForChangeID(branches, changeID)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Change-Id: %s\n", changeID)
	fmt.Fprintf(&b, "Target: %s\n", target)
	fmt.Fprintf(&b, "Local commits: %d\n\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s\n", e.SHA, e.Date)
	}
	if len(entries) == 0 {
		fmt.Fprintln(&b, "(no local commits found — they may have been garbage collected or are outside the reflog retention window)")
	}
	return b.String(), nil
}

// upstreamTarget returns the upstream tracking branch for the current
// branch (e.g. "origin/main").
func upstreamTarget() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "@{u}").Output()
	if err != nil {
		return "", fmt.Errorf("determining upstream target: %w (is the current branch tracking a remote?)", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// branchesTrackingTarget returns all local branch names that track the
// given upstream branch.
func branchesTrackingTarget(target string) ([]string, error) {
	out, err := exec.Command("git", "for-each-ref",
		"--format=%(refname:short) %(upstream:short)",
		"refs/heads/",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), " ", 2)
		if len(fields) == 2 && fields[1] == target {
			branches = append(branches, fields[0])
		}
	}
	return branches, nil
}

type lineageEntry struct {
	SHA  string
	Date string
}

// scanReflogsForChangeID scans the reflogs of the given branches for
// commits carrying the specified Change-Id in their trailers.
func scanReflogsForChangeID(branches []string, changeID string) ([]lineageEntry, error) {
	// Build the git log command: -g walks reflogs for the listed refs.
	args := []string{"log", "-g",
		"--format=%H %cI %(trailers:key=Change-Id,valueonly=true,separator=%x00)",
	}
	for _, br := range branches {
		args = append(args, br)
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log reflog: %w", err)
	}

	seen := make(map[string]bool)
	var entries []lineageEntry

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			continue
		}
		sha := fields[0]
		date := fields[1]
		trailerVal := strings.TrimSpace(fields[2])

		for _, id := range strings.Split(trailerVal, "\x00") {
			if strings.TrimSpace(id) == changeID {
				if !seen[sha] {
					seen[sha] = true
					entries = append(entries, lineageEntry{SHA: sha, Date: date})
				}
				break
			}
		}
	}

	return entries, nil
}
