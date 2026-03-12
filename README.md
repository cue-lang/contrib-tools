## `contrib-tools`

`contrib-tools` provides general code and tools for contributors to the [CUE](https://cuelang.org) project.

### `cueckoo` MCP server

The `cueckoo` command includes an MCP server that exposes CUE project tools to AI
assistants like Claude Code. Available tools:

- **gerrit_comments** — fetch review comments from a GerritHub change
- **trybot_result** — fetch the latest trybot (CI) result for a GerritHub change
- **guidance** — return the latest common guidance for CUE project repos
- **slack_thread** — fetch a Slack thread from the CUE community workspace
- **discord_thread** — fetch a Discord thread from the CUE Discord server

#### Setup

Install `cueckoo`:

```
go install github.com/cue-lang/contrib-tools/cmd/cueckoo@latest
```

Add the MCP server to Claude Code:

```
claude mcp add --transport stdio --scope user cueckoo -- cueckoo mcp
```

Some tools require environment variables:

- `SLACK_TOKEN` — Slack Bot or User token (`xoxb-...` or `xoxp-...`) for `slack_thread`
- `DISCORD_TOKEN` — Discord Bot token for `discord_thread`
