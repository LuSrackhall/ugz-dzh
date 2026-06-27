---
name: myspec-gwt
description: "创建 git worktree 以隔离实施。Use when you need an isolated workspace for a change. Can be called directly or by myspec-br."
---

# Git Worktree 创建

创建一个隔离的 git worktree，供变更实施使用。

先检查是否已在隔离环境中，然后创建工作树。

<HARD-GATE>
必须在默认分支（main 或 master）上才能创建工作树。如果已在 worktree 或非默认分支，停止并报告。
</HARD-GATE>

**Windows 环境要求：** 本技能中的 shell 命令基于 bash 语法。Windows 用户需要使用 Git Bash、WSL 或 MSYS2。如果检测到 Windows PowerShell，提示用户切换到 bash 兼容环境。

## Checklist

你必须按顺序完成以下步骤：

1. **检测现有隔离** — 检查是否已在 worktree 或 submodule 中
2. **验证默认分支** — 确保当前在 main/master 且工作区干净
3. **确认分支名** — 使用传入的名称，或询问用户
4. **检查已有 worktree** — 避免重复创建，检测损坏的 worktree
5. **验证 .gitignore** — 确保 `.worktrees/` 被忽略
6. **创建 worktree** — 使用 `git worktree add`
7. **报告状态** — 告知用户 worktree 路径和分支名

## 流程

### Step 0: 检测现有隔离

在创建任何东西之前，检查是否已在隔离环境中。

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
```

**Submodule 检查：** `GIT_DIR != GIT_COMMON` 在 submodule 中也成立。确认不是 submodule：

```bash
git rev-parse --show-superproject-working-tree 2>/dev/null
```

**如果 `GIT_DIR != GIT_COMMON`（且不是 submodule）：** 已在 worktree 中。报告当前路径和分支，停止。

**如果 `GIT_DIR == GIT_COMMON`（或在 submodule 中）：** 在正常仓库中。继续 Step 1。

### Step 1: 验证默认分支

```bash
BRANCH=$(git branch --show-current)
```

**检测默认分支名：** 不硬编码 `main`。优先检查 `main`，如果不存在则检查 `master`：

```bash
DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@')
if [ -z "$DEFAULT_BRANCH" ]; then
  if git show-ref --verify --quiet refs/heads/main; then
    DEFAULT_BRANCH="main"
  elif git show-ref --verify --quiet refs/heads/master; then
    DEFAULT_BRANCH="master"
  else
    DEFAULT_BRANCH=""
  fi
fi
```

**如果不在默认分支：**

> "当前在分支 `<branch>`，不在 `<default-branch>`。请先切换：`git checkout <default-branch>`"

**如果 `DEFAULT_BRANCH` 为空（仓库无提交）：**

> "仓库尚无提交。请先至少创建一个提交，然后才能创建工作树。"

停止。

**如果工作区有未提交的更改（包括合并冲突）：**

检查是否在合并状态：
```bash
git merge HEAD 2>/dev/null
test -f .git/MERGE_HEAD
```

**如果在合并中：**
> "当前有未完成的合并。请先完成合并（`git merge --continue`）或中止（`git merge --abort`）。"

**否则：**
> "工作区有未提交的更改。请先提交或暂存。"

停止。

### Step 2: 确认分支名

分支名格式：`change/<kebab-case-name>`

**如果调用方传入了名称：** 验证格式（只允许 `[a-z0-9-]`），直接使用。

**如果未传入名称：** 询问用户：

> "请为这次变更命名（kebab-case，如 `add-user-auth`）："

**验证输入：** 分支名必须匹配 `[a-z0-9-]+`（小写字母、数字、连字符）。如果不合法，提示用户修正。

将用户输入转为 `change/<name>` 格式。

### Step 3: 检查已有 worktree

**先清理过时引用：**

```bash
git worktree prune
```

**检查同名 worktree：**

```bash
git worktree list --porcelain | grep -A1 "branch refs/heads/change/<name>$"
```

注意：`$` 锚定行尾，防止子串误匹配（如 `change/add` 匹配到 `change/add-user`）。

**如果分支存在且有 worktree：** 验证 worktree 目录是否实际存在：

```bash
WORKTREE_PATH=$(git worktree list --porcelain | grep -B1 "branch refs/heads/change/<name>$" | grep "worktree" | cut -d' ' -f2)
if [ -d "$WORKTREE_PATH" ]; then
  echo "已有工作树：$WORKTREE_PATH"
  # 停止
else
  echo "worktree 目录已损坏，将重新创建"
  git worktree remove --force "$WORKTREE_PATH" 2>/dev/null
  # 继续 Step 4
fi
```

**如果分支存在但没有 worktree（orphaned branch）：**

> "分支 `change/<name>` 已存在但无工作树。将为该分支创建新工作树。"

确保 `.worktrees/` 目录存在，然后为现有分支创建工作树（不带 `-b`）：

```bash
mkdir -p .worktrees
git worktree add ".worktrees/change/<name>" "change/<name>"
```

**如果分支不存在：** 继续 Step 4。

### Step 4: 验证 .gitignore

```bash
git check-ignore -q .worktrees 2>/dev/null
```

**如果未被忽略：** 添加到 .gitignore。

```bash
echo ".worktrees/" >> .gitignore
git add .gitignore
```

**注意：** 先 `git add` 但不 `git commit`。将 `.gitignore` 的提交延迟到 worktree 创建成功后，避免 worktree 创建失败时留下孤立的提交。

### Step 5: 创建 worktree

```bash
mkdir -p .worktrees
git worktree add ".worktrees/change/<name>" -b "change/<name>"
```

**如果成功：** 提交之前暂存的 `.gitignore`（如果有）：

```bash
git commit -m "chore: add .worktrees/ to gitignore" 2>/dev/null || true
```

**如果失败（权限错误、磁盘空间不足等）：**

> "Worktree 创建失败：`<error message>`"
> "可能原因：磁盘空间不足、权限被拒绝。"
> "回退：将在当前目录继续工作。"

如果 `.gitignore` 已暂存但未提交，丢弃它：
```bash
git reset HEAD .gitignore 2>/dev/null
git checkout -- .gitignore 2>/dev/null
```

降级到原地工作，继续后续流程。

### Step 6: 报告状态

```
工作树已就绪

路径：.worktrees/change/<name>
分支：change/<name>
```

## 快速参考

| 情况 | 操作 |
|------|------|
| 已在 worktree 中 | 报告当前路径，停止 |
| 不在默认分支 | 要求切换到 main/master |
| 仓库无提交 | 报错停止 |
| 工作区不干净（含合并冲突） | 要求解决后重试 |
| 分支已存在且有 worktree | 报告已有 worktree 路径 |
| worktree 目录损坏 | prune + remove + 重新创建 |
| 分支已存在但无 worktree | 为现有分支创建新 worktree |
| .worktrees/ 未被忽略 | 添加到 .gitignore（延迟提交） |
| worktree 创建失败 | 降级到原地工作 |
| 分支名不合法 | 要求用户修正为 `[a-z0-9-]+` |

## Red Flags

**绝对不要：**
- 跳过 Step 0 的隔离检测
- 在非默认分支上创建工作树
- 跳过 `.gitignore` 验证
- 在 worktree 创建失败后继续假装成功
- 硬编码 `main`（必须兼容 `master`）

**绝对要：**
- 先 `git worktree prune` 清理过时引用
- 验证 worktree 目录实际存在（不只是 git 注册信息）
- 创建失败时降级到原地工作
- 验证分支名格式
- 将 `.gitignore` 提交延迟到 worktree 创建成功后
