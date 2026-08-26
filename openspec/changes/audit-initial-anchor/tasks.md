# audit-initial-anchor Tasks

## 1. 期初锚定建账月

- [x] 1.1 `GetInitBalanceForGenerate`：调整额生效条件改 `month == StartMonth`（建账月）；其余月份续链
- [x] 1.2 `HasInitialAdjustment`：仅建账月 true
- [x] 1.3 删除 `ensureBackfillForAll` 及其两处调用（AddManualAdjustment / UpdateBalancesAfterGenerate）
- [x] 1.4 `AddManualAdjustment`：幂等更新（同科目更新调整额/说明，不报"已存在"）

## 2. 测试与文档

- [x] 2.1 balance_test：生效月→建账月用例；删除 TestBackfill / TestAutoAccountNoBackfill；TestAddManualAdjustment 重复用例改更新断言
- [x] 2.2 README 期初调整章节：锚定建账月、生效月字段信息性、中途修正 -f 重建、补记业务走凭证
- [x] 2.3 audit-initial-balance spec 顶部加修正 Note

## 3. 验证

- [x] 3.1 `go test ./...` 全绿
- [x] 3.2 e2e 全流程通过
- [x] 3.3 实测：期初调整落建账月（2025-10 期初=100000 + 账页"期初余额"行）；非建账月续链（2025-11=10 月末）；中途修正（add-manual 100000→150000 幂等更新，-f 重建后 2025-10/11 期初=150000）

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
