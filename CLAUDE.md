# contrib-tools

## Common guidance

Use the cueckoo MCP server's `guidance` tool to get the latest common
guidance for CUE project repos. The server is registered as the
`cueckoo` MCP server (via `cueckoo mcp`). Follow all instructions
returned by the `guidance` tool.

## Project-specific instructions

This repo provides general code and tools for contributors to the CUE
project. The main command is `cueckoo`.

### Building and testing

    go build ./...
    go test ./...
    go tool staticcheck ./...
