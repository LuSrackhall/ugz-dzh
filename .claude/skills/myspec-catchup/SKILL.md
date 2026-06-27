---
name: myspec-catchup
description: "Sync worktree with latest main and run lightweight checks. Use to catch up with main changes before merging, or as a standalone check."
---

# myspec-catchup

Sync the worktree branch with the latest main, then run lightweight checks to confirm the implementation still works. Can be used standalone or called by myspec-merge.

**Input**: Optionally specify a change name. If omitted, check conversation context or prompt for selection.

## Steps

0. **Worktree guard**

   Verify the user is currently in a git worktree (not on main directly):

   ```bash
   BRANCH=$(git branch --show-current)
   ```

   If the branch is `main` or `master`:
   > "myspec-catchup must be run from a worktree. You are currently on main. Run myspec-apply first to start implementation in a worktree."

   Stop.

1. **Select the change**

   If a name is provided, use it. Otherwise:
   - Infer from conversation context
   - Auto-select if only one active change exists
   - If ambiguous, run `openspec list --json` and use AskUserQuestion

   Announce: "Using change: <name>"

2. **Main sync check**

   Check if local main and origin/main are in sync:

   ```bash
   git fetch origin
   LOCAL_MAIN=$(git rev-parse main)
   ORIGIN_MAIN=$(git rev-parse origin/main)
   ```

   **If origin/main is ahead of local main:**
   > "origin/main has N new commit(s) that local main does not have. Should I pull to update local main?"

   Use AskUserQuestion. If user confirms:
   ```bash
   git checkout main
   git pull origin main
   git checkout change/<name>
   ```

   Then ask the user how to sync main into the worktree branch:

   | Option | Description | When to use |
   |--------|-------------|-------------|
   | Rebase (recommended) | Replay worktree commits on top of latest main | Keeps linear history, preferred for feature branches |
   | Merge | Merge main into worktree branch | Preserves exact commit history, creates merge commit |

   Use AskUserQuestion. Based on user choice:

   **Rebase:**
   ```bash
   git rebase main
   ```
   If conflicts arise during rebase, resolve them in the worktree. Report conflicts to the user and assist with resolution.

   **Merge:**
   ```bash
   git merge main
   ```
   If conflicts arise during merge, resolve them in the worktree. Report conflicts to the user and assist with resolution.

   **If local main is ahead of origin/main:**
   > "Local main has N new commit(s) not pushed to origin. Should I push to origin?"

   Use AskUserQuestion. If user confirms:
   ```bash
   git checkout main
   git push origin main
   git checkout change/<name>
   ```

   Then also ask the user how to sync main into the worktree branch (same rebase/merge choice as above).

   **If in sync:**
   > "Local main and origin/main are in sync. Skipping sync step."

   **IMPORTANT:** All main branch operations (pull, push) MUST be confirmed by the user. Never execute main branch operations without explicit user approval.

3. **Post-sync validation**

   After syncing main into the worktree (or if already in sync), run lightweight checks to confirm the implementation still works:

   a. **Detect and run the project's test/build command:**
   - `go.mod` → `go build ./... && go test ./...`
   - `package.json` → `npm run build && npm test` (or `npm run check`)
   - `Makefile` → `make build && make test` (or `make check`)
   - If no test/build command found, skip and note: "No test/build command detected. Skipping validation."

   b. **If tests/build FAIL:**
   > "Post-sync validation failed. The sync may have introduced issues."
   > Show the failure output.
   > **Do NOT proceed.** The user must fix the issues first.

   c. **If tests/build PASS:**
   > "Post-sync validation passed."

4. **Completion**

   If called from myspec-merge:
   > "Catchup complete. Worktree is up to date with main and validated."
   → Return control to myspec-merge (proceed to merge method selection).

   If called standalone:
   > "Catchup complete. Worktree is up to date with main and validated."
   > "When ready to merge, run myspec-merge skill."

## Guardrails

- MUST be run from a worktree, not from main
- All main branch operations (pull, push) MUST be confirmed by the user
- Resolve merge/rebase conflicts in the worktree when possible
- Do NOT skip post-sync validation when a build/test command is available — syncing may introduce issues
- Use lightweight checks (build + test), NOT full myspec-verify
- If called standalone, report completion and suggest myspec-merge. Do NOT proceed to merge automatically.
