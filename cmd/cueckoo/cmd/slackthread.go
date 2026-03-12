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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// fetchSlackThread fetches a Slack thread given a Slack message URL and
// returns the formatted thread content.
func fetchSlackThread(rawURL string) (string, error) {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		return "", fmt.Errorf("SLACK_TOKEN environment variable is not set")
	}

	channel, ts, err := parseSlackURL(rawURL)
	if err != nil {
		return "", err
	}

	msgs, err := slackFetchThread(token, channel, ts)
	if err != nil {
		return "", err
	}

	names, err := slackResolveUsers(token, msgs)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for i, msg := range msgs {
		if i > 0 {
			fmt.Fprintln(&b, "\n---")
		}
		uid, _ := msg["user"].(string)
		name := names[uid]
		if name == "" {
			name = uid
		}
		ts, _ := msg["ts"].(string)
		text, _ := msg["text"].(string)
		fmt.Fprintf(&b, "[%s] %s\n\n%s\n", name, ts, text)
	}
	return b.String(), nil
}

// slackURLRe matches Slack message URLs and extracts channel ID and timestamp.
var slackURLRe = regexp.MustCompile(`/archives/([A-Z0-9]+)/p(\d{10})(\d+)`)

func parseSlackURL(raw string) (channel, ts string, err error) {
	m := slackURLRe.FindStringSubmatch(raw)
	if m == nil {
		return "", "", fmt.Errorf("cannot parse Slack URL: %s", raw)
	}
	return m[1], m[2] + "." + m[3], nil
}

func slackFetchThread(token, channel, ts string) ([]map[string]any, error) {
	params := url.Values{
		"channel": {channel},
		"ts":      {ts},
		"limit":   {"200"},
	}

	body, err := slackAPI(token, "conversations.replies", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		OK       bool             `json:"ok"`
		Error    string           `json:"error,omitempty"`
		Messages []map[string]any `json:"messages,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing Slack response: %w", err)
	}
	if !resp.OK {
		hint := ""
		if strings.Contains(resp.Error, "not_authed") || strings.Contains(resp.Error, "invalid_auth") {
			hint = " (check your SLACK_TOKEN)"
		}
		if strings.Contains(resp.Error, "missing_scope") {
			hint = " (token is missing required OAuth scope)"
		}
		return nil, fmt.Errorf("slack API error: %s%s", resp.Error, hint)
	}

	return resp.Messages, nil
}

func slackResolveUsers(token string, msgs []map[string]any) (map[string]string, error) {
	ids := make(map[string]bool)
	for _, msg := range msgs {
		if uid, ok := msg["user"].(string); ok {
			ids[uid] = true
		}
	}

	names := make(map[string]string)
	for uid := range ids {
		name, err := slackLookupUser(token, uid)
		if err != nil {
			names[uid] = uid
			continue
		}
		names[uid] = name
	}
	return names, nil
}

func slackLookupUser(token, userID string) (string, error) {
	params := url.Values{"user": {userID}}
	body, err := slackAPI(token, "users.info", params)
	if err != nil {
		return "", err
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
		User  struct {
			Profile struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if !resp.OK {
		return "", fmt.Errorf("users.info: %s", resp.Error)
	}

	name := resp.User.Profile.DisplayName
	if name == "" {
		name = resp.User.Profile.RealName
	}
	return name, nil
}

func slackAPI(token, method string, params url.Values) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://slack.com/api/"+method+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
