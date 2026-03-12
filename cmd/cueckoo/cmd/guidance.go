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
CLs and GitHub PRs are supported workflows. Repos that use GerritHub
have a codereview.cfg file in the repository root.

### git-codereview

CUE projects use git-codereview (golang.org/x/review/git-codereview)
for managing Gerrit changes. It is installed on all CUE maintainer
machines and available as "git codereview" (a git subcommand). Use it
for all Gerrit interactions — do not use raw git push to Gerrit.

Key commands:

    git codereview change [-a] [-s] [-m <message>] [branchname]

      Create or switch to a work branch. With no arguments, amends the
      current pending commit with any staged changes. With a branch
      name, creates or switches to that branch. Flags:
        -a    stage all tracked modified files (like git commit -a)
        -s    add Signed-off-by trailer
        -m    set commit message without opening editor
      Special: "git codereview change NNNN" downloads CL NNNN from
      Gerrit; "NNNN/PP" downloads a specific patchset.

    git codereview mail [-r reviewer,...] [-cc mail,...] [-f]
                        [-autosubmit] [-wip] [revision]

      Push pending change to Gerrit for review. Flags:
        -r    comma-separated reviewer emails
        -cc   comma-separated CC emails
        -f    force mail even with staged changes
        -autosubmit  set Auto-Submit+1
        -wip         mark as work-in-progress

    git codereview pending [-c] [-l] [-s]

      Show status of all pending changes. Flags:
        -c    current branch only
        -l    local info only (no Gerrit queries, faster)
        -s    short output

    git codereview sync

      Fetch from remote, merge upstream changes, rebase pending
      commits on top.

    git codereview rebase-work

      Interactive rebase over pending changes. Shorthand for
      git rebase -i $(git codereview branchpoint).

    git codereview reword [commit...]

      Edit pending commit messages without affecting the working tree
      or staged index. Safe to use while tests are running.

    git codereview submit [-i | revision...]

      Submit pending change through Gerrit to the upstream branch.

    git codereview branchpoint

      Print the commit hash where the current branch diverged from
      upstream. Useful for diffs: git diff $(git codereview branchpoint)

    git codereview hooks

      Install git hooks that add Change-Id trailers and check gofmt.
      Run automatically by other commands.

All commands accept -n (dry run) and -v (verbose).

### GerritHub workflow

- Create a work branch: git codereview change my-branch
- Stage changes and create a commit: git codereview change -a -s
- Send for review: git codereview mail
- The Change-Id trailer links commits to GerritHub changes — it is
  added automatically by git codereview hooks
- Download an existing CL to work on: git codereview change NNNN

### Working with chains of commits

Gerrit encourages chains of related commits on a single branch. Each
commit becomes a separate CL linked by its Change-Id.

- Add commits with git commit directly (not git codereview change)
- git codereview change (no arguments) amends the top commit
- To edit a commit further down the chain, use
  git codereview rebase-work to interactively rebase
- To edit only commit messages, use git codereview reword
- When mailing a branch with multiple commits, specify which:
  git codereview mail HEAD
- git codereview pending shows all pending commits

### Addressing review feedback

- Use the gerrit_comments MCP tool to fetch review feedback
- Focus on unresolved threads — these need action
- When /COMMIT_MSG appears as a file path in review comments, it
  refers to feedback on the commit message, not a source file
- Each review comment is like a ticket: either implement the
  suggestion or explain why not
- After making changes to the top commit: git codereview change
  then git codereview mail to send the updated patchset
- For changes to commits further down: use
  git codereview rebase-work, then git codereview mail
- If all threads are resolved, report that no action is needed

## CI (trybots)

CUE projects run CI via cueckoo runtrybot, not through Gerrit labels.

    cueckoo runtrybot

With no arguments, it derives a revision and Change-Id for each pending
commit in the current branch. If multiple pending commits are found, you
must specify which commits or CLs to run, or pass HEAD to run trybots
for all of them.

Flags:
  -f, --force     force the trybots to run, ignoring any results
  --nounity       do not simultaneously trigger a unity build

Requires a GitHub username and classic personal access token with the
"repo" scope, configured via a git credential helper or the GITHUB_USER
and GITHUB_PAT environment variables.

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
standalone .txtar reproduction files using testscript format. The txtar
format is a trivial text-based file archive where files are delimited
by "-- filename --" markers. Commands precede the archive section.

Running reproductions:

    testscript repro.txtar       # Run a reproduction
    testscript -v repro.txtar    # Verbose output
    testscript -u repro.txtar    # Auto-update golden files

Reproductions can also be piped via stdin:

    testscript <<'EOD'
    exec cue def
    -- cue.mod/module.cue --
    module: "mod.com"
    -- x.cue --
    package x
    a: 41
    a: 42
    EOD

Use the cmp command within testscript to validate output against
golden files — the tool displays diffs when output diverges.

Creating txtar archives from existing directories:

    txtar-c > repro.txtar        # Archive current directory
    txtar-x < repro.txtar        # Extract an archive

Conventions for txtar reproductions:
- Keep them self-contained and hermetic
- Omit go.sum files; use "exec go mod tidy" to populate them
- Include expected output for validation using cmp
- Reference specific commit hashes, not branch names

See https://github.com/cue-lang/cue/wiki/Creating-test-or-performance-reproducers
for full details.

## Copyright Headers

Files do not list author names. New files should use the standard
Apache 2.0 copyright header with the current year. Do not update
the copyright year for existing files that you change.

## CLAUDE.md structure

Each CUE project repo should have a CLAUDE.md file at its root. The
file should start by bootstrapping from the common guidance provided
by the cueckoo MCP server's guidance tool, then add any repo-specific
instructions. For example:

    # Project Name

    ## Common guidance

    Use the cueckoo MCP guidance tool to get the latest common guidance
    for CUE project repos. Follow all instructions returned by that tool.

    ## Project-specific instructions

    (repo-specific conventions, build commands, test instructions, etc.)

This structure ensures that common conventions are always up to date
(served dynamically by the MCP tool) while allowing each repo to layer
on its own instructions.
`
