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
// searches the local git reflog for all commits that carry that Change-Id
// in their trailers. This shows the local history of a change over time.
func fetchGerritLineage(change string) (string, error) {
	rc, err := resolveChange(change)
	if err != nil {
		return "", err
	}
	changeID, err := rc.ChangeID()
	if err != nil {
		return "", err
	}

	// List all reflog entries with their Change-Id trailer values.
	// The format produces lines like: <sha> <committer-date> <Change-Id>
	out, err := exec.Command("git", "log", "-g", "--all",
		"--format=%H %cI %(trailers:key=Change-Id,valueonly=true,separator=%x00)",
	).Output()
	if err != nil {
		return "", fmt.Errorf("git log reflog: %w", err)
	}

	// Filter for entries matching our Change-Id.
	type entry struct {
		SHA  string
		Date string
	}
	seen := make(map[string]bool)
	var entries []entry

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		// The line is: <sha> <committer-date> <Change-Id(s)>
		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			continue
		}
		sha := fields[0]
		date := fields[1]
		trailerVal := strings.TrimSpace(fields[2])

		// The trailer value may contain multiple Change-Ids separated
		// by null bytes (from the separator=%x00 format). Check each.
		for _, id := range strings.Split(trailerVal, "\x00") {
			if strings.TrimSpace(id) == changeID {
				if !seen[sha] {
					seen[sha] = true
					entries = append(entries, entry{SHA: sha, Date: date})
				}
				break
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Change-Id: %s\n", changeID)
	fmt.Fprintf(&b, "Local commits: %d\n\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s\n", e.SHA, e.Date)
	}
	if len(entries) == 0 {
		fmt.Fprintln(&b, "(no local commits found — they may have been garbage collected or are outside the reflog retention window)")
	}
	return b.String(), nil
}
