# rebuild-and-guards Tasks

## 1. 全量重建脚本

- [x] 1.1 scripts/rebuild.sh：读 JSON 启动月 → 逐月 generate -f（凭证目录 {YEAR}_{MM}/{YEAR}/{MM} 两种形态）
- [x] 1.2 修复：变量边界（$START）全角括号并入变量名 bug → ${START}
- [x] 1.3 实测：重建 2025 全年（10/11/12 月）✓

## 2. check 漂移比对

- [x] 2.1 check 读最新月 xlsx 期末表 vs JSON Balances，不一致告警（-f 可重建）
- [x] 2.2 实测：篡改 xlsx 后 check 报"⚠ 漂移: 银行存款"✓
- [x] 2.3 删除误导的 SetAccountProperty 提示（N6：无此命令）；未知类别提示改"未分类"

## 3. 凭证号重号检测

- [x] 3.1 generate 文件名"记字第X号"数字重复 → 告警
- [x] 3.2 实测：记字第0001号.md + 记字第0001号 更正.md → "凭证号重复 1"✓

## 4. README 文档

- [x] 4.1 红字表达（-/(500)/全角−）+ 结转流程
- [x] 4.2 打印说明（日记账/报表直接打印查看版，GL/ML 用位格版）

## 5. 验证

- [x] 5.1 `go test ./...` 全绿

---

## Post-Implementation Workflow

<!-- DO NOT MODIFY THIS SECTION — it defines the required workflow after all tasks are complete -->

After completing ALL tasks above, follow this sequence strictly:

1. **Verify**: Run `/opsx:verify` to produce verify.md
2. **User Acceptance**: Present change summary, ask user to confirm the problem is solved
3. **Merge**: After user accepts, go to main branch and merge (must ask user)
4. **Archive**: Run `/opsx:archive` on main
5. **Cleanup**: `git worktree remove .worktrees/change/<name>`

**Iteration**: If user does not accept, analyze the issue and recommend:
fix in place / new change / git reset + stash / git reset / abandon.
