# Plan: slimming the cueckoo common guidance

This is a proposal, not an implemented change. It exists to be
reviewed on GerritHub; the review is the basis for deciding next
steps. No guidance content or tool behavior is changed by this
commit.

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

`formattedGuidance()` feeds the same `commonGuidance` string to both
delivery channels:

- the `@`-import, inlined into every session before any model turn;
  and
- the MCP `guidance` tool, fetched on demand.

These are currently redundant. The high-value move is to stop
serving identical content on both, and split the constant in two:

- `coreGuidance` — served by the `@`-import and the tool (always
  available).
- `referenceGuidance` — served by the tool only. The tool returns
  `core + reference` (a superset); the import returns `core` alone.

Drift detection and `guidance --install` / `--check` then operate on
`core` (what is on disk). This single code change unlocks roughly
half the size reduction.

A few rules are better enforced by a check than stated in prose: the
72-column wrap, Signed-off-by presence, Change-Id presence,
no-AI-attribution, the copyright header, and American English. These
become hooks or lints rather than text.

## Decision legend

- KEEP — stays in the lean always-on body (the `@`-import).
- TRIM — stays, but cut to its principle; detail moves to the tool.
- TOOL — move to the MCP `guidance` tool, fetched on demand (a
  natural trigger exists).
- DOCS — move out of loaded context entirely, into cueckoo repo docs
  or a `--help` command (setup- or maintenance-only).
- HOOK — mechanical rule better enforced by a check than by prose.

## Section-by-section

| Section | Decision | Rationale |
| --- | --- | --- |
| Repo workflows discriminator | KEEP | Selects which rules apply; everything downstream depends on it. Short. |
| Commit Messages | KEEP prose + HOOK mechanics | Keep summary form, "Fixes" placement, canonical URLs. Move wrap, sign-off presence, Change-Id presence, and no-AI-attribution to a commit-msg validator. |
| Code Review (intro) | KEEP | Short; routes GerritHub vs PR. |
| git-codereview | TRIM | Keep "use git codereview, not raw push" and the `@{u}` rule. Command cheat-sheet to TOOL. |
| GerritHub workflow | KEEP | Basic branch/commit/mail flow — common path. |
| Working with chains of commits | TOOL | Reference for an activity with a clear trigger. |
| Inserting a new commit at the start of a chain | TOOL | Niche recipe. |
| Editing a commit within a chain | TOOL | Recipe; large. |
| Extracting files into a different commit | TOOL | Niche recipe. |
| Preserving Change-Ids when splitting commits | TRIM to core + TOOL | One-line invariant stays in core; the patterns move to TOOL. |
| Amending a commit message during a rebase edit | TOOL | Recipe. |
| Keeping commit messages accurate | TRIM | Keep the principle ("after any rewrite, verify each message matches its diff") in core; mechanics to TOOL. |
| Using git codereview reword | TOOL | Recipe. |
| Preserving Change-Ids (principle) | KEEP | The load-bearing invariant that anchors every recipe. Short. |
| Addressing review feedback | TRIM | Keep the hard behavioral rules (use gerrit_comments; must post a gerrit_reply draft per thread; resolution is server-side). The patchset-SHA-vs-HEAD explanation to TOOL. |
| GitHub PR workflow | KEEP | The PR-only repos' main path. |
| Git worktrees | TRIM | Keep the rule (`.claude/worktrees/`, gitignored). Cut the prose justification. |
| CI (both variants) | KEEP | Short; "tests = local, trybots = CI" plus the runtrybot/Actions split. |
| Community | KEEP | Short; names the tools. |
| Testing and Reproductions | TOOL | Only relevant when building a txtar repro — clear trigger. Leave a one-line pointer in core. |
| Style (American English) | KEEP + optional HOOK | Keep the one-liner; an en-US lint could back it. |
| Copyright Headers | HOOK + one line | Mechanical; a header check is more reliable than prose. |
| CLAUDE.md structure | DOCS | Setup-only meta; irrelevant to working sessions. |
| Configuring a repo to use this guidance | DOCS | Setup-only; belongs in cueckoo's README or a `guidance --setup-help`. |
| Lifecycle of the cache file | DOCS | Meta about the mechanism; not needed in-session. |
| Working on issues | TRIM | Keep "read body and all comments before acting." Cut the gh command detail to one line. |
| Creating issues | TRIM | Keep "check for templates first." Trim the API detail. |
| Diagnosing guidance-rule slips | TOOL | Rare forensic activity; the section opens with "reload the guidance," which is the tool model. |
| Improving this guidance | DOCS | Meta. |

## Net result

- Always-on core (the `@`-import): the discriminator, commit
  conventions (prose), code-review routing plus the gerrit feedback
  rules, basic GerritHub and PR flows, the Change-Id invariant, CI,
  community, style, the worktree rule, issue rules, and one-line
  pointers to the tool. Rough estimate ~300-350 lines, down from
  ~1000.
- On-demand (the `guidance` tool, a superset): the full
  rebase/Change-Id recipe set, the reproduction guide, the
  review-feedback mechanics, and the slip-diagnosis protocol.
- Out of loaded context entirely (DOCS): ~190 lines of "how the
  guidance system configures itself."
- Enforced, not stated (HOOK): wrap, sign-off, Change-Id presence,
  no-AI-attribution, copyright header, American English.

## Tradeoff to weigh

The TOOL moves rest on a real assumption: that the model fetches the
rebase reference when it begins rebase surgery. The current guidance
is full of "IMPORTANT: always set GIT_SEQUENCE_EDITOR ..." precisely
because getting this wrong is costly and silent. If the model
forgets to fetch, it operates from training priors and the failure
mode is exactly the Change-Id loss the recipes exist to prevent.

Two mitigations:

1. Keep the invariants (the "Preserving Change-Ids" principle and the
   one-line "never invent or replace a Change-Id; never use -m on the
   commit that keeps it") in always-on core, so the "don't" survives
   even without a fetch.
2. Add a single high-salience line in core: "Before any rebase,
   amend, squash, or split, fetch the rebase reference via the
   guidance tool."

If depending on a fetch is undesirable, an alternative keeps the
rebase manual always-on and gets the win purely from DOCS-moving the
meta sections and HOOK-converting the mechanical rules — a smaller
reduction (~190-250 lines) but no new failure mode.
