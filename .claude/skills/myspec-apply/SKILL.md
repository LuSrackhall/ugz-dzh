---
name: myspec-apply
description: "Implement tasks from an OpenSpec change with automatic task-group commits. Wraps openspec-apply-change with conventional commit behavior and post-implementation guidance."
---

# myspec-apply

Implement tasks from an OpenSpec change. Wraps OpenSpec's apply workflow with automatic git commits per task group.

**Input**: Optionally specify a change name. If omitted, check conversation context or prompt for selection.

## Steps

1. **Select the change**

   If a name is provided, use it. Otherwise:
   - Infer from conversation context
   - Auto-select if only one active change exists
   - If ambiguous, run `openspec list --json` and use AskUserQuestion

   Announce: "Using change: <name>"

2. **Get context and tasks**

   ```bash
   openspec instructions apply --change "<name>" --json
   ```

   Parse the JSON to get:
   - `contextFiles`: artifact paths to read
   - `tasks`: task list with status
   - `state`: if "blocked", suggest `/opsx:continue`; if "all_done", suggest myspec-verify

   Read all context files from `contextFiles` before starting implementation.

3. **Implement tasks by task group**

   Tasks are grouped under `## N.` headings in tasks.md. For each task group:

   a. **Show which group is being worked on:**
   ```
   Working on task group N: <group description>
   ```

   b. **Implement all tasks in the group:**
   - Work through each `- [ ]` task
   - Make minimal, focused code changes
   - Mark each task complete immediately: `- [ ]` → `- [x]`

   c. **After completing all tasks in the group, commit:**
   ```bash
   git add -A && git commit -m "<type>(<scope>): <description>"
   ```

   **Commit message rules:**
   - Format: conventional commit (`feat`, `fix`, `docs`, `refactor`, `test`, `chore`)
   - Scope: infer from change name or task group topic
   - Description: use the user's preferred language. If no language preference is established, use English.
   - Keep it concise (one line)

4. **After all task groups are complete**

   Show completion summary:
   ```
   ## Implementation Complete

   **Change:** <name>
   **Progress:** N/N tasks complete

   ### Completed This Session
   - [x] Task group 1: ...
   - [x] Task group 2: ...
   ```

   Then prompt: **"All tasks complete. Run myspec-verify skill to verify implementation and get user acceptance."**

   **Do NOT** run build, test, merge, archive, or any other post-implementation action.

## Guardrails

- Always read context files before starting implementation
- If a task is unclear, pause and ask for clarification
- If implementation reveals design issues, pause and suggest updating artifacts
- Keep code changes minimal and scoped to each task
- Mark each task checkbox immediately after completing it
- Commit after each task group, not after each task and not after all tasks
- Do NOT defer checkbox marking to the end of implementation
- Do NOT perform any post-implementation actions beyond prompting for myspec-verify
