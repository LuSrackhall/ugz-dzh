---
name: myspec-br
description: "You MUST use this before any creative work - creating features, building components, adding functionality, or modifying behavior. Explores user intent, requirements and design before implementation. Produces a design document as output."
---

# Brainstorming Ideas Into Designs

Turn ideas into fully formed designs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what the user is building, present the design and get approval. The terminal product is a design document.

<HARD-GATE>
Do NOT invoke any implementation skill, write any code, scaffold any project, or take any implementation action until you have presented a design and the user has approved it. This applies to EVERY project regardless of perceived simplicity.
</HARD-GATE>

## Anti-Pattern: "This Is Too Simple To Need A Design"

Every project goes through this process. A todo list, a single-function utility, a config change — all of them. "Simple" projects are where unexamined assumptions cause the most wasted work. The design can be short (a few sentences for truly simple projects), but you MUST present it and get approval.

## Checklist

You MUST create a task for each of these items and complete them in order:

1. **Explore project context** — check files, docs, recent commits
2. **Ask clarifying questions** — one at a time, understand purpose/constraints/success criteria
3. **Propose 2-3 approaches** — with trade-offs and your recommendation
4. **Present design** — in sections scaled to their complexity, get user approval after each section
5. **User approves design** — confirmed through section-by-section review
6. **Design self-review** — quick inline check for placeholders, contradictions, ambiguity, scope
7. **User reviews design** — ask user to review the final design before proceeding
8. **Write and commit design doc** — see output path logic below
9. **Inform user of next steps** — tell user how to proceed with implementation

## Process Flow

```dot
digraph brainstorming {
    "Explore project context" [shape=box];
    "Ask clarifying questions" [shape=box];
    "Propose 2-3 approaches" [shape=box];
    "Present design sections" [shape=box];
    "User approves design?" [shape=diamond];
    "Design self-review" [shape=box];
    "User reviews design?" [shape=diamond];
    "Write and commit design doc" [shape=box];
    "Inform user of next steps" [shape=doublecircle];

    "Explore project context" -> "Ask clarifying questions";
    "Ask clarifying questions" -> "Propose 2-3 approaches";
    "Propose 2-3 approaches" -> "Present design sections";
    "Present design sections" -> "User approves design?";
    "User approves design?" -> "Present design sections" [label="no, revise"];
    "User approves design?" -> "Design self-review" [label="yes"];
    "Design self-review" -> "User reviews design?";
    "User reviews design?" -> "Design self-review" [label="changes requested"];
    "User reviews design?" -> "Write and commit design doc" [label="approved"];
    "Write and commit design doc" -> "Inform user of next steps";
}
```

**The terminal state is a committed design document plus a clear instruction for what to do next.**

## The Process

### 1. Explore Project Context

Before asking any questions, understand the current state:

- Check files, docs, recent commits to understand the codebase
- Assess scope: if the request describes multiple independent subsystems, flag this immediately. Help the user decompose into sub-projects, each getting its own design cycle.

### 2. Ask Clarifying Questions

- Ask questions one at a time to refine the idea
- Prefer multiple choice questions when possible, but open-ended is fine too
- Only one question per message — if a topic needs more exploration, break it into multiple questions
- Focus on understanding: purpose, constraints, success criteria

### 3. Propose Approaches

- Propose 2-3 different approaches with trade-offs
- Present options conversationally with your recommendation and reasoning
- Lead with your recommended option and explain why

### 4. Present the Design

- Once you believe you understand what the user is building, present the design
- Scale each section to its complexity: a few sentences if straightforward, up to 200-300 words if nuanced
- Ask after each section whether it looks right so far
- Cover: context, goals/non-goals, key decisions, risks/trade-offs
- Be ready to go back and clarify if something doesn't make sense

**Design for isolation and clarity:**

- Break the system into smaller units that each have one clear purpose, communicate through well-defined interfaces, and can be understood and tested independently
- For each unit, you should be able to answer: what does it do, how do you use it, and what does it depend on?
- Can someone understand what a unit does without reading its internals? Can you change the internals without breaking consumers? If not, the boundaries need work.

**Working in existing codebases:**

- Explore the current structure before proposing changes. Follow existing patterns.
- Where existing code has problems that affect the work, include targeted improvements as part of the design.
- Don't propose unrelated refactoring. Stay focused on what serves the current goal.

### 5. User Approves Design

Design is approved through section-by-section review. Each section gets explicit user confirmation before moving to the next.

### 6. Design Self-Review

After the user approves the design, look at it with fresh eyes:

1. **Placeholder scan:** Any "TBD", "TODO", incomplete sections, or vague requirements? Fix them.
2. **Internal consistency:** Do any sections contradict each other? Does the architecture match the feature descriptions?
3. **Scope check:** Is this focused enough for a single implementation, or does it need decomposition?
4. **Ambiguity check:** Could any requirement be interpreted two different ways? If so, pick one and make it explicit.

Fix any issues inline. No need to re-review — just fix and move on.

### 7. User Reviews Design

After the self-review passes, ask the user to review the final design:

> "Design is ready. Please review the full design above and let me know if you want to make any changes before I write it to a file."

Wait for the user's response. If they request changes, make them and re-run the self-review. Only proceed once the user approves.

### 8. Write and Commit Design Document

After the user approves, write the design to a file and commit.

The document MUST follow this structure:

```markdown
## Context

<!-- Background and current state. What exists today? What are the constraints? -->

## Goals / Non-Goals

**Goals:**
<!-- What this design aims to achieve -->

**Non-Goals:**
<!-- What is explicitly out of scope -->

## Decisions

<!-- Key design decisions and rationale -->
<!-- For each decision: what was chosen, why, and what alternatives were considered -->

## Risks / Trade-offs

<!-- Known risks and trade-offs -->
<!-- Format: [Risk] → Mitigation -->
```

**Output path and worktree decision:**

Ask the user: **"是否创建工作树隔离这次变更？"**

- **如果用户选择创建工作树：**
  1. 从设计内容中派生一个 kebab-case 变更名（如 `add-user-auth`）
  2. 调用 `myspec-gwt` 技能创建工作树（传入变更名）
  3. 工作树创建成功后，**cd 到工作树目录**
  4. 在工作树中运行 `openspec new change "<name>"`
  5. 将设计文档写入 `openspec/changes/<name>/brainstorm-spec.md`
  6. 提交文件

- **如果用户选择不创建工作树：**
  1. 运行 `openspec new change "<name>"`
  2. 将设计文档写入 `openspec/changes/<name>/brainstorm-spec.md`
  3. 提交文件
  4. 警告用户："变更目录在 main 上，废弃时需要手动清理。"

### 9. Inform User of Next Steps

设计文档已提交。根据项目配置告知用户下一步：

**如果使用 myspec-driven schema：**
> "设计文档已写入 `<path>`。运行 **myspec-propose** 技能生成实施 artifact。如果提示 'change 已存在'，选择继续已有 change。"
>
> "propose 完成后，依次运行：myspec-apply → myspec-verify → myspec-merge。"

**如果使用其他 schema 或无 schema：**
> "设计文档已写入 `<path>`。请根据项目工作流继续。"

**不自动调用 propose。** 用户自行决定何时开始。

**不输出后续流程指引。** 实施后的收尾流程（verify → 验收 → merge → archive → cleanup）由 schema 的 `apply.instruction` 在 apply 阶段自动提供，不需要在此提前告知。

## Red Flags: Excuses to Skip This Process

| Rationalization | Why It's Wrong |
|---|---|
| "This is just a simple question" | Even simple questions can lead to implementation. If it might lead to code, go through the process. |
| "Let me explore the codebase first" | Exploring is step 1 of the process. Start the checklist. |
| "The user already knows what they want" | Knowing what you want ≠ having a validated design. Assumptions hide in the gaps. |
| "I can design and implement at the same time" | No. Design decisions made during implementation are invisible and unreviewable. |
| "This is too small for a design" | Small changes cause big bugs when assumptions are wrong. The design can be short, but it must exist. |
| "I'll just make a quick fix first" | Fixes without design create technical debt. Design first, fix within the design. |

## Key Principles

- **One question at a time** — Don't overwhelm with multiple questions
- **Multiple choice preferred** — Easier to answer than open-ended when possible
- **YAGNI ruthlessly** — Remove unnecessary features from all designs
- **Explore alternatives** — Always propose 2-3 approaches before settling
- **Incremental validation** — Present design, get approval before moving on
- **Be flexible** — Go back and clarify when something doesn't make sense
- **Ground in reality** — Explore the actual codebase, don't just theorize
