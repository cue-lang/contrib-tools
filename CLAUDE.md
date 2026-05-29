# contrib-tools

<!-- The CUE project common guidance is imported below, managed by
     `cueckoo`. If the referenced file is missing on your machine,
     run `cueckoo version update` to write it (and pick up any
     newer `cueckoo` while you are at it). See
     https://github.com/cue-lang/cue/issues/4355 for context. -->
@~/.cache/cueckoo/common-guidance.md

## Project-specific instructions

This repo provides general code and tools for contributors to the CUE
project. The main command is `cueckoo`.

### Building and testing

    go build ./...
    go test ./...
    go tool staticcheck ./...
