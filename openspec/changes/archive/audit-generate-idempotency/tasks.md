# audit-generate-idempotency Tasks

## 1. 幂等判定

- [x] 1.1 新增 `generator.AlreadyGenerated(path) (bool, error)`（扫描"本月合计"行）
- [x] 1.2 `cmd/generate.go` else 分支：文件存在时调用 `AlreadyGenerated`，含本月合计 → 拒绝；不含 → 放行
- [x] 1.3 测试：AlreadyGenerated 单测（含/不含本月合计）

## 2. 无活动月份月结行

- [x] 2.1 `WriteMonthClosings`：extendedActivity/extendedChanged 副本扩展（期初≠0 且无分录且 GL Sheet 存在 → act={0,0} 补月结行）；不修改入参
- [x] 2.2 测试：无活动科目补"本月合计 0/0 + 期末=期初"；期初=0 不补

## 3. 集成验证

- [x] 3.1 `go test ./...` 全绿
- [x] 3.2 e2e 回归：`bash scripts/test-e2e.sh --skip-test` 通过
- [x] 3.3 实测：同月无 -f 重跑被拒（含本月合计行）；year-close 后无 -f 生成 2026-01 成功；2026-01 账本 23 个无活动科目补"本月合计 0/0"行

## 4. 关联修正（Change 2 口径）

- [x] 4.1 借贷平衡校验口径：绝对值 → **带符号净额**（e2e 数据"记字第0001号"红字调账凭证 贷-11700/贷+11700 同侧冲抵，绝对值误报、净额正确）；同步更新 spec/design/测试

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
