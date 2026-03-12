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

    cue/ast/astutil: fix scope resolution for let clauses in comprehensions
    cmd/cue: add --out flag to export for controlling output format
    internal/core/adt: reduce allocations during unification of large structs

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
- All commits must include a Change-Id trailer. The Change-Id is
  what GerritHub uses to uniquely identify a change — see
  "Preserving Change-Ids" below. IMPORTANT: never write or invent a
  Change-Id yourself. Change-Ids are generated automatically by
  git codereview hooks (installed via git codereview hooks). Use
  git codereview change to create or amend commits and the hook
  will add or preserve the Change-Id

## Code Review

All CUE project repos use GerritHub for code review. Both GerritHub
CLs and GitHub PRs are supported workflows.

Repos that use GerritHub have a codereview.cfg file in the repository
root. This file identifies the GerritHub instance (which must match
the origin remote) and the GitHub mirror:

    gerrit: https://cue.gerrithub.io/a/cue-lang/<repo>
    github: https://github.com/cue-lang/<repo>

GerritHub is the source of truth; GitHub is a mirror.

### git-codereview

CUE projects use git-codereview (golang.org/x/review/git-codereview)
for managing Gerrit changes. It is installed on all CUE maintainer
machines and available as "git codereview" (a git subcommand). Use it
for all Gerrit interactions — do not use raw git push to Gerrit.

IMPORTANT: when comparing a branch against its upstream base, always
use "git codereview branchpoint" rather than hardcoding a branch name
like "master" or "origin/master". There may not be a local master
branch, and the actual divergence point may differ. For example:

    git diff $(git codereview branchpoint)
    git log --oneline $(git codereview branchpoint)..HEAD

Key commands (use "git codereview <command> -h" for full usage):

    git codereview change    create/switch branch, or amend pending commit
    git codereview mail      push pending change to Gerrit for review
    git codereview sync      fetch and rebase on upstream
    git codereview rebase-work  interactive rebase over pending changes
    git codereview reword    edit pending commit messages (safe while tests run)
    git codereview branchpoint  print where branch diverged from upstream
    git codereview hooks     install Change-Id and gofmt hooks

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
- To edit only commit messages, always use git codereview reword.
  Do not use git codereview change -m to rewrite a message, as it
  replaces the message entirely and can generate a new Change-Id
- When mailing a branch with multiple commits, specify which:
  git codereview mail HEAD
- git log $(git codereview branchpoint)..HEAD shows all pending commits

### Editing a commit within a chain

Use git codereview rebase-work, mark the target commit as "edit",
then make your changes and amend with git codereview change.

To extract changes from a commit (e.g. to move part of it into a
new commit before or after), use git reset --soft HEAD^ to open the
commit while keeping all its changes staged. Then selectively unstage
what you want to extract:

    git reset HEAD <file>          # unstage an entire file
    git reset -p HEAD <file>       # unstage selected hunks from a file

The unstaged changes are still in the working tree — they are not
lost. What remains staged is what stays in this commit; what was
unstaged can be committed separately. This means you can move
individual hunks within a single file to a different commit:

    git reset -p HEAD somefile.go  # unstage just the hunks to move
    git commit -c ORIG_HEAD        # commit the rest (preserving the
                                   # original message and Change-Id)
    git commit -a                  # commit the unstaged hunks as a
                                   # new, separate change

This is much better than git reset HEAD^ (without --soft), which
unstages everything and forces you to rebuild the commit from
scratch — error-prone and easy to get wrong with larger changes.

When done editing, run git rebase --continue to replay the rest of
the chain.

### Preserving Change-Ids

IMPORTANT: every commit's Change-Id is a permanent link to its
GerritHub CL. Changing a Change-Id creates a new CL instead of
updating the existing one, which breaks review history.

Change-Ids must be preserved across any operation that rewrites
commits — amending, rebasing, squashing, or resetting and rebuilding
a branch from scratch. If you reset a branch and recreate commits
(rather than using git codereview rebase-work), you must ensure the
resulting stack of commits keeps the same Change-Ids as the starting
state. Before any such operation, note the Change-Id of each pending
commit (via git log) and verify they are
unchanged afterwards.

The safest approach is to use git codereview rebase-work for editing
commits within a chain, as it preserves Change-Ids automatically.

### Addressing review feedback

- Use the gerrit_comments MCP tool to fetch review feedback
- Focus on unresolved threads — these need action
- When /COMMIT_MSG appears as a file path in review comments, it
  refers to feedback on the commit message, not a source file
- Each review comment is like a ticket: either implement the
  suggestion or explain why not
- If all threads are resolved, report that no action is needed
- Thread resolution is a GerritHub-side state: a thread is resolved
  when a reviewer has seen a new patchset on GerritHub and accepted
  that it addresses their feedback. Editing files locally does not
  resolve threads — the changes must be mailed (git codereview mail)
  and reviewed first

Review comments refer to a specific patchset (version) of a commit
on GerritHub. The commit SHA for that patchset is included in the
gerrit_comments output. Your local commit may have been amended or
rebased since that patchset was uploaded, so the commit SHA will
differ and the file paths and line numbers in the comments may not
match your working tree exactly. Use the comment context (file path,
surrounding code, and the reviewer's description) to locate the
relevant code rather than relying on exact line numbers.

Before making any changes, first determine which commit the feedback
applies to. A branch may have multiple pending commits, each a
separate CL. Use git log $(git codereview branchpoint)..HEAD to see
the full stack.

IMPORTANT: do not edit files or stage changes until you are
positioned at the correct commit. If the target commit is not at
the top of the stack, you must rebase first:

1. git codereview rebase-work — mark the target commit as "edit"
2. The rebase stops with that commit as HEAD
3. Now read the code, make your edits, and stage them
4. git codereview change — amend the commit
5. git rebase --continue — replay the remaining commits

Do not make edits at the top of the stack and then try to move
them down via stash/pop or cherry-pick — this will cause conflicts
when intermediate commits touch the same code.

If the target commit is already at the top of the stack, simply
edit the working tree and run git codereview change.

## CI (trybots)

IMPORTANT: in the CUE project, "tests" always refers to running the
project's test suite locally (e.g. "go test ./..."). Remote CI is
always referred to as "trybots". If someone says "the tests are
failing", they mean locally — if they meant CI, they would say
"the trybots are failing".

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
- Include working CUE examples where helpful
- Keep answers concise but complete

## Testing and Reproductions

When investigating CUE behaviour or community-reported issues, create
standalone .txtar reproduction files using testscript format. The
testscript CLI used is github.com/rogpeppe/go-internal/cmd/testscript.
The txtar format is a trivial text-based file archive where files are
delimited by "-- filename --" markers. Commands precede the archive
section.

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

    Use the cueckoo MCP server's guidance tool to get the latest common
    guidance for CUE project repos. The server is registered as the
    cueckoo MCP server (via cueckoo mcp). Follow all instructions
    returned by the guidance tool.

    ## Project-specific instructions

    (repo-specific conventions, build commands, test instructions, etc.)

This structure ensures that common conventions are always up to date
(served dynamically by the MCP tool) while allowing each repo to layer
on its own instructions.

## Improving this guidance

If in the course of following this guidance you find it to be
incorrect, misleading, incomplete, or have suggestions for how it
could be improved, prompt the user to raise an issue at:

    https://github.com/cue-lang/cue/issues

Use a "contrib-tools:" subject prefix and describe the problem with
the current guidance and the suggested improvement. This is a public
issue tracker — do not include private details, credentials, or
other sensitive information in the report.
`
