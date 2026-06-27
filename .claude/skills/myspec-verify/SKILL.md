---
name: myspec-verify
description: "Verify implementation, get user acceptance, and handle iteration. Wraps openspec-verify-change with user acceptance checkpoint and iteration decision loop."
---

# myspec-verify

Verify implementation against change artifacts, present results to the user for acceptance, and handle iteration if the user is not satisfied.

**Input**: Optionally specify a change name. If omitted, check conversation context or prompt for selection.

## Steps

1. **Select the change**

   If a name is provided, use it. Otherwise:
   - Infer from conversation context
   - Auto-select if only one active change exists
   - If ambiguous, run `openspec list --json` and use AskUserQuestion

   Announce: "Using change: <name>"

2. **Get context files**

   ```bash
   openspec instructions apply --change "<name>" --json
   ```

   Read all files from `contextFiles` (brainstorm-spec, proposal, specs, design, tasks).

3. **Phase 1: Document verification**

   Perform three-dimensional verification:

   **Completeness:**
   - Check all tasks.md checkboxes: `- [x]` vs `- [ ]`
   - Check delta spec requirements against codebase for coverage

   **Correctness:**
   - Map each requirement to implementation evidence in code
   - Check scenario coverage

   **Coherence:**
   - Verify implementation follows design.md decisions
   - Check code pattern consistency

   Record findings as CRITICAL / WARNING / SUGGESTION.

4. **Phase 2: User acceptance**

   Present a change summary to the user:

   ```
   ## Verification Summary

   **Change:** <name>

   | Dimension | Status |
   |-----------|--------|
   | Completeness | X/Y tasks, N reqs covered |
   | Correctness | M/N reqs implemented |
   | Coherence | Issues found / Clean |

   ### Key Changes
   - <file>: <what changed>
   - ...

   ### Issues (if any)
   - CRITICAL: ...
   - WARNING: ...
   ```

   Then ask: **"代码实现是否解决了你最初提出的问题？如果接受，我将回补文档并继续合并。"**

   (Translation: "Does the code implementation solve the problem you originally raised? If accepted, I will backfill documentation and proceed to merge.")

   **IMPORTANT:** This question is about CODE FUNCTIONALITY, not documentation. The documentation will be backfilled AFTER user acceptance. Make this distinction clear to the user.

5. **Phase 3a: User accepts**

   Backfill ALL artifacts to match the final implementation. Do NOT skip any artifact.

   For EACH artifact, read it, compare against the actual implementation, and update:

   1. **brainstorm-spec.md** — update Context/Decisions/Risks to match what was actually built
   2. **proposal.md** — update What Changes/Capabilities/Impact to match actual scope
   3. **specs/** — update each delta spec to reflect actual requirements implemented
   4. **design.md** — update Decisions to match actual implementation approach
   5. **tasks.md** — update task list to match all tasks actually completed (add missing, remove unused)

   **IMPORTANT:** You MUST check EVERY artifact, not just the ones you think changed.
   Implementation often diverges from the original plan in ways that affect multiple artifacts.

   After updating, verify completeness:
   - List all artifacts and confirm each was reviewed and updated
   - If any artifact was not touched, review it again

   Commit the backfilled artifacts:
   ```bash
   git add -A && git commit -m "docs: backfill artifacts to match implementation"
   ```

   Then prompt: **"Artifacts updated. Run myspec-merge skill to sync with main, merge, and archive."**

6. **Phase 3b: User does not accept**

   a. **Analyze the root cause:**
   - What went wrong?
   - Is it a minor implementation issue or a fundamental approach problem?

   b. **Recommend an iteration strategy:**

   | Strategy | When to recommend |
   |----------|------------------|
   | Fix in place | Implementation detail issues, edge cases (default) |
   | New change in same worktree | Need to re-plan, existing code is useful reference |
   | Git reset + stash reference | Need clean baseline but want to keep code as reference |
   | Git reset, full redo | Fundamental approach error |
   | Abandon change | Requirements need redefining |

   Present recommendation with reasoning.

   c. **Let the user choose** (they may pick a different strategy).

   d. **Execute the chosen strategy:**
   - Fix in place → return to myspec-apply skill
   - New change → `openspec new change "<new-name>"`, keep old code
   - Git reset + stash → `git stash && git reset --hard <pre-impl-commit>`
   - Git reset → `git reset --hard <pre-impl-commit>`
   - Abandon → prompt user to run cleanup manually

   e. After executing strategy, prompt: **"Run myspec-apply skill to re-implement."**

## Guardrails

- Do NOT skip the user acceptance step. The user MUST explicitly confirm.
- Do NOT proceed to merge or archive. Those are handled by myspec-merge.
- Do NOT run build or test. Those are the user's responsibility.
- When backfilling artifacts, update ALL artifacts, not just the ones that drifted.
- When recommending iteration strategies, always lead with the recommended one and explain why.
