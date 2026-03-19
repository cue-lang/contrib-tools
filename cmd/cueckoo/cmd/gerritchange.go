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
	"encoding/json"
	"fmt"
	"strings"
)

// fetchGerritChange resolves a change identifier to its git fetch URL and ref.
// The identifier can be a Gerrit URL, or the standard prefixed formats
// (cl:, changeid:, git:).
func fetchGerritChange(change string) (string, error) {
	rc, err := resolveChange(change)
	if err != nil {
		return "", err
	}
	changeNumber, err := rc.Number()
	if err != nil {
		return "", err
	}

	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s/?o=CURRENT_REVISION", changeNumber))
	if err != nil {
		return "", err
	}

	var detail struct {
		Project   string `json:"project"`
		Number    int    `json:"_number"`
		Revisions map[string]struct {
			Number int    `json:"_number"`
			Ref    string `json:"ref"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", fmt.Errorf("parsing Gerrit change detail: %w", err)
	}

	if len(detail.Revisions) == 0 {
		return "", fmt.Errorf("no revisions found for change %s", changeNumber)
	}

	// Find the current revision (there should be exactly one with CURRENT_REVISION).
	var ref string
	var patchSet int
	for _, rev := range detail.Revisions {
		ref = rev.Ref
		patchSet = rev.Number
	}

	fetchURL := fmt.Sprintf("%s/a/%s", gerritBase, detail.Project)

	gerritURL := fmt.Sprintf("%s/c/%s/+/%d/%d", gerritBase, detail.Project, detail.Number, patchSet)

	var b strings.Builder
	fmt.Fprintf(&b, "Change: %d (patchset %d)\n", detail.Number, patchSet)
	fmt.Fprintf(&b, "Project: %s\n", detail.Project)
	fmt.Fprintf(&b, "URL: %s\n", gerritURL)
	fmt.Fprintf(&b, "Fetch URL: %s\n", fetchURL)
	fmt.Fprintf(&b, "Ref: %s\n", ref)
	fmt.Fprintf(&b, "\nTo fetch this change:\n")
	fmt.Fprintf(&b, "  git fetch %s %s\n", fetchURL, ref)
	return b.String(), nil
}
