# audit-initial-balance Tasks

## 1. balance 包改造

- [x] 1.1 `GetInitBalanceForGenerate` 新增调整额来源（D1 优先级：手动生效月==当前月、自动首次月==当前月，调整额≠0 直取）
- [x] 1.2 `UpdateBalancesAfterGenerate` 新科目 `FirstRecord.Amount` 置 0（不再写首月净额）
- [x] 1.3 新增 `inferPropertyByType(general)` 按类别推断属性；`UpdateBalancesAfterGenerate` / `AddManualAdjustment` 改用类别推断
- [x] 1.4 `ensureBackfillForAll` 仅对 手动调整科目（调整额≠0）回填 `启动月..生效月-1`；自动识别科目不再回填
- [x] 1.5 新增 `PurgePhantomInitials(cfg)`（幂等：自动科目 FirstRecord.Amount 置 0 + 删除 m<首次月 且 无发生额的 Balances）
- [x] 1.6 新增期初试算平衡校验函数（按月份快照汇总借/贷，返回差额）

## 2. generator 改造

- [x] 2.1 构建期初映射时同步 `initialSource`（wb.InitialAdjust：调整额生效科目）
- [x] 2.2 `insertCarryForward` / `insertCarryForwardAtRow` 增加摘要参数；触发条件扩展（1 月 或 当月调整额生效）
- [x] 2.3 `appendCarryForwardOnly` 摘要按来源（1 月延续→上年结转，否则→期初余额）
- [x] 2.4 `generate.go`：加载 JSON 后调用 `PurgePhantomInitials`；当月期初映射试算平衡校验（不平告警）
- [x] 2.5 **-f 重建基线修复（D8）**：cmd/generate.go -f 删除 `>= month`（当月及以后）；NewWorkbook 复制上月校验同年度（跨年新建空工作薄）
- [x] 2.6 ML 明细账新 Sheet 期初行摘要改为"期初余额"；合并 GL 期初行摘要按来源

## 3. cmd/check.go 增强

- [x] 3.1 期初试算平衡校验（最新月份快照，不平报错）
- [x] 3.2 未知类别科目提示（可 SetAccountProperty 修正）

## 4. 测试

- [x] 4.1 `balance_test.go`：同步现有断言；新增调整额优先级/生效月语义/跨年不覆盖/回填仅手动/幻影迁移/平衡校验/属性类别测试
- [x] 4.2 generator 相关测试
- [x] 4.3 `go test ./...` 全绿
- [x] 4.4 e2e 回归：`bash scripts/test-e2e.sh --skip-test` 生成通过；实测验证：幻影期初清理（银行存款 Balances 仅 10/11/12）、add-manual 生效（银行存款-工商银行期初 100000）、-f 重建期初不再污染、期初行摘要（建账=期初余额、2026-01 跨年=上年结转）、check 期初平衡校验（正确检出测试数据 -15760.35 不平衡）

## 5. 文档

- [x] 5.1 README 期初调整章节修正（调整额真实生效、生效月语义、-f 级联重建工作流）

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
