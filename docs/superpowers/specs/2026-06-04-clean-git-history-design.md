# 清理 git 历史中的敏感财务数据

## 背景

仓库包含真实财务凭证数据（含真实人名、单位名、金额），在推送到远程前必须从 git 历史中彻底移除。

## 清理范围

| 路径 | 原因 |
|---|---|
| `example/2026_01/*.md` (3 文件) | 已跟踪，含真实人名与单位名 |
| `test/e2e/output/` (4 文件) | 已删除但仍在历史 commit 中 |

## 不动的部分

| 路径 | 理由 |
|---|---|
| `test/e2e/test_data/` | 已 gitignore，未跟踪 |
| `test/e2e/out/` | 已 gitignore，未跟踪 |
| `scripts/verify_ml_closings.go` | 不含敏感数据 |

## 执行步骤

### 1. 备份

将 `example/` 和 `test/e2e/test_data/` 复制到仓库外备份目录，确认完整后再继续。

### 2. 更新 `.gitignore`

添加 `example/` 规则。

### 3. git filter-repo 清理

使用 `git-filter-repo` 移除以下路径的所有历史记录：
- `example/`
- `test/e2e/output/`
- `docs/superpowers/specs/2026-06-04-clean-git-history-design.md`（本设计文档也需从历史中移除，因为旧版本含敏感信息）

### 4. 清理后重新提交设计文档

filter-repo 完成后，重新添加本设计文档（已脱敏版本）并提交。

### 5. 验证

- `git log --all -- example/` → 无输出
- `git log --all -- test/e2e/output/` → 无输出
- 本地文件仍在原位
- `go build ./...` 和 `go test ./...` 正常通过

## 风险

- 所有 commit hash 会改变（filter-repo 的副作用）
- 如果未来推送到已有远程，需要 force push
- 本地开发不受影响，文件仍在原位
