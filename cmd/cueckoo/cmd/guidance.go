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

// commonGuidance is the canonical set of instructions shared across all
// CUE project repos. It is returned by the "guidance" MCP tool and can
// be used to keep per-repo CLAUDE.md files in sync.
const commonGuidance = `# CUE Project — Common Guidance

This guidance applies to all repositories in the CUE project. It is
served by the cueckoo MCP server and should be incorporated into each
repo's CLAUDE.md.

## Commit Messages

Commit messages follow specific conventions. The first line is a short
summary prefixed by the primary affected package or area, e.g.:

    cue/ast/astutil: fix resolution bugs
    cmd/cue: support new export flag
    internal/core: optimize unification

The first line should complete the sentence "This change modifies CUE
to ___." — it does not start with a capital letter, is not a complete
sentence, and summarises the result of the change.

Follow the first line with a blank line, then a description that
provides context and explains what the change does. Write in complete
sentences with correct punctuation. Do not use markdown or other markup.

Additional conventions:
- Include a Signed-off-by line (use git commit -s or git codereview
  change -s) to assert the Developer Certificate of Origin
- Always use --no-gpg-sign (or -c commit.gpgsign=false) to skip GPG
  signing, which requires interactive input
- No AI authorship attribution in commit messages
- Reference issues with "Fixes #NNN" (closes the issue on submit) or
  "Updates #NNN" (links without closing). For subrepositories, use
  the fully-qualified form: "Fixes cue-lang/cue#NNN"
- All commits must include a Change-Id trailer (added automatically
  by git codereview change). The Change-Id is what GerritHub uses to
  uniquely identify a change — it must never be modified or removed
  when amending, rebasing, or editing a commit. Changing it creates
  a new GerritHub change instead of updating the existing one

## Code Review

All CUE project repos use GerritHub for code review. Both GerritHub
CLs and GitHub PRs are supported workflows.

### GerritHub workflow

- Create changes with: git codereview change -s
- Send for review with: git codereview mail
- Revise after feedback with: git codereview change (amends the
  commit) then git codereview mail
- Keep a single commit per branch; squash if you accidentally create
  multiple commits
- The Change-Id trailer links commits to GerritHub changes

### Addressing review feedback

- Use the gerrit_comments MCP tool to fetch review feedback
- Focus on unresolved threads — these need action
- When /COMMIT_MSG appears as a file path in review comments, it
  refers to feedback on the commit message, not a source file
- Each review comment is like a ticket: either implement the
  suggestion or explain why not
- If all threads are resolved, report that no action is needed

## Community

The CUE community uses Slack, Discord, and GitHub Discussions:

- Slack: CUE community workspace (https://cuelang.slack.com)
- Discord: CUE Discord server
- GitHub Discussions: https://github.com/cue-lang/cue/discussions

Use the slack_thread and discord_thread MCP tools to fetch conversation
context when helping with community questions.

When drafting responses:
- Output as raw markdown suitable for copy-paste into the target
  platform
- Reference existing cuelang.org pages with full URLs where relevant
- Include working CUE examples where helpful
- Keep answers concise but complete

Content gaps (topics that should be documented on cuelang.org but
aren't yet) are tracked at:
https://github.com/cue-lang/docs-and-content/issues

## Testing and Reproductions

When investigating CUE behaviour or community-reported issues, create
standalone .txtar reproduction files using testscript format:

    testscript repro.txtar       # Run a reproduction
    testscript -v repro.txtar    # Verbose output
    testscript -u repro.txtar    # Auto-update golden files

This is useful for quickly validating CUE behaviour without setting
up a full module or test harness.

## Copyright Headers

Files do not list author names. New files should use the standard
Apache 2.0 copyright header with the current year. Do not update
the copyright year for existing files that you change.
`
