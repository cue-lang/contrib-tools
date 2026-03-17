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
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strings"
)

const gerritBase = "https://cue.gerrithub.io"

// fetchGerritComments fetches review comments from a GerritHub change
// and returns them formatted, grouped by thread with resolved state.
func fetchGerritComments(change string, unresolvedOnly bool) (string, error) {
	changeNumber, err := resolveChangeNumber(change)
	if err != nil {
		return "", err
	}

	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s/comments", changeNumber))
	if err != nil {
		return "", err
	}

	var data map[string][]gerritComment
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parsing Gerrit response: %w", err)
	}

	// Fetch revision info to map patchset numbers to commit SHAs.
	psCommits := fetchPatchSetCommits(changeNumber)

	// Build threads in two passes. First pass: index all comments by ID
	// so that the second pass can always walk in_reply_to chains to find
	// the root, regardless of the order comments appear in the response.
	commentMap := make(map[string]gerritComment)
	for filepath, comments := range data {
		for _, c := range comments {
			c.filepath = filepath
			commentMap[c.ID] = c
		}
	}

	threads := make(map[string][]gerritComment)
	for _, c := range commentMap {
		root := c.ID
		for {
			parent, ok := commentMap[root]
			if !ok || parent.InReplyTo == "" {
				break
			}
			root = parent.InReplyTo
		}
		threads[root] = append(threads[root], c)
	}

	// Sort threads by their root comment's filepath and line for stable output.
	type threadKey struct {
		id       string
		filepath string
		line     int
	}
	var keys []threadKey
	for id, chain := range threads {
		root := chain[0]
		keys = append(keys, threadKey{id: id, filepath: root.filepath, line: root.Line})
	}
	slices.SortFunc(keys, func(a, b threadKey) int {
		if c := cmp.Compare(a.filepath, b.filepath); c != 0 {
			return c
		}
		return cmp.Compare(a.line, b.line)
	})

	var b strings.Builder
	unresolvedCount := 0
	totalCount := 0

	for _, key := range keys {
		chain := threads[key.id]
		slices.SortFunc(chain, func(a, b gerritComment) int {
			return cmp.Compare(a.Updated, b.Updated)
		})

		last := chain[len(chain)-1]
		root := chain[0]
		isUnresolved := last.Unresolved
		state := "RESOLVED"
		if isUnresolved {
			state = "UNRESOLVED"
			unresolvedCount++
		}
		totalCount++

		if unresolvedOnly && !isUnresolved {
			continue
		}

		line := "?"
		if root.Line > 0 {
			line = fmt.Sprintf("%d", root.Line)
		}
		psInfo := "?"
		if root.PatchSet > 0 {
			psInfo = fmt.Sprintf("PS%d", root.PatchSet)
			if commit, ok := psCommits[root.PatchSet]; ok {
				psInfo = fmt.Sprintf("PS%d, commit %.12s", root.PatchSet, commit)
			}
		}

		fmt.Fprintf(&b, "--- %s:%s (%s, %s) [%s] id:%s ---\n", root.filepath, line, psInfo, root.Author.Name, state, root.ID)
		fmt.Fprintln(&b, root.Message)

		for _, reply := range chain[1:] {
			fmt.Fprintf(&b, "  >> %s (id:%s): %s\n", reply.Author.Name, reply.ID, reply.Message)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "Total threads: %d, Unresolved: %d\n", totalCount, unresolvedCount)
	return b.String(), nil
}

type gerritComment struct {
	ID         string       `json:"id"`
	InReplyTo  string       `json:"in_reply_to,omitempty"`
	Message    string       `json:"message"`
	Updated    string       `json:"updated"`
	Author     gerritAuthor `json:"author"`
	Line       int          `json:"line,omitempty"`
	PatchSet   int          `json:"patch_set,omitempty"`
	Unresolved bool         `json:"unresolved"`
	filepath   string
}

type gerritAuthor struct {
	Name string `json:"name"`
}

// fetchPatchSetCommits fetches the revision info for a change and returns
// a map from patchset number to commit SHA. Returns an empty map on error.
func fetchPatchSetCommits(changeNumber string) map[int]string {
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s/?o=ALL_REVISIONS", changeNumber))
	if err != nil {
		return nil
	}

	var detail struct {
		Revisions map[string]struct {
			Number int `json:"_number"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil
	}

	result := make(map[int]string, len(detail.Revisions))
	for commit, rev := range detail.Revisions {
		result[rev.Number] = commit
	}
	return result
}

func resolveChangeNumber(arg string) (string, error) {
	prefix, value, ok := strings.Cut(arg, ":")
	if !ok {
		return "", fmt.Errorf("change argument must use a prefix (cl:, changeid:, or git:), got %q", arg)
	}

	switch prefix {
	case "cl":
		return value, nil

	case "changeid":
		return resolveChangeID(value)

	case "git":
		// Extract Change-Id from the commit message of the given git ref.
		out, err := exec.Command("git", "log", "-1", "--format=%(trailers:key=Change-Id,valueonly=true)", value).Output()
		if err != nil {
			return "", fmt.Errorf("git log for ref %q: %w", value, err)
		}
		changeID := strings.TrimSpace(string(out))
		if changeID == "" {
			return "", fmt.Errorf("no Change-Id found in commit message for ref %q", value)
		}
		return resolveChangeID(changeID)

	default:
		return "", fmt.Errorf("unknown change prefix %q, expected one of cl, changeid, git", prefix)
	}
}

func resolveChangeID(changeID string) (string, error) {
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/?q=%s", changeID))
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
		return "", fmt.Errorf("no change found for Change-Id %q", changeID)
	}

	return fmt.Sprintf("%d", changes[0].Number), nil
}

func gerritAPIGet(path string) ([]byte, error) {
	return gerritAPIRequest("GET", path, nil)
}

// gerritAPIRequest makes a Gerrit REST API request with the given HTTP method.
// If body is non-nil, it is JSON-marshaled and sent as the request body.
// Accepts 200 or 201 as success status codes (Gerrit returns 201 for resource creation).
func gerritAPIRequest(method, path string, body any) ([]byte, error) {
	fullURL := gerritBase + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Look up credentials via git credential helper.
	username, password, err := gitCredentials(context.Background(), fullURL)
	if err != nil {
		return nil, fmt.Errorf("no git credentials found for %s: %w (configure a git credential helper for GerritHub)", gerritBase, err)
	}
	req.SetBasicAuth(username, password)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("gerrit API %s %s returned %s: %s", method, path, resp.Status, respBody)
	}

	// Gerrit REST API prefixes JSON with )]}'<newline> — strip it.
	respBody = bytes.TrimPrefix(respBody, []byte(")]}'"))
	respBody = bytes.TrimLeft(respBody, "\n")

	return respBody, nil
}
