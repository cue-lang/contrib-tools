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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
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

	body, err := gerritAPI(fmt.Sprintf("/a/changes/%s/comments", changeNumber))
	if err != nil {
		return "", err
	}

	var data map[string][]gerritComment
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parsing Gerrit response: %w", err)
	}

	// Build threads: group by root comment.
	threads := make(map[string][]gerritComment)
	commentMap := make(map[string]gerritComment)

	for filepath, comments := range data {
		for _, c := range comments {
			c.filepath = filepath
			commentMap[c.ID] = c

			if c.InReplyTo == "" {
				threads[c.ID] = append(threads[c.ID], c)
			} else {
				// Find root of thread.
				root := c.InReplyTo
				for {
					parent, ok := commentMap[root]
					if !ok || parent.InReplyTo == "" {
						break
					}
					root = parent.InReplyTo
				}
				threads[root] = append(threads[root], c)
			}
		}
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
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].filepath != keys[j].filepath {
			return keys[i].filepath < keys[j].filepath
		}
		return keys[i].line < keys[j].line
	})

	var b strings.Builder
	unresolvedCount := 0
	totalCount := 0

	for _, key := range keys {
		chain := threads[key.id]
		sort.Slice(chain, func(i, j int) bool {
			return chain[i].Updated < chain[j].Updated
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
		ps := "?"
		if root.PatchSet > 0 {
			ps = fmt.Sprintf("%d", root.PatchSet)
		}

		fmt.Fprintf(&b, "--- %s:%s (PS%s, %s) [%s] ---\n", root.filepath, line, ps, root.Author.Name, state)
		fmt.Fprintln(&b, root.Message)

		for _, reply := range chain[1:] {
			fmt.Fprintf(&b, "  >> %s: %s\n", reply.Author.Name, reply.Message)
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
		out, err := exec.Command("git", "log", "-1", "--format=%B", value).Output()
		if err != nil {
			return "", fmt.Errorf("git log for ref %q: %w", value, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Change-Id: ") {
				changeID := strings.TrimPrefix(line, "Change-Id: ")
				return resolveChangeID(changeID)
			}
		}
		return "", fmt.Errorf("no Change-Id found in commit message for ref %q", value)

	default:
		return "", fmt.Errorf("unknown change prefix %q, expected one of cl, changeid, git", prefix)
	}
}

func resolveChangeID(changeID string) (string, error) {
	body, err := gerritAPI(fmt.Sprintf("/a/changes/?q=%s", changeID))
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

func gerritAPI(path string) ([]byte, error) {
	fullURL := gerritBase + path
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// Look up credentials via git credential helper.
	if username, password, err := gitCredentials(context.Background(), fullURL); err == nil {
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gerrit API %s returned %s: %s", path, resp.Status, body)
	}

	// Gerrit REST API prefixes JSON with )]}'<newline> — strip it.
	body = bytes.TrimPrefix(body, []byte(")]}'"))
	body = bytes.TrimLeft(body, "\n")

	return body, nil
}
