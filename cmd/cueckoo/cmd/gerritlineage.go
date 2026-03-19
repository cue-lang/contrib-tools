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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// fetchGerritLineage resolves a change identifier to a Change-Id, then
// searches the local git reflog for all commits that carry that Change-Id
// in their trailers. This shows the local history of a change over time.
func fetchGerritLineage(change string) (string, error) {
	changeID, err := resolveToChangeID(change)
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

// resolveToChangeID resolves a change argument to a Change-Id (the Ixxxx
// trailer value). This is the inverse of resolveChangeNumber which resolves
// to a CL number.
func resolveToChangeID(arg string) (string, error) {
	// Handle URLs by extracting the change number first.
	if strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") {
		number, _, err := resolveChangeURL(arg)
		if err != nil {
			return "", err
		}
		return changeNumberToChangeID(number)
	}

	prefix, value, ok := strings.Cut(arg, ":")
	if !ok {
		return "", fmt.Errorf("change argument must use a prefix (cl:, changeid:, or git:), got %q", arg)
	}

	switch prefix {
	case "changeid":
		return value, nil

	case "git":
		out, err := exec.Command("git", "log", "-1", "--format=%(trailers:key=Change-Id,valueonly=true)", value).Output()
		if err != nil {
			return "", fmt.Errorf("git log for ref %q: %w", value, err)
		}
		changeID := strings.TrimSpace(string(out))
		if changeID == "" {
			return "", fmt.Errorf("no Change-Id found in commit message for ref %q", value)
		}
		return changeID, nil

	case "cl":
		return changeNumberToChangeID(value)

	default:
		return "", fmt.Errorf("unknown change prefix %q, expected one of cl, changeid, git", prefix)
	}
}

// changeNumberToChangeID queries the Gerrit API to get the Change-Id for a
// change number.
func changeNumberToChangeID(number string) (string, error) {
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s", number))
	if err != nil {
		return "", err
	}

	var detail struct {
		ChangeID string `json:"change_id"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", fmt.Errorf("parsing Gerrit change detail: %w", err)
	}
	if detail.ChangeID == "" {
		return "", fmt.Errorf("no Change-Id found for change %s", number)
	}
	return detail.ChangeID, nil
}
