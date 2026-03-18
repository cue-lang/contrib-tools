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
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// httpClient is a shared HTTP client with a sensible timeout, used by all
// MCP tool implementations instead of http.DefaultClient.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func newMCPCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run an MCP server exposing CUE project tools",
		Long: `Run a Model Context Protocol (MCP) server over stdio.

This exposes CUE project tools (Slack thread fetching, Discord thread
fetching, Gerrit comment fetching) as MCP tools that can be used by
AI assistants like Claude Code.

Add to Claude Code:

  claude mcp add --transport stdio --scope user cueckoo -- cueckoo mcp

Environment variables:
  SLACK_TOKEN     Slack Bot or User token (xoxb-... or xoxp-...)
  DISCORD_TOKEN   Discord Bot token
`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMCP(context.Background())
		},
	}
	return cmd
}

func runMCP(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cueckoo",
		Version: "v0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "slack_thread",
		Description: `Fetch a Slack thread and return all messages with resolved usernames.

Takes a Slack message URL (e.g. https://cuelang.slack.com/archives/C012UU8B72M/p1234567890123456)
and returns all messages in that thread, with user IDs resolved to display names.

Requires SLACK_TOKEN environment variable.`,
	}, handleSlackThread)

	mcp.AddTool(server, &mcp.Tool{
		Name: "discord_thread",
		Description: `Fetch a Discord thread and return all messages with resolved usernames.

Takes a Discord message URL (e.g. https://discord.com/channels/953939596592424020/953939596592424023/1234567890)
and returns all messages in that thread, with user IDs resolved to display names.

Requires DISCORD_TOKEN environment variable.`,
	}, handleDiscordThread)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_comments",
		Description: `Fetch review comments from a GerritHub change, grouped by thread with resolved state.

The change argument must use one of these prefixed formats:

  cl:<number>        — CL number, e.g. cl:1233340
  changeid:<id>      — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>          — any git ref (commit SHA, branch, tag, HEAD, HEAD~2, etc.), e.g. git:HEAD

When presenting results, focus on unresolved threads as these need action. For each
unresolved thread, summarise the feedback and suggest a concrete plan for addressing it.
When the file /COMMIT_MSG appears as a path, present those comments as feedback on the
commit message itself, not a source file. If all threads are resolved, report that no
action is needed.

Set unresolved_only to true to show only unresolved threads.`,
	}, handleGerritComments)

	mcp.AddTool(server, &mcp.Tool{
		Name: "trybot_result",
		Description: `Fetch the latest trybot (CI) result for a GerritHub change.

The change argument must use one of these prefixed formats:

  cl:<number>        — CL number, e.g. cl:1233340
  changeid:<id>      — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>          — any git ref (commit SHA, branch, tag, HEAD, HEAD~2, etc.), e.g. git:HEAD

If the trybot run failed, this tool fetches the failed job logs from GitHub
Actions and extracts the error output. Use this to understand why CI failed
and what needs to be fixed.

Requires the gh CLI to be installed and authenticated.`,
	}, handleTrybotResult)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_change",
		Description: `Resolve a GerritHub change to its git fetch URL and ref.

Takes a change identifier and returns the git fetch URL and ref needed to
retrieve the change locally. The caller can then use the fetch URL and ref
for checkout, cherry-pick, diff, or any other git operation.

The change argument can be:

  A Gerrit URL         — e.g. https://cue.gerrithub.io/c/cue-lang/cue/+/1233920
  cl:<number>          — CL number, e.g. cl:1233920
  changeid:<id>        — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>            — any git ref, e.g. git:HEAD`,
	}, handleGerritChange)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_draft_comment",
		Description: `Post a new draft review comment on a specific file and line of a GerritHub change.

This creates a new comment (not a reply to an existing comment). The draft is
NOT published until the user reviews the drafts in Gerrit and hits Reply.

Use this when performing a code review to leave feedback on specific lines of
code. The response includes a link to the CL patchset where the user can
review all draft comments and publish them.

The change argument can be:

  A Gerrit URL         — e.g. https://cue.gerrithub.io/c/cue-lang/cue/+/1233920
  cl:<number>          — CL number, e.g. cl:1233920
  changeid:<id>        — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>            — any git ref, e.g. git:HEAD

The patchset parameter is required — it identifies which patchset of the
change the comment applies to. This is known from the gerrit_change output
used to fetch the code for review.

Set line to 0 to post a file-level comment (not attached to a specific line).

Comments default to unresolved (requiring action from the CL author). Set
resolved to true for FYI or informational comments that do not require
a response.`,
	}, handleGerritDraftComment)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_reply",
		Description: `Post a draft reply to a GerritHub review comment.

The reply is created as a draft — it is NOT published until the user reviews
the drafts in Gerrit and hits Reply. This is safe to call freely: the user
always has a chance to review and edit drafts before they become visible to
the reviewer.

Typical reply messages: "Done.", "Acknowledged.", or a brief description of
what was changed to address the feedback.

The change argument must use one of these prefixed formats:

  cl:<number>        — CL number, e.g. cl:1233340
  changeid:<id>      — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>          — any git ref (commit SHA, branch, tag, HEAD, HEAD~2, etc.), e.g. git:HEAD

The comment_id is the ID of the comment to reply to, as shown in the
gerrit_comments output (e.g. id:abc123). Pass the ID without the "id:" prefix.

The response includes the draft ID, which can be used with gerrit_update_draft
to edit the draft or gerrit_delete_draft to remove it.`,
	}, handleGerritReply)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_update_draft",
		Description: `Update an existing draft reply on a GerritHub change.

Use this to edit the message of a draft that was previously created with
gerrit_reply. The draft_id is returned in the gerrit_reply response.

The change argument must use one of these prefixed formats:

  cl:<number>        — CL number, e.g. cl:1233340
  changeid:<id>      — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>          — any git ref (commit SHA, branch, tag, HEAD, HEAD~2, etc.), e.g. git:HEAD`,
	}, handleGerritUpdateDraft)

	mcp.AddTool(server, &mcp.Tool{
		Name: "gerrit_delete_draft",
		Description: `Delete a draft reply from a GerritHub change.

Use this to remove a draft that was previously created with gerrit_reply.
The draft_id is returned in the gerrit_reply response.

The change argument must use one of these prefixed formats:

  cl:<number>        — CL number, e.g. cl:1233340
  changeid:<id>      — Change-Id, e.g. changeid:Ia15e97465869aa18ba2b8c9795cec18f438d7b76
  git:<ref>          — any git ref (commit SHA, branch, tag, HEAD, HEAD~2, etc.), e.g. git:HEAD`,
	}, handleGerritDeleteDraft)

	mcp.AddTool(server, &mcp.Tool{
		Name: "guidance",
		Description: `Return the latest common guidance/instructions for CUE project repos.

This returns canonical instructions that should be incorporated into each
repo's CLAUDE.md file. It covers commit message conventions, GerritHub code
review workflows (including git-codereview usage, working with commit chains,
editing and splitting commits, and preserving Change-Ids), CI/trybots,
community support, testing with txtar reproductions, and CLAUDE.md structure.

Call this tool when setting up a new repo or when asked to verify that
a repo's instructions are current.`,
	}, handleGuidance)

	return server.Run(ctx, &mcp.StdioTransport{})
}

type slackThreadInput struct {
	URL string `json:"url" jsonschema:"Slack message URL"`
}

func handleSlackThread(ctx context.Context, req *mcp.CallToolRequest, input slackThreadInput) (*mcp.CallToolResult, any, error) {
	result, err := fetchSlackThread(input.URL)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type discordThreadInput struct {
	URL string `json:"url" jsonschema:"Discord message URL"`
}

func handleDiscordThread(ctx context.Context, req *mcp.CallToolRequest, input discordThreadInput) (*mcp.CallToolResult, any, error) {
	result, err := fetchDiscordThread(input.URL)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type guidanceInput struct{}

func handleGuidance(ctx context.Context, req *mcp.CallToolRequest, input guidanceInput) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: commonGuidance},
		},
	}, nil, nil
}

type trybotResultInput struct {
	Change string `json:"change" jsonschema:"prefixed change identifier: cl:<number>, changeid:<id>, or git:<ref>"`
}

func handleTrybotResult(ctx context.Context, req *mcp.CallToolRequest, input trybotResultInput) (*mcp.CallToolResult, any, error) {
	result, err := fetchTrybotResult(input.Change)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritChangeInput struct {
	Change string `json:"change" jsonschema:"change identifier: a Gerrit URL, or prefixed as cl:<number>, changeid:<id>, or git:<ref>"`
}

func handleGerritChange(ctx context.Context, req *mcp.CallToolRequest, input gerritChangeInput) (*mcp.CallToolResult, any, error) {
	result, err := fetchGerritChange(input.Change)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritDraftCommentInput struct {
	Change   string `json:"change" jsonschema:"change identifier: a Gerrit URL, or prefixed as cl:<number>, changeid:<id>, or git:<ref>"`
	PatchSet string `json:"patchset" jsonschema:"patchset number to post the comment against (from gerrit_change output)"`
	Path     string `json:"path" jsonschema:"file path to comment on (relative to repo root)"`
	Line     int    `json:"line,omitempty" jsonschema:"line number to comment on (0 or omit for file-level comment)"`
	Resolved bool   `json:"resolved,omitempty" jsonschema:"whether the comment is resolved/FYI (default false, meaning the comment requires action from the CL author). Set to true for informational comments."`
	Message  string `json:"message" jsonschema:"review comment text"`
}

func handleGerritDraftComment(ctx context.Context, req *mcp.CallToolRequest, input gerritDraftCommentInput) (*mcp.CallToolResult, any, error) {
	result, err := postGerritDraftComment(input.Change, input.PatchSet, input.Path, input.Line, input.Resolved, input.Message)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritReplyInput struct {
	Change    string `json:"change" jsonschema:"prefixed change identifier: cl:<number>, changeid:<id>, or git:<ref>"`
	CommentID string `json:"comment_id" jsonschema:"ID of the comment to reply to (from gerrit_comments output)"`
	Message   string `json:"message" jsonschema:"reply text (e.g. Done., Acknowledged., or a description of what was changed)"`
}

func handleGerritReply(ctx context.Context, req *mcp.CallToolRequest, input gerritReplyInput) (*mcp.CallToolResult, any, error) {
	result, err := postGerritReply(input.Change, input.CommentID, input.Message)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritUpdateDraftInput struct {
	Change  string `json:"change" jsonschema:"prefixed change identifier: cl:<number>, changeid:<id>, or git:<ref>"`
	DraftID string `json:"draft_id" jsonschema:"ID of the draft to update (from gerrit_reply response)"`
	Message string `json:"message" jsonschema:"new reply text"`
}

func handleGerritUpdateDraft(ctx context.Context, req *mcp.CallToolRequest, input gerritUpdateDraftInput) (*mcp.CallToolResult, any, error) {
	result, err := updateGerritDraft(input.Change, input.DraftID, input.Message)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritDeleteDraftInput struct {
	Change  string `json:"change" jsonschema:"prefixed change identifier: cl:<number>, changeid:<id>, or git:<ref>"`
	DraftID string `json:"draft_id" jsonschema:"ID of the draft to delete (from gerrit_reply response)"`
}

func handleGerritDeleteDraft(ctx context.Context, req *mcp.CallToolRequest, input gerritDeleteDraftInput) (*mcp.CallToolResult, any, error) {
	result, err := deleteGerritDraft(input.Change, input.DraftID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type gerritCommentsInput struct {
	Change         string `json:"change" jsonschema:"prefixed change identifier: cl:<number>, changeid:<id>, or git:<ref>"`
	UnresolvedOnly bool   `json:"unresolved_only,omitempty" jsonschema:"show only unresolved threads"`
}

func handleGerritComments(ctx context.Context, req *mcp.CallToolRequest, input gerritCommentsInput) (*mcp.CallToolResult, any, error) {
	result, err := fetchGerritComments(input.Change, input.UnresolvedOnly)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}
