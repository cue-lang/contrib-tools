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

// postGerritReply posts a draft reply to a Gerrit review comment.
// The reply is created as a draft — it is not published until the user
// hits Reply in the Gerrit web UI.
func postGerritReply(change, commentID, message string) (string, error) {
	if commentID == "" {
		return "", fmt.Errorf("comment_id is required")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	changeNumber, err := resolveChangeNumber(change)
	if err != nil {
		return "", err
	}

	// Fetch all comments to find the path and line for the given comment ID.
	body, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s/comments", changeNumber))
	if err != nil {
		return "", fmt.Errorf("fetching comments: %w", err)
	}

	var data map[string][]gerritComment
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parsing comments: %w", err)
	}

	// Look up the comment by ID to get its path and line.
	var found *gerritComment
	for filepath, comments := range data {
		for i := range comments {
			if comments[i].ID == commentID {
				comments[i].filepath = filepath
				found = &comments[i]
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("comment ID %q not found in change %s", commentID, changeNumber)
	}

	// Create a draft reply.
	draft := struct {
		Path       string `json:"path"`
		Line       int    `json:"line,omitempty"`
		Message    string `json:"message"`
		InReplyTo  string `json:"in_reply_to"`
		Unresolved bool   `json:"unresolved"`
	}{
		Path:       found.filepath,
		Line:       found.Line,
		Message:    message,
		InReplyTo:  commentID,
		Unresolved: false,
	}

	respBody, err := gerritAPIRequest("PUT", fmt.Sprintf("/a/changes/%s/revisions/current/drafts", changeNumber), draft)
	if err != nil {
		return "", fmt.Errorf("creating draft reply: %w", err)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("parsing draft response: %w", err)
	}

	return fmt.Sprintf("Draft reply posted to %s:%d (in reply to %s). Draft ID: %s. The draft will be published when the user hits Reply in Gerrit.", found.filepath, found.Line, commentID, created.ID), nil
}

// updateGerritDraft updates an existing draft reply on a Gerrit change.
func updateGerritDraft(change, draftID, message string) (string, error) {
	if draftID == "" {
		return "", fmt.Errorf("draft_id is required")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	changeNumber, err := resolveChangeNumber(change)
	if err != nil {
		return "", err
	}

	// Fetch the existing draft to preserve its path, line, and in_reply_to.
	draftsBody, err := gerritAPIGet(fmt.Sprintf("/a/changes/%s/revisions/current/drafts", changeNumber))
	if err != nil {
		return "", fmt.Errorf("fetching drafts: %w", err)
	}

	var drafts map[string][]gerritComment
	if err := json.Unmarshal(draftsBody, &drafts); err != nil {
		return "", fmt.Errorf("parsing drafts: %w", err)
	}

	var found *gerritComment
	for filepath, comments := range drafts {
		for i := range comments {
			if comments[i].ID == draftID {
				comments[i].filepath = filepath
				found = &comments[i]
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("draft ID %q not found in change %s", draftID, changeNumber)
	}

	update := struct {
		Path       string `json:"path"`
		Line       int    `json:"line,omitempty"`
		Message    string `json:"message"`
		InReplyTo  string `json:"in_reply_to,omitempty"`
		Unresolved bool   `json:"unresolved"`
	}{
		Path:       found.filepath,
		Line:       found.Line,
		Message:    message,
		InReplyTo:  found.InReplyTo,
		Unresolved: false,
	}

	_, err = gerritAPIRequest("PUT", fmt.Sprintf("/a/changes/%s/revisions/current/drafts/%s", changeNumber, draftID), update)
	if err != nil {
		return "", fmt.Errorf("updating draft: %w", err)
	}

	return fmt.Sprintf("Draft %s updated on %s:%d.", draftID, found.filepath, found.Line), nil
}

// deleteGerritDraft deletes a draft reply from a Gerrit change.
func deleteGerritDraft(change, draftID string) (string, error) {
	if draftID == "" {
		return "", fmt.Errorf("draft_id is required")
	}

	changeNumber, err := resolveChangeNumber(change)
	if err != nil {
		return "", err
	}

	_, err = gerritAPIRequest("DELETE", fmt.Sprintf("/a/changes/%s/revisions/current/drafts/%s", changeNumber, draftID), nil)
	if err != nil {
		return "", fmt.Errorf("deleting draft: %w", err)
	}

	return fmt.Sprintf("Draft %s deleted.", draftID), nil
}
