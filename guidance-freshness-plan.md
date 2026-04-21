# Plan: hash-based freshness check for `cueckoo guidance`

## Problem

The `cueckoo` MCP server exposes a `guidance` tool that returns canonical
instructions contributors should follow in CUE project repos. The content
is embedded in the `cueckoo` binary. When the guidance changes:

- Long-running Claude Code sessions keep using whatever guidance text they
  retrieved earlier in the session.
- Even a full Claude Code restart is not enough: `claude -c` restores prior
  conversation context, so the stale guidance text is still in-scope.

Users currently have to notice the change and tell Claude to re-invoke the
`guidance` tool. We want that to happen automatically.

## Current state

- Guidance is an embedded Go constant `commonGuidance` in
  `cmd/cueckoo/cmd/guidance.go` (~657 lines).
- Only consumer today is the MCP tool handler `handleGuidance` in
  `cmd/cueckoo/cmd/mcp.go:279-285`, returning it as a `TextContent` block.
- No CLI `guidance` subcommand exists.
- `.claude/settings.json` has no hooks; each repo's `CLAUDE.md` tells
  Claude to invoke the MCP tool.

## Approach

1. Include a `guidance-hash` (sha256 of the guidance text) in the MCP tool
   response, plus a short instruction telling Claude what to do if it
   later sees a different hash.
2. Ship a cheap `cueckoo guidance --hash` CLI that prints just the hash.
3. Wire a Claude Code `SessionStart` hook that runs `cueckoo guidance
   --hash` and injects the current hash into Claude's context on every
   new or resumed session. Claude compares this to whatever hash was in
   the last `guidance` tool response it remembers and re-invokes the tool
   if they differ.

This gives us the property that on `claude -c` (and on any fresh session),
Claude always sees the current hash in-context and can self-correct.

---

## Piece 1 — Compute hash once, share between CLI and MCP

In `cmd/cueckoo/cmd/guidance.go`, alongside `commonGuidance`:

```go
import (
    "crypto/sha256"
    "encoding/hex"
)

var commonGuidanceHash = func() string {
    sum := sha256.Sum256([]byte(commonGuidance))
    return hex.EncodeToString(sum[:])
}()
```

Full 64-char hex — cheap, unambiguous, no collision risk worth optimising
for.

## Piece 2 — Embed hash in the MCP tool response

Modify `handleGuidance` in `cmd/cueckoo/cmd/mcp.go` so the returned text
starts with a machine- and human-readable header:

```
guidance-hash: <hex>

If a later system-reminder reports a different guidance-hash, re-invoke
this tool to get the updated guidance.

---

<existing commonGuidance body>
```

Claude has a concrete hash to remember and an explicit instruction for
what to do when it sees a different one.

## Piece 3 — New `cueckoo guidance` CLI subcommand

Add a cobra subcommand (new file, e.g. `cmd/cueckoo/cmd/guidance_cmd.go`
— keeping `guidance.go` as data-only):

- `cueckoo guidance` — prints the same text the MCP tool returns
  (including the header). Useful for humans, for debugging, and for any
  non-MCP consumer of the guidance.
- `cueckoo guidance --hash` — prints only `commonGuidanceHash` + newline.
  Cheap, shell-scriptable, zero-dependency; this is what the hook calls.

## Piece 4 — Hook wiring in `.claude/settings.json`

Add a `SessionStart` hook in the repo's per-project
`.claude/settings.json` (the committed one, not `settings.local.json`),
so every contributor who clones the repo picks it up automatically. This
assumes `cueckoo` is on `PATH`, which is already the precondition for
the MCP server registration, so no new assumption. The hook's stdout
becomes additional context for Claude on new sessions **and** on
`claude -c` resumes:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "printf 'Current cueckoo guidance-hash: %s\\n' \"$(cueckoo guidance --hash)\""
          }
        ]
      }
    ]
  }
}
```

`SessionStart` on its own is sufficient. `cueckoo mcp` runs over stdio
as a child process of the Claude Code process, so the MCP server uses
whichever `cueckoo` binary was on `PATH` when Claude started and keeps
using it for the life of that Claude process. Within a single session
the guidance hash cannot change; the only events that could change it
— a fresh start and `claude -c` resume — both fire `SessionStart`. A
`UserPromptSubmit` hook would add cost with no additional coverage.

## Piece 5 — Update `CLAUDE.md` (small)

Extend the existing note so Claude is explicitly aware of the hash
mechanism:

> Use the cueckoo MCP server's `guidance` tool to get the latest common
> guidance for CUE project repos. The response includes a
> `guidance-hash`; if a later system-reminder shows a different hash,
> re-invoke the tool.

## Piece 6 — Make the guidance self-bootstrapping

The `commonGuidance` body itself should include a "How to configure a
repo to use this guidance" section that tells Claude exactly what to do
when it is invoked in a repo that is not yet wired up. Concretely, that
section should instruct Claude to:

1. Ensure the repo's `CLAUDE.md` contains the guidance prelude (the
   Piece 5 wording), creating or updating it as needed.
2. Ensure the repo's `.claude/settings.json` contains the `SessionStart`
   hook from Piece 4, merging with any existing hooks rather than
   overwriting.
3. Leave `.claude/settings.local.json` alone (that's per-contributor
   state, not repo state).

With this in place, adopting the mechanism in a new repo is a one-shot
ask: a contributor who already has `cueckoo mcp` working can open an
un-configured repo, say something like *"please update this project to
follow the cueckoo MCP guidance"*, and Claude will invoke the `guidance`
tool and perform the wiring itself. No hand-copied JSON, no separate
onboarding doc to keep in sync.

This also means the set of files a repo needs to commit
(`CLAUDE.md` prelude + `.claude/settings.json` hook) is defined in
exactly one place — the guidance text — so when we evolve the hook
command or the prelude wording, every repo picks up the change the next
time a contributor asks Claude to re-verify the configuration.

---

## Open questions

1. **Hash length.** Full 64-char hex is cheap and unambiguous; short hex
   (say 12 chars) is prettier in logs. No strong reason to shorten.
2. **Does this generalise?** If this works, the same header+hook pattern
   could version any cueckoo MCP tool whose output represents canonical
   instructions. Out of scope here, but worth noting.

## Rollout order

1. Piece 1 + 3 (hash var + CLI subcommand) — smallest, self-contained CL.
2. Piece 2 + 6 (MCP response header + self-bootstrapping section in the
   guidance body) — one CL, since they both edit the guidance text /
   handler.
3. Piece 4 + 5 (hook + CLAUDE.md) in this repo — final CL, once the
   binary side is released and contributors have picked up the new
   `cueckoo`. Other repos pick it up on demand via Piece 6.
