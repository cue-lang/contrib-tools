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
	"os"
	"regexp"
	"strings"
)

// fetchDiscordThread fetches a Discord thread given a Discord message URL
// and returns the formatted thread content.
func fetchDiscordThread(rawURL string) (string, error) {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return "", fmt.Errorf("DISCORD_TOKEN environment variable is not set")
	}

	guildID, channelID, messageID, err := parseDiscordURL(rawURL)
	if err != nil {
		return "", err
	}

	msg, err := discordFetchMessage(token, channelID, messageID)
	if err != nil {
		return "", err
	}

	// Determine which channel to fetch messages from.
	threadChannelID := channelID
	if thread, ok := msg["thread"].(map[string]any); ok {
		if id, ok := thread["id"].(string); ok {
			threadChannelID = id
		}
	}

	msgs, err := discordFetchMessages(token, threadChannelID)
	if err != nil {
		return "", err
	}

	names := discordResolveUsers(token, guildID, msgs)

	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			fmt.Fprintln(&b, "\n---")
		}
		author := discordAuthorName(m, names)
		id, _ := m["id"].(string)
		text, _ := m["content"].(string)
		fmt.Fprintf(&b, "[%s] %s\n\n%s\n", author, id, text)
	}
	return b.String(), nil
}

// discordURLRe matches Discord message URLs.
var discordURLRe = regexp.MustCompile(`discord\.com/channels/(\d+)/(\d+)/(\d+)`)

func parseDiscordURL(raw string) (guildID, channelID, messageID string, err error) {
	m := discordURLRe.FindStringSubmatch(raw)
	if m == nil {
		return "", "", "", fmt.Errorf("cannot parse Discord URL: %s", raw)
	}
	return m[1], m[2], m[3], nil
}

func discordFetchMessage(token, channelID, messageID string) (map[string]any, error) {
	path := fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)
	body, err := discordAPI(token, path)
	if err != nil {
		return nil, err
	}

	var msg map[string]any
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("parsing Discord response: %w", err)
	}
	if errMsg, ok := msg["message"].(string); ok {
		return nil, fmt.Errorf("discord API error: %s", errMsg)
	}
	return msg, nil
}

func discordFetchMessages(token, channelID string) ([]map[string]any, error) {
	var all []map[string]any
	after := "0"

	for {
		path := fmt.Sprintf("/channels/%s/messages?limit=100&after=%s", channelID, after)
		body, err := discordAPI(token, path)
		if err != nil {
			return nil, err
		}

		var msgs []map[string]any
		if err := json.Unmarshal(body, &msgs); err != nil {
			var errResp map[string]any
			if json.Unmarshal(body, &errResp) == nil {
				if errMsg, ok := errResp["message"].(string); ok {
					return nil, fmt.Errorf("discord API error: %s", errMsg)
				}
			}
			return nil, fmt.Errorf("parsing Discord response: %w", err)
		}

		if len(msgs) == 0 {
			break
		}

		// Discord returns newest first even with "after", so reverse.
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}

		all = append(all, msgs...)
		after = msgs[len(msgs)-1]["id"].(string)
	}

	return all, nil
}

func discordAuthorName(msg map[string]any, names map[string]string) string {
	author, ok := msg["author"].(map[string]any)
	if !ok {
		return "unknown"
	}
	id, _ := author["id"].(string)
	if name, ok := names[id]; ok {
		return name
	}
	if name, ok := author["global_name"].(string); ok && name != "" {
		return name
	}
	if name, ok := author["username"].(string); ok && name != "" {
		return name
	}
	return id
}

func discordResolveUsers(token, guildID string, msgs []map[string]any) map[string]string {
	ids := make(map[string]bool)
	for _, msg := range msgs {
		if author, ok := msg["author"].(map[string]any); ok {
			if id, ok := author["id"].(string); ok {
				ids[id] = true
			}
		}
	}

	names := make(map[string]string)
	for id := range ids {
		name, err := discordLookupMember(token, guildID, id)
		if err != nil {
			continue
		}
		if name != "" {
			names[id] = name
		}
	}
	return names
}

func discordLookupMember(token, guildID, userID string) (string, error) {
	path := fmt.Sprintf("/guilds/%s/members/%s", guildID, userID)
	body, err := discordAPI(token, path)
	if err != nil {
		return "", err
	}

	var member struct {
		Nick string `json:"nick"`
		User struct {
			GlobalName string `json:"global_name"`
			Username   string `json:"username"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &member); err != nil {
		return "", err
	}

	if member.Nick != "" {
		return member.Nick, nil
	}
	if member.User.GlobalName != "" {
		return member.User.GlobalName, nil
	}
	return member.User.Username, nil
}

func discordAPI(token, path string) ([]byte, error) {
	u := "https://discord.com/api/v10" + path
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}
	req.Header.Set("Authorization", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
