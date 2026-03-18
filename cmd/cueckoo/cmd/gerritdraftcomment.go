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
)

// postGerritDraftComment posts a new draft comment on a specific file and
// line of a GerritHub change. The draft is not published until the user
// reviews and hits Reply in the Gerrit web UI.
func postGerritDraftComment(change, patchset, path string, line int, resolved bool, message string) (string, error) {
	if patchset == "" {
		return "", fmt.Errorf("patchset is required")
	}
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	changeNumber, err := resolveChangeArg(change)
	if err != nil {
		return "", err
	}

	draft := struct {
		Path       string `json:"path"`
		Line       int    `json:"line,omitempty"`
		Message    string `json:"message"`
		Unresolved bool   `json:"unresolved"`
	}{
		Path:       path,
		Line:       line,
		Message:    message,
		Unresolved: !resolved,
	}

	respBody, err := gerritAPIRequest("PUT", fmt.Sprintf("/a/changes/%s/revisions/%s/drafts", changeNumber, patchset), draft)
	if err != nil {
		return "", fmt.Errorf("creating draft comment: %w", err)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("parsing draft response: %w", err)
	}

	lineInfo := ""
	if line > 0 {
		lineInfo = fmt.Sprintf(":%d", line)
	}

	gerritURL := fmt.Sprintf("%s/c/%s/%s", gerritBase, changeNumber, patchset)

	return fmt.Sprintf("Draft comment posted on %s%s. Draft ID: %s.\nReview drafts at: %s", path, lineInfo, created.ID, gerritURL), nil
}
