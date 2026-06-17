# Plan: slimming the cueckoo common guidance

This is a proposal, not an implemented change. It exists to be
reviewed on GerritHub; the review is the basis for deciding next
steps. This commit changes only this document.

## Status / baseline

The MCP `guidance` tool has already been removed (it returned the
same body the `@`-import already loads, and analysis of local session
transcripts showed it dominated the cueckoo MCP token footprint
because each call re-injected the full body). This plan assumes that
baseline: the guidance now has exactly one delivery channel, the
`@`-import of `~/.cache/cueckoo/common-guidance.md`. The earlier draft
of this plan routed on-demand material to that MCP tool; this revision
replaces those routings with an on-disk reference file (see "The lever
specific to this setup").

The guidance BEGIN marker has also been changed to carry a content
fingerprint instead of the cueckoo version, so the `@`-imported text
(and the prompt cache built on it) turns over only when the guidance
content changes — not on every cueckoo release. This plan assumes that
baseline too (see "Interaction with prompt caching").

## Problem

The common guidance served by cueckoo (the `commonGuidance` string
in `cmd/cueckoo/cmd/guidance.go`) has grown to roughly 1000 lines.
It is inlined into every Claude Code session in every CUE repo via
the `@~/.cache/cueckoo/common-guidance.md` import. This risks the
well-known "over-specified CLAUDE.md" failure mode: when the file is
too long, important rules get lost in the noise and are effectively
ignored. The standard remedy is to prune ruthlessly, and to convert
rules that only need mechanical enforcement into hooks.

## Diagnosis

The issue is not primarily that rules are wrong. It is that a
reference manual is being delivered as always-on context. Breaking
the body down by how often a given session actually needs it:

- git-codereview rebase recipes (the "Working with chains" through
  "Using git codereview reword" subsections): ~340 lines. Relevant
  only when actually performing rebase surgery on a chain of
  commits.
- Meta-about-the-guidance (CLAUDE.md structure, Configuring a repo,
  Lifecycle of the cache file, Diagnosing guidance-rule slips,
  Improving this guidance): ~290 lines. Relevant only when setting
  up a new repo or doing forensics on a rule slip.
- Everyday core (commit messages, the workflow discriminator, review
  feedback, CI, community, reproductions, style, copyright, issues):
  ~370 lines. Relevant to most sessions.

So over half the file is reference material needed by a minority of
sessions, yet it is inlined every time.

## The lever specific to this setup

With the MCP tool gone, the single body is delivered one way: every
byte of `commonGuidance` is inlined into every session. The lever is
to split the baked-in content in two, by audience:

- `coreGuidance` — written to `~/.cache/cueckoo/common-guidance.md`
  and inlined via the `@`-import. The lean, always-on body.
- `referenceGuidance` — written to a sibling on-disk file (e.g.
  `~/.cache/cueckoo/guidance-reference.md`) that is NOT `@`-imported.
  The model reads it with the Read tool only when a task needs it,
  and the lean core carries explicit pointers telling it when to do
  so. Optionally also surfaced by a `cueckoo guidance --reference`
  subcommand for direct/manual use.

`cueckoo version update` already keeps the on-disk core in sync; it
would write and sync the reference file the same way. Drift detection
and `guidance --install` / `--check` extend to cover both files.

This is the on-demand channel the earlier draft assigned to the MCP
tool, re-expressed as a file the model Reads. It is strictly better
for our case: a fetched file is content not already in context, so it
does not duplicate the `@`-import (the exact redundancy that made the
MCP tool expensive), and sessions that never need it never load it.

A few rules are better enforced by a check than stated in prose: the
72-column wrap, Signed-off-by presence, Change-Id presence,
no-AI-attribution, the copyright header, and American English. These
become hooks or lints rather than text.

## Decision legend

- KEEP — stays in the lean always-on body (the `@`-import).
- TRIM — stays, but cut to its principle; detail moves to REF.
- REF — move to the on-demand reference file (read with the Read
  tool, or via `cueckoo guidance --reference`) when a natural trigger
  exists.
- DOCS — move out of loaded context entirely, into cueckoo repo docs
  or a `--help` command (setup- or maintenance-only).
- HOOK — mechanical rule better enforced by a check than by prose.

## Section-by-section

| Section | Decision | Rationale |
| --- | --- | --- |
| Repo workflows discriminator | KEEP | Selects which rules apply; everything downstream depends on it. Short. |
| Commit Messages | KEEP prose + HOOK mechanics | Keep summary form, "Fixes" placement, canonical URLs. Move wrap, sign-off presence, Change-Id presence, and no-AI-attribution to a commit-msg validator. |
| Code Review (intro) | KEEP | Short; routes GerritHub vs PR. |
| git-codereview | TRIM | Keep "use git codereview, not raw push" and the `@{u}` rule. Command cheat-sheet to REF. |
| GerritHub workflow | KEEP | Basic branch/commit/mail flow — common path. |
| Working with chains of commits | REF | Reference for an activity with a clear trigger. |
| Inserting a new commit at the start of a chain | REF | Niche recipe. |
| Editing a commit within a chain | REF | Recipe; large. |
| Extracting files into a different commit | REF | Niche recipe. |
| Preserving Change-Ids when splitting commits | TRIM to core + REF | One-line invariant stays in core; the patterns move to REF. |
| Amending a commit message during a rebase edit | REF | Recipe. |
| Keeping commit messages accurate | TRIM | Keep the principle ("after any rewrite, verify each message matches its diff") in core; mechanics to REF. |
| Using git codereview reword | REF | Recipe. |
| Preserving Change-Ids (principle) | KEEP | The load-bearing invariant that anchors every recipe. Short. |
| Addressing review feedback | TRIM | Keep the hard behavioral rules (use gerrit_comments; must post a gerrit_reply draft per thread; resolution is server-side). The patchset-SHA-vs-HEAD explanation to REF. |
| GitHub PR workflow | KEEP | The PR-only repos' main path. |
| Git worktrees | TRIM | Keep the rule (`.claude/worktrees/`, gitignored). Cut the prose justification. |
| CI (both variants) | KEEP | Short; "tests = local, trybots = CI" plus the runtrybot/Actions split. |
| Community | KEEP | Short; names the tools. |
| Testing and Reproductions | REF | Only relevant when building a txtar repro — clear trigger. Leave a one-line pointer in core. |
| Style (American English) | KEEP + optional HOOK | Keep the one-liner; an en-US lint could back it. |
| Copyright Headers | HOOK + one line | Mechanical; a header check is more reliable than prose. |
| CLAUDE.md structure | DOCS | Setup-only meta; irrelevant to working sessions. |
| Configuring a repo to use this guidance | DOCS | Setup-only; belongs in cueckoo's README or a `guidance --setup-help`. |
| Lifecycle of the cache file | DOCS | Meta about the mechanism; not needed in-session. |
| Working on issues | TRIM | Keep "read body and all comments before acting." Cut the gh command detail to one line. |
| Creating issues | TRIM | Keep "check for templates first." Trim the API detail. |
| Diagnosing guidance-rule slips | REF | Rare forensic activity; its Step 1 already starts by re-reading the on-disk guidance, which is the REF model. |
| Improving this guidance | DOCS | Meta. |

## Net result

- Always-on core (the `@`-import): the discriminator, commit
  conventions (prose), code-review routing plus the gerrit feedback
  rules, basic GerritHub and PR flows, the Change-Id invariant, CI,
  community, style, the worktree rule, issue rules, and one-line
  pointers to the reference file. Rough estimate ~300-350 lines, down
  from ~1000.
- On-demand reference file (read when needed): the full
  rebase/Change-Id recipe set, the reproduction guide, the
  review-feedback mechanics, and the slip-diagnosis protocol.
- Out of loaded context entirely (DOCS): ~190 lines of "how the
  guidance system configures itself."
- Enforced, not stated (HOOK): wrap, sign-off, Change-Id presence,
  no-AI-attribution, copyright header, American English.

## Tradeoff to weigh

The REF moves rest on a real assumption: that the model reads the
rebase reference when it begins rebase surgery. The current guidance
is full of "IMPORTANT: always set GIT_SEQUENCE_EDITOR ..." precisely
because getting this wrong is costly and silent. If the model forgets
to read the reference, it operates from training priors and the
failure mode is exactly the Change-Id loss the recipes exist to
prevent.

Two mitigations:

1. Keep the invariants (the "Preserving Change-Ids" principle and the
   one-line "never invent or replace a Change-Id; never use -m on the
   commit that keeps it") in always-on core, so the "don't" survives
   even without a read.
2. Add a single high-salience line in core: "Before any rebase,
   amend, squash, or split, read the rebase reference at
   ~/.cache/cueckoo/guidance-reference.md."

If depending on a read is undesirable, an alternative keeps the rebase
manual always-on and gets the win purely from DOCS-moving the meta
sections and HOOK-converting the mechanical rules — a smaller
reduction (~190-250 lines) but no new failure mode.

## Interaction with prompt caching

Claude Code applies Anthropic prompt caching to the system prompt
(which includes the inlined CLAUDE.md / `@`-import) and the growing
conversation prefix. The exact breakpoint placement is an
implementation detail, but the economics are stable: a cache write
costs ~1.25x a normal input token, a cache read ~0.1x, and the cache
has a ~5-minute TTL (refreshed on each use). So within a warm session
the always-on core is paid at full price once (the first-turn write)
and then re-read at ~10% on every subsequent turn.

This reshapes the rationale for slimming in four ways:

1. Caching blunts the token-cost argument, not the adherence
   argument. Once cached, the body is cheap to re-send; what slimming
   primarily buys is attention — rules not lost in the noise — and a
   cached instruction is exactly as ignorable as an uncached one. So
   the dominant reason to slim is adherence and clarity; token saving
   is secondary. (Secondary, not zero: the one-time write scales with
   size, and the 5-minute TTL means an idle gap forces a re-write, so
   in bursty interactive use a large core is re-written several times
   per session, not just once.)

2. Stability matters more than size for cache hits, which require an
   exact-prefix match. The BEGIN marker now carries a fingerprint of
   the guidance content (`=== BEGIN CUECKOO GUIDANCE (sha256:...) ===`)
   rather than the cueckoo version, so a version-only release no longer
   changes the cached prefix; the marker — and the cache — turns over
   only when the guidance content actually changes. (It previously
   embedded the cueckoo version, which busted the cache on every
   release even when the body was byte-for-byte identical.) The lesson
   for this plan still holds: any change to the always-on core carries
   a cache cost on top of the review cost, so keep the core stable and
   let the volatile or rarely-needed material live in the reference
   file.

3. The core/reference split is cache-favorable. The reference file is
   loaded mid-conversation via the Read tool, so it lands after the
   cached system-prompt prefix: editing the reference does not
   invalidate the core's cache, and the reference only enters context
   (and the cache) in the minority of sessions that touch it.

4. This is also why removing the MCP tool helped, framed through
   caching: the tool was not expensive because of a caching failure
   but because each call paid a fresh full-price write for a body the
   `@`-import had already placed in the cached system prompt — paying
   twice for the same content. Reading an on-demand reference file
   pays once, for content that is not otherwise present.

Recommendation: optimize the always-on core for stability and
adherence, not raw byte count; with the content-hashed marker the
cache now turns over only on genuine content changes, so the priority
is to minimize churn in the core itself; and keep the large, edit-prone
reference material on disk to be read on demand, so most sessions
neither load nor cache it.
