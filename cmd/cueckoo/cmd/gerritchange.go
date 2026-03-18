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
	"net/url"
	"strings"
)

// fetchGerritChange resolves a change identifier to its git fetch URL and ref.
// The identifier can be a Gerrit URL, or the standard prefixed formats
// (cl:, changeid:, git:).
func fetchGerritChange(change string) (string, error) {
	changeNumber, err := resolveChangeArg(change)
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

// resolveChangeArg resolves a change argument that may be a Gerrit URL
// or a prefixed identifier (cl:, changeid:, git:) to a change number.
func resolveChangeArg(arg string) (string, error) {
	number, _, err := resolveChangeWithRevision(arg)
	return number, err
}

// resolveChangeWithRevision resolves a change argument to a change number
// and a Gerrit revision identifier. The revision is a patchset number
// (e.g. "1", "2") if specified in the input (via URL), or "current" otherwise.
func resolveChangeWithRevision(arg string) (number, revision string, _ error) {
	if strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") {
		return resolveChangeURL(arg)
	}
	n, err := resolveChangeNumber(arg)
	return n, "current", err
}

// resolveChangeURL extracts a change number and optional patchset from a
// Gerrit URL like https://cue.gerrithub.io/c/cue-lang/cue/+/1233920 or
// https://cue.gerrithub.io/c/cue-lang/cue/+/1233920/2.
func resolveChangeURL(rawURL string) (number, revision string, _ error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing URL %q: %w", rawURL, err)
	}

	// The URL path looks like /c/cue-lang/cue/+/1233920 or
	// /c/cue-lang/cue/+/1233920/2 (with optional patchset).
	// The change number is the segment after "+", and the patchset
	// (if present) is the segment after that.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "+" && i+1 < len(parts) {
			number = parts[i+1]
			revision = "current"
			if i+2 < len(parts) {
				revision = parts[i+2]
			}
			return number, revision, nil
		}
	}

	return "", "", fmt.Errorf("could not extract change number from URL %q", rawURL)
}
