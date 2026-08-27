# audit-design-review Tasks

## 1. 现金/银行日记账

- [x] 1.1 `WriteJournals`：现金日记账（库存现金）+ 银行存款日记账（含子科目），筛选当月分录按日期逐笔
- [x] 1.2 列：日期/凭证字号/摘要/对方科目（同凭证去重，超 3 加"等"）/借方/贷方/余额
- [x] 1.3 日结（本日合计）+ 月结（本月合计/期末结存）；现金余额为负告警
- [x] 1.4 实测：e2e 2025-10 现金日记账逐笔/日结/月结/对方科目正确；负余额 -195.33 正确显示

## 2. 期初/期末表借贷分列

- [x] 2.1 期初/期末表改"科目/方向/借方金额/贷方金额"（资产费用→借列，负债权益收入→贷列）
- [x] 2.2 合计借贷分列 + 差额提示
- [x] 2.3 实测：2025-12 期初表借方合计 360383.16 = 贷方合计 360383.16 试算平衡

## 3. year-close 新年度 JSON

- [x] 3.1 year-close 自动复制 {year}.json → {year+1}.json
- [x] 3.2 实测：e2e year-close 生成 output/2026/2026.json ✓

## 4. 损益结转草稿

- [x] 4.1 year-close 对损益科目输出结转建议分录（借 本年收益 / 贷 科目 等，按余额方向）
- [x] 4.2 实测：结转草稿输出（公益支出/管理费用等）✓

## 5. 验证

- [x] 5.1 `go test ./...` 全绿
- [x] 5.2 e2e 全流程 + --keep-json 通过

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
