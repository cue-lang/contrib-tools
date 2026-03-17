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
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestParseSlackURL(t *testing.T) {
	tests := []struct {
		url     string
		channel string
		ts      string
		wantErr bool
	}{
		{
			url:     "https://cuelang.slack.com/archives/C012UU8B72M/p1772986989323289",
			channel: "C012UU8B72M",
			ts:      "1772986989.323289",
		},
		{
			url:     "https://cuelang.slack.com/archives/C01234ABCDE/p1234567890123456",
			channel: "C01234ABCDE",
			ts:      "1234567890.123456",
		},
		{
			url:     "not-a-slack-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			channel, ts, err := parseSlackURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if channel != tt.channel {
				t.Errorf("channel = %q, want %q", channel, tt.channel)
			}
			if ts != tt.ts {
				t.Errorf("ts = %q, want %q", ts, tt.ts)
			}
		})
	}
}

func TestParseDiscordURL(t *testing.T) {
	tests := []struct {
		url       string
		guildID   string
		channelID string
		messageID string
		wantErr   bool
	}{
		{
			url:       "https://discord.com/channels/953939596592424020/953939596592424023/1234567890123456789",
			guildID:   "953939596592424020",
			channelID: "953939596592424023",
			messageID: "1234567890123456789",
		},
		{
			url:     "not-a-discord-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			guildID, channelID, messageID, err := parseDiscordURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if guildID != tt.guildID {
				t.Errorf("guildID = %q, want %q", guildID, tt.guildID)
			}
			if channelID != tt.channelID {
				t.Errorf("channelID = %q, want %q", channelID, tt.channelID)
			}
			if messageID != tt.messageID {
				t.Errorf("messageID = %q, want %q", messageID, tt.messageID)
			}
		})
	}
}

func TestResolveChangeNumber_CL(t *testing.T) {
	// cl: prefix should return the number directly without any network calls.
	got, err := resolveChangeNumber("cl:1233340")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1233340" {
		t.Errorf("got %q, want %q", got, "1233340")
	}
}

func TestResolveChangeNumber_InvalidPrefix(t *testing.T) {
	// A bare value without a prefix should be rejected.
	_, err := resolveChangeNumber("1233340")
	if err == nil {
		t.Fatal("expected error for bare value without prefix")
	}
}

func TestResolveChangeNumber_UnknownPrefix(t *testing.T) {
	_, err := resolveChangeNumber("foo:bar")
	if err == nil {
		t.Fatal("expected error for unknown prefix")
	}
}

func TestMCPToolsRegistered(t *testing.T) {
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cueckoo-test",
		Version: "v0.0.0-test",
	}, nil)

	// Register the same tools as runMCP does.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_thread",
		Description: "Fetch a Slack thread.",
	}, handleSlackThread)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "discord_thread",
		Description: "Fetch a Discord thread.",
	}, handleDiscordThread)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gerrit_comments",
		Description: "Fetch Gerrit review comments.",
	}, handleGerritComments)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "trybot_result",
		Description: "Fetch trybot CI result.",
	}, handleTrybotResult)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gerrit_reply",
		Description: "Post a draft reply to a Gerrit comment.",
	}, handleGerritReply)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gerrit_update_draft",
		Description: "Update an existing draft reply.",
	}, handleGerritUpdateDraft)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gerrit_delete_draft",
		Description: "Delete a draft reply.",
	}, handleGerritDeleteDraft)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "guidance",
		Description: "Return common guidance.",
	}, handleGuidance)

	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "v0.0.0-test",
	}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Verify all tools are listed.
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	wantTools := map[string]bool{
		"slack_thread":        false,
		"discord_thread":      false,
		"gerrit_comments":     false,
		"gerrit_reply":        false,
		"gerrit_update_draft": false,
		"gerrit_delete_draft": false,
		"trybot_result":       false,
		"guidance":            false,
	}
	for _, tool := range res.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q not found in server tool list", name)
		}
	}
}

func TestPostGerritReply_EmptyCommentID(t *testing.T) {
	_, err := postGerritReply("cl:123", "", "Done.")
	if err == nil {
		t.Fatal("expected error for empty comment_id")
	}
}

func TestPostGerritReply_EmptyMessage(t *testing.T) {
	_, err := postGerritReply("cl:123", "abc123", "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestUpdateGerritDraft_EmptyDraftID(t *testing.T) {
	_, err := updateGerritDraft("cl:123", "", "Done.")
	if err == nil {
		t.Fatal("expected error for empty draft_id")
	}
}

func TestUpdateGerritDraft_EmptyMessage(t *testing.T) {
	_, err := updateGerritDraft("cl:123", "abc123", "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestDeleteGerritDraft_EmptyDraftID(t *testing.T) {
	_, err := deleteGerritDraft("cl:123", "")
	if err == nil {
		t.Fatal("expected error for empty draft_id")
	}
}

func TestMCPGuidanceTool(t *testing.T) {
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cueckoo-test",
		Version: "v0.0.0-test",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "guidance",
		Description: "Return common guidance.",
	}, handleGuidance)

	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "v0.0.0-test",
	}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Call the guidance tool and verify it returns content.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "guidance",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("guidance tool returned an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("guidance tool returned no content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "Commit Messages") {
		t.Errorf("guidance text missing expected section; got:\n%s", tc.Text[:200])
	}
}
