# contrib-tools

## Common guidance

Use the cueckoo MCP guidance tool to get the latest common guidance
for CUE project repos. Follow all instructions returned by that tool.

## Project-specific instructions

This repo provides general code and tools for contributors to the CUE
project. The main command is `cueckoo`.

### Building and testing

    go build ./...
    go test ./...
    go tool staticcheck ./...
