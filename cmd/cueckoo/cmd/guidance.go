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

import (
	"crypto/sha256"
	"encoding/hex"
)

// commonGuidance is the canonical set of instructions shared across all
// CUE project repos. It is returned by the "guidance" MCP tool and can
// be used to keep per-repo CLAUDE.md files in sync.
const commonGuidance = `# CUE Project — Common Guidance

This guidance applies to all repositories in the CUE project. It is
served by the cueckoo MCP server and should be incorporated into each
repo's CLAUDE.md.

## Repo workflows: GerritHub vs GitHub-PR-only

CUE project repos use one of two code review and CI workflows. Most
of this guidance is common to both, but several sections apply only
to one. Before applying any workflow-specific section, determine
which workflow your repo uses:

- GerritHub workflow: the repo has a codereview.cfg file in its
  root identifying the GerritHub instance and GitHub mirror.
  GerritHub is the source of truth; GitHub is a mirror; CI runs
  via cueckoo runtrybot. Review happens on GerritHub CLs.
- GitHub-PR-only workflow: the repo has no codereview.cfg. Review
  happens entirely on GitHub PRs and CI runs via GitHub Actions.
  There are no Change-Ids, no git-codereview tooling, and no
  gerrit_* MCP tools.

Sections below that apply to only one workflow are marked "Applies
to: GerritHub repos" or "Applies to: GitHub-PR-only repos". Skip
sections that do not apply to the repo you are in.

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
- When referring to CUE or Go GitHub issues or GerritHub CLs from
  within the body of a commit message (the prose between the summary
  line and the trailers) or from within source code, use the
  canonical short URLs: https://cuelang.org/issue/NNN and
  https://cuelang.org/cl/NNN for CUE, and https://go.dev/issue/NNN
  and https://go.dev/cl/NNN for Go. This does not apply to the
  commit summary line, nor to the "Fixes", "Updates", or "For"
  trailing lines, which use the "#NNN" or "cue-lang/<repo>#NNN"
  forms described above
- (GerritHub repos only) All commits must include a Change-Id
  trailer. The Change-Id is what GerritHub uses to uniquely identify
  a change — see "Preserving Change-Ids" below. IMPORTANT: never
  write or invent a Change-Id yourself. Change-Ids are generated
  automatically by git codereview hooks (installed via
  git codereview hooks). Use git codereview change to create or
  amend commits and the hook will add or preserve the Change-Id.
  GitHub-PR-only repos do not use Change-Ids

## Code Review

Code review happens on GerritHub (for repos with a codereview.cfg)
or on GitHub PRs (for repos without one). See "Repo workflows"
above.

For GerritHub repos, the codereview.cfg file identifies the
GerritHub instance (which must match the origin remote) and the
GitHub mirror:

    gerrit: https://cue.gerrithub.io/a/cue-lang/<repo>
    github: https://github.com/cue-lang/<repo>

GerritHub is the source of truth; GitHub is a mirror.

The following subsections — up to and including "Addressing review
feedback" — describe the GerritHub workflow. They apply only to
repos with a codereview.cfg. Skip them for GitHub-PR-only repos
and use the "GitHub PR workflow" subsection at the end of this
section instead.

### git-codereview

CUE projects use git-codereview (golang.org/x/review/git-codereview)
for managing Gerrit changes. It is installed on all CUE maintainer
machines and available as "git codereview" (a git subcommand). Use it
for all Gerrit interactions — do not use raw git push to Gerrit.

IMPORTANT: when comparing a branch against its upstream base, always
use @{u} (git shorthand for the upstream tracking branch) rather than
hardcoding a branch name like "master" or "origin/master". There may
not be a local master branch, and the actual divergence point may
differ. For example:

    git diff @{u}
    git log --oneline @{u}..HEAD

Key commands (use "git codereview <command> -h" for full usage):

    git codereview change    create/switch branch, or amend pending commit
    git codereview mail      push pending change to Gerrit for review
    git codereview sync      fetch and rebase on upstream
    git codereview rebase-work  interactive rebase over pending changes
    git codereview reword    edit pending commit messages (safe while tests run)
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
- To edit only commit messages, use git codereview reword — see
  "Using git codereview reword" below for details
- When mailing a branch with multiple commits, specify which:
  git codereview mail HEAD
- git log @{u}..HEAD shows all pending commits

### Inserting a new commit at the start of a chain

To insert a new commit before all pending commits, create an empty
commit at HEAD, then reorder it to the front during a rebase:

    git commit --allow-empty -s -m "pkg/foo: summary"
    GIT_SEQUENCE_EDITOR="sed -i -e '/<sha>/d' -e '1i edit <sha> pkg/foo: summary'" \
      git codereview rebase-work

The rebase stops at the empty commit (now first in the chain).
Stage your changes and amend (git codereview change cannot be used
during a rebase, so use git commit --amend directly):

    git add <files>
    git commit --amend --no-edit

Then resume:

    GIT_EDITOR=true git rebase --continue

Do not try to insert a commit by marking the first existing commit
for edit and creating a new commit while paused there — that inserts
the new commit after the edit point, not before it.

### Editing a commit within a chain

git codereview rebase-work runs git rebase -i under the hood. It
opens an editor listing pending commits — each can be marked as
"pick" (keep), "edit" (stop to amend), "reword", etc.

IMPORTANT: always set GIT_SEQUENCE_EDITOR when running
git codereview rebase-work. Without it, the command uses the
default editor which may block waiting for input, or may return
immediately as a no-op (leaving all commits as "pick" and doing
nothing). Neither outcome is useful. Always set
GIT_SEQUENCE_EDITOR to a command that edits the rebase todo list
to produce the desired result.

To mark a specific commit for editing by its SHA:

    GIT_SEQUENCE_EDITOR="sed -i '/<sha>/s/pick/edit/'" \
      git codereview rebase-work

where <sha> is a short SHA (or unique prefix) from git log. The
rebase pauses at that commit with it as HEAD. Make your edits,
stage them, and amend. Use git commit --amend --no-edit for
code-only changes, or cueckoo rewrite-commit-msg as GIT_EDITOR
if the message also needs updating (see "Amending a commit message
during a rebase edit" below). Then resume:

    GIT_EDITOR=true git rebase --continue

To mark multiple commits for editing:

    GIT_SEQUENCE_EDITOR="sed -i -e '/<sha1>/s/pick/edit/' \
      -e '/<sha2>/s/pick/edit/'" git codereview rebase-work

IMPORTANT: when starting a rebase, always derive SHAs from the
current branch state. Use git log @{u}..HEAD to get current SHAs
— never use a SHA remembered from earlier in the conversation.
Rebasing rewrites commit SHAs, so any SHA noted before a rebase
is stale and must not be used. Using a stale SHA can silently
duplicate, drop, or reorder commits.

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
    git commit -a -s               # commit the unstaged hunks as a
                                   # new, separate change (hooks add
                                   # a fresh Change-Id)

This is much better than git reset HEAD^ (without --soft), which
unstages everything and forces you to rebuild the commit from
scratch — error-prone and easy to get wrong with larger changes.

### Extracting files into a different commit

When a commit has accumulated files that belong in a different
existing commit (e.g. review feedback was applied to the wrong
commit), the reset --soft HEAD^ approach can cause modify/delete
conflicts. A cleaner approach uses squash-then-split:

1. Use GIT_SEQUENCE_EDITOR to squash the offending commit into
   the commit that should receive the extra files, and insert an
   empty commit before the squashed result:

   First, create an empty commit at HEAD:

       git commit --allow-empty -s -m "temp: extract files"

   Then rebase to squash and reorder:

       GIT_SEQUENCE_EDITOR="sed -i \
         -e '/<offending>/s/pick/fixup/' \
         -e '/<empty>/d' \
         -e '/<target>/i edit <empty-sha> temp: extract files' \
         -e '/<target>/s/pick/edit/'" \
         git codereview rebase-work

   This squashes the offending commit into the target, inserts
   the empty commit before the target, and marks both for edit.

2. The rebase pauses at the empty commit. Pull in just the files
   that belong here from the next commit (the squashed target):

       git checkout <next-sha> -- path/to/file.go
       git commit --amend --no-edit

3. Continue — the rebase pauses at the squashed target commit.
   The extracted files are already applied (from step 2), so
   the commit auto-resolves with just its own changes remaining.
   Amend if the message needs updating:

       git commit --amend --no-edit
       GIT_EDITOR=true git rebase --continue

This avoids modify/delete conflicts because the extraction happens
cleanly before the target commit is replayed.

### Preserving Change-Ids when splitting commits

When splitting a commit into two during a rebase edit, one of the
resulting commits must keep the original Change-Id (to preserve the
link to the existing GerritHub CL). The other gets a new Change-Id.

Which commit keeps the original Change-Id? The commit that retains
the original purpose of the CL must keep its Change-Id. This
preserves meaningful patchset history on GerritHub — reviewers can
see how the CL evolved across patchsets. Do not assign the original
Change-Id to extracted code that serves a different purpose — that
repurposes the CL and makes the patchset history nonsensical.

The commit that keeps the original Change-Id must use
git commit -c ORIG_HEAD. The -c flag pre-populates the editor with
the original commit message including the Change-Id trailer. The
other commit uses git commit -s (or git commit -a -s) and the
codereview hooks add a fresh Change-Id automatically.

IMPORTANT: never use git commit -m "..." for the commit that must
keep the original Change-Id. The -m flag replaces the message
entirely, and the hooks generate a new Change-Id — orphaning the
original CL. The -c flag is safe because it opens an editor
pre-populated with the original message and Change-Id.

There are two patterns depending on whether the extracted code goes
before or after the original commit:

Pattern 1: extract code into a new commit AFTER the original.
The original work is committed first, keeping its Change-Id:

    git reset --soft HEAD^             # open the commit
    git reset -p HEAD somefile.go      # unstage hunks to extract
    git commit -c ORIG_HEAD            # ORIGINAL work, keeps Change-Id
    git commit -a -s                   # extracted code, fresh Change-Id

Pattern 2: extract code into a new commit BEFORE the original.
The extracted code is committed first with a fresh Change-Id,
then the original work is committed with -c ORIG_HEAD:

    git reset --soft HEAD^             # open the commit
    git reset HEAD harness.go          # unstage file to extract
    git commit -a -s                   # extracted code, fresh Change-Id
    git commit -c ORIG_HEAD            # ORIGINAL work, keeps Change-Id

Note the order in pattern 2: git commit -a -s creates the new
preceding commit from the unstaged (extracted) files, then
git commit -c ORIG_HEAD commits whatever was left staged — the
original work with its Change-Id preserved.

To drive the -c flag non-interactively, use
cueckoo rewrite-commit-msg as GIT_EDITOR:

    GIT_EDITOR="cueckoo rewrite-commit-msg -m 'cmd/foo: narrower summary

    Updated description.'" git commit -c ORIG_HEAD

When done editing, resume the rebase:

    GIT_EDITOR=true git rebase --continue

Always use GIT_EDITOR=true when resuming a rebase non-interactively.
Without it, if the next commit in the sequence triggers a merge
conflict, resolving the conflict and continuing would open an editor
for the merge commit message, blocking the automation. Setting
GIT_EDITOR=true makes it return immediately.

### Amending a commit message during a rebase edit

When paused at a commit marked for "edit" during a rebase, the
commit message can be updated as part of the amend step. Always
combine code changes and message updates into a single amend —
do not amend code with --no-edit and then amend the message
separately. Multiple amends are unnecessary and increase the risk
of Change-Id loss.

To keep the existing message unchanged (code-only edit):

    git add <files>
    git commit --amend --no-edit

To amend both code and message together, use
cueckoo rewrite-commit-msg as GIT_EDITOR:

    git add <files>
    GIT_EDITOR="cueckoo rewrite-commit-msg -m 'cmd/foo: updated summary

    New description of the change.'" git commit --amend

cueckoo rewrite-commit-msg replaces the message body with the -m
argument while preserving all trailers (Change-Id, Signed-off-by,
etc.). This is safe for any message rewrite — simple summary
changes or complete rewrites.

For minor edits (e.g. just the summary line), sed is also safe:

    git add <files>
    GIT_EDITOR="sed -i '1s/.*/cmd\/foo: updated summary/'" \
      git commit --amend

IMPORTANT: avoid git commit --amend -m "..." and
git commit --amend -F <file> when the commit has a Change-Id.
Both -m and -F replace the entire message, and the codereview
hooks will generate a new Change-Id — orphaning the original CL.
Use cueckoo rewrite-commit-msg instead.

After amending, resume the rebase with GIT_EDITOR=true as above.

### Keeping commit messages accurate

After any operation that changes the content of a commit — amending
code, moving hunks between commits, squashing, or splitting — verify
that every affected commit's message still accurately describes the
resulting change. If it does not, update it.

The preferred approach is to update each commit's message at the
point you are editing it. When paused at a commit during a rebase,
check whether the message still matches the diff and amend it as
part of the same step (see "Amending a commit message during a
rebase edit" above). This avoids a separate pass after the rebase.

If a rebase has already completed, review the full stack
(git log @{u}..HEAD) and fix any messages that no longer match
their diffs (git diff HEAD~1 for each commit).

Do not wait for the user to ask — this check must be automatic after
every rebase or edit operation that changes commit content.

How to update messages depends on context:

- During a rebase edit stop (preferred): amend the message as part
  of the amend step — this is the best time to do it since you are
  already looking at the commit.
- For the top commit (outside a rebase): use git codereview change.
- For commits deeper in the stack (outside a rebase): use
  git codereview reword.

### Using git codereview reword

git codereview reword rewrites commit messages without changing code.
It performs a rebase internally, so it cannot be used while another
rebase is in progress.

With no arguments, it rewords all pending commits. With arguments, it
rewords only the specified commits:

    git codereview reword              # reword all pending commits
    git codereview reword abc123       # reword a specific commit

It invokes GIT_EDITOR for each commit message. To drive it
non-interactively, use cueckoo rewrite-commit-msg as GIT_EDITOR:

    GIT_EDITOR="cueckoo rewrite-commit-msg -m 'cmd/foo: new summary

    New description.'" git codereview reword abc123

For minor edits (e.g. just the summary line), sed also works:

    GIT_EDITOR="sed -i '1s/.*/cmd\/foo: new summary line/'" \
      git codereview reword abc123

IMPORTANT: git codereview reword preserves Change-Ids automatically.
Do not use git codereview change -m to rewrite a message, as it
replaces the message entirely and can generate a new Change-Id.

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

IMPORTANT: for every unresolved comment thread you address, you MUST
post a draft reply using the gerrit_reply MCP tool. This is not
optional — the reviewer needs to see what action was taken for each
piece of feedback. Typical replies: "Done.", "Acknowledged.", or a
brief description of what was changed. Drafts are not published until
the user reviews them and hits Reply in Gerrit.

The workflow for each unresolved thread is:
1. Make the code or commit message change
2. Post a draft reply via gerrit_reply explaining what was done
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

When investigating review feedback, examine the code at the commit
being reviewed, not at HEAD. The reviewer's comments reference a
specific patchset — line numbers, surrounding code, and even the
existence of fields may differ from the current tip. Either rebase
to edit the commit first (which positions HEAD at that commit), or
use git show <sha>:<file> to read the file at the correct point
in history before deciding what to change.

Before making any changes, first determine which commit the feedback
applies to. A branch may have multiple pending commits, each a
separate CL. Use git log @{u}..HEAD to see
the full stack.

IMPORTANT: do not edit files or stage changes until you are
positioned at the correct commit. This is a mechanical rule with
no exceptions — it applies to ALL changes, including seemingly
trivial ones like adding a comment, fixing a typo, or adding a
TODO. There is no shortcut. Editing at HEAD and moving changes
down via stash/pop is error-prone and wastes time. The rebase-edit
workflow is always faster in practice because it avoids conflict
resolution.

If the target commit is not at the top of the stack, you must
rebase first:

1. Use GIT_SEQUENCE_EDITOR to mark the target commit as "edit":
   GIT_SEQUENCE_EDITOR="sed -i '/<sha>/s/pick/edit/'" \
     git codereview rebase-work
2. The rebase stops with that commit as HEAD
3. Now read the code, make your edits, and stage them
4. Amend the commit: git commit --amend --no-edit (code only)
   or use cueckoo rewrite-commit-msg if the message needs updating
5. GIT_EDITOR=true git rebase --continue — replay the rest

Do not make edits at the top of the stack and then try to move
them down via stash/pop or cherry-pick — this will cause conflicts
when intermediate commits touch the same code.

If the target commit is already at the top of the stack, simply
edit the working tree and run git codereview change.

### GitHub PR workflow

Applies to: GitHub-PR-only repos (no codereview.cfg).

All review happens on GitHub PRs. There are no Change-Ids, no
git-codereview tooling, and no gerrit_* MCP tools. Use the gh CLI
for PR interactions.

Basic workflow:

- Create a work branch: git checkout -b my-branch
- Stage changes and commit with sign-off: git commit -a -s
- Push to the remote: git push -u origin my-branch
- Open a PR: gh pr create

A branch with multiple commits becomes a single PR covering all of
them, not one PR per commit. To update a PR, push new commits or
amend and force-push with --force-with-lease.

Addressing review feedback:

- Fetch comments with: gh pr view <N> --comments or
  gh api repos/<owner>/<repo>/pulls/<N>/comments
- Focus on unresolved threads — these need action
- Each comment is like a ticket: either implement the suggestion or
  explain why not
- Make the code change, push a new commit (or amend and force-push),
  and reply to each thread explaining what was done — via the
  GitHub UI or gh pr review

When investigating PR review feedback, examine the code at the
commit being reviewed: review comments reference line numbers in a
specific SHA, which may differ from the current tip. Use
git show <sha>:<file> or check the PR out locally with
gh pr checkout <N>.

## CI

IMPORTANT: in the CUE project, "tests" always refers to running the
project's test suite locally (e.g. "go test ./..."). Remote CI is
always referred to as "trybots". If someone says "the tests are
failing", they mean locally — if they meant CI, they would say
"the trybots are failing".

### GerritHub repos (with codereview.cfg)

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

### GitHub-PR-only repos (no codereview.cfg)

CI runs via GitHub Actions workflows defined in .github/workflows.
There is no separate trigger step — opening or updating a PR runs
the workflows. Inspect results with:

    gh pr checks <N>            # summary of checks for PR N
    gh run view <run-id>        # details of a specific run
    gh run view --log <run-id>  # stream the logs

cueckoo runtrybot does not apply to these repos.

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
    cueckoo MCP server (via cueckoo mcp). The response includes a
    guidance-hash — if a later system-reminder reports a different
    hash, re-invoke the tool to pick up the changes. Follow all
    instructions returned by the guidance tool.

    ## Project-specific instructions

    (repo-specific conventions, build commands, test instructions, etc.)

This structure ensures that common conventions are always up to date
(served dynamically by the MCP tool) while allowing each repo to layer
on its own instructions.

## Configuring a repo to use this guidance

A CUE project repo opts in to this guidance by committing two pieces of
configuration alongside its CLAUDE.md:

1. CLAUDE.md prelude — the "Common guidance" section shown under
   "CLAUDE.md structure" above, which tells Claude to invoke the
   guidance tool and to re-invoke it when the guidance-hash changes.

2. .claude/settings.json hook — a SessionStart hook that runs
   "cueckoo guidance --hash" and injects the current hash into
   Claude's context on every new session and on "claude -c" resumes,
   so staleness can be detected without the user having to prompt.
   The hook entry (merge with any existing hooks rather than
   overwriting) is:

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

   Commit .claude/settings.json (not .claude/settings.local.json) so
   every contributor picks up the hook automatically —
   .claude/settings.local.json is per-contributor state and is not a
   shared repo asset.

A SessionStart hook alone is sufficient: "cueckoo mcp" runs over stdio
as a child of the Claude Code process, so the guidance served within a
single session cannot change. The only events that can change it — a
fresh start or "claude -c" resume — both fire SessionStart.

When asked to configure a repo to follow the cueckoo MCP guidance,
perform both steps (creating or updating CLAUDE.md and
.claude/settings.json) and report which files changed.

## Creating issues

When creating issues on any GitHub repository, always check for issue
templates first and follow them:

    gh api repos/<owner>/<repo>/contents/.github/ISSUE_TEMPLATE \
      --jq '.[].name'

If templates exist, read the relevant template to understand the
required sections, then include all required fields and labels in
the issue body. Use gh issue create with --label to set the labels
specified in the template frontmatter.

Never create an issue without checking for templates first. Failing
to follow templates makes issues harder to triage and may cause them
to be closed or ignored.

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

// commonGuidanceHash is the hex-encoded sha256 of commonGuidance. It
// is included in responses from the guidance MCP tool and the
// "cueckoo guidance" CLI so callers can detect when the underlying
// guidance has changed without re-reading the full body.
var commonGuidanceHash = func() string {
	sum := sha256.Sum256([]byte(commonGuidance))
	return hex.EncodeToString(sum[:])
}()

// formattedGuidance returns commonGuidance prefixed with a
// machine-readable guidance-hash header and an instruction for
// consumers to re-read the guidance when they see a different hash
// in a later system-reminder. Both the MCP tool handler and the
// "cueckoo guidance" CLI return this exact text.
func formattedGuidance() string {
	return "guidance-hash: " + commonGuidanceHash + "\n\n" +
		"If a later system-reminder reports a different guidance-hash, " +
		"re-read this guidance (via the cueckoo guidance MCP tool or " +
		"the \"cueckoo guidance\" CLI) to pick up the changes.\n\n" +
		"---\n\n" +
		commonGuidance
}
