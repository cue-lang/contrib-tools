# contrib-tools

## Common guidance

You MUST load and follow the cueckoo common guidance for this repo.
The guidance is served by the cueckoo MCP server (registered via
`cueckoo mcp`) and is also available from the `cueckoo guidance`
CLI. At the start of every session, before doing any work in this
repo:

- If the cueckoo guidance is not already loaded in your context
  (look for a `=== BEGIN CUECKOO GUIDANCE` line), invoke the
  cueckoo MCP server's `guidance` tool now to load it.
- If a later system-reminder reports a guidance-hash that differs
  from the one in your loaded `=== BEGIN CUECKOO GUIDANCE
  (hash: ...) ===` line, re-invoke the tool to pick up the
  changes.

Treat everything between the BEGIN and END markers as authoritative
for this session.

## Project-specific instructions

This repo provides general code and tools for contributors to the CUE
project. The main command is `cueckoo`.

### Building and testing

    go build ./...
    go test ./...
    go tool staticcheck ./...
