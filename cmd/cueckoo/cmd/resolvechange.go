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
	"os/exec"
	"strings"
)

// resolvedChange holds the result of parsing a change argument. It lazily
// resolves fields that require network requests — only the fields actually
// accessed by the caller trigger API calls.
type resolvedChange struct {
	number   string // CL number, set if known locally
	changeID string // Change-Id (Ixxxx), set if known locally
	revision string // patchset number or "current"
}

// resolveChange parses a change argument and returns a resolvedChange.
// The argument can be:
//   - A Gerrit URL (https://cue.gerrithub.io/c/cue-lang/cue/+/1233920 or .../1233920/2)
//   - cl:<number>
//   - changeid:<id>
//   - git:<ref>
//
// Only local operations are performed during parsing. Network requests
// are deferred until Number() or ChangeID() is called and the needed
// value is not already known.
func resolveChange(arg string) (*resolvedChange, error) {
	// Handle URLs.
	if strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") {
		return resolveChangeFromURL(arg)
	}

	prefix, value, ok := strings.Cut(arg, ":")
	if !ok {
		return nil, fmt.Errorf("change argument must use a prefix (cl:, changeid:, or git:), got %q", arg)
	}

	switch prefix {
	case "cl":
		return &resolvedChange{number: value, revision: "current"}, nil

	case "changeid":
		return &resolvedChange{changeID: value, revision: "current"}, nil

	case "git":
		out, err := exec.Command("git", "log", "-1", "--format=%(trailers:key=Change-Id,valueonly=true)", value).Output()
		if err != nil {
			return nil, fmt.Errorf("git log for ref %q: %w", value, err)
		}
		changeID := strings.TrimSpace(string(out))
		if changeID == "" {
			return nil, fmt.Errorf("no Change-Id found in commit message for ref %q", value)
		}
		return &resolvedChange{changeID: changeID, revision: "current"}, nil

	default:
		return nil, fmt.Errorf("unknown change prefix %q, expected one of cl, changeid, git", prefix)
	}
}

// resolveChangeFromURL extracts a change number and optional patchset from a
// Gerrit URL like https://cue.gerrithub.io/c/cue-lang/cue/+/1233920 or
// https://cue.gerrithub.io/c/cue-lang/cue/+/1233920/2.
func resolveChangeFromURL(rawURL string) (*resolvedChange, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL %q: %w", rawURL, err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "+" && i+1 < len(parts) {
			rc := &resolvedChange{
				number:   parts[i+1],
				revision: "current",
			}
			if i+2 < len(parts) {
				rc.revision = parts[i+2]
			}
			return rc, nil
		}
	}

	return nil, fmt.Errorf("could not extract change number from URL %q", rawURL)
}

// Number returns the CL number, querying the Gerrit API if only the
// Change-Id is known locally.
func (rc *resolvedChange) Number() (string, error) {
	if rc.number != "" {
		return rc.number, nil
	}
	// We have a Change-Id but no number — look it up.
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/?q=%s", rc.changeID))
	if err != nil {
		return "", err
	}
	var changes []struct {
		Number int `json:"_number"`
	}
	if err := json.Unmarshal(body, &changes); err != nil {
		return "", fmt.Errorf("parsing Gerrit changes response: %w", err)
	}
	if len(changes) == 0 {
		return "", fmt.Errorf("no change found for Change-Id %q", rc.changeID)
	}
	rc.number = fmt.Sprintf("%d", changes[0].Number)
	return rc.number, nil
}

// ChangeID returns the Change-Id (Ixxxx trailer value), querying the
// Gerrit API if only the CL number is known locally.
func (rc *resolvedChange) ChangeID() (string, error) {
	if rc.changeID != "" {
		return rc.changeID, nil
	}
	// We have a number but no Change-Id — look it up.
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s", rc.number))
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
		return "", fmt.Errorf("no Change-Id found for change %s", rc.number)
	}
	rc.changeID = detail.ChangeID
	return rc.changeID, nil
}

// Revision returns the patchset identifier ("current" or a number like "2").
func (rc *resolvedChange) Revision() string {
	return rc.revision
}
