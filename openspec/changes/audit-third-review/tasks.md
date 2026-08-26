# audit-third-review Tasks

## 1. D1 合并父级冲突

- [x] 1.1 D1a：GenerateWorkbook 禁止合并总账科目直接记账（报错）
- [x] 1.2 D1b：ExtractLastMonthFinals 跳过合并视图 sheet
- [x] 1.3 D1c：WriteMergeGLClosings 合并期初只聚子科目
- [x] 1.4 实测：直接记账被拒（含提示）；只记子科目通过且月结 1 组（修复前 2 组）

## 2. F1 跨年结转校验

- [x] 2.1 year-close 三告警不阻断：非 12 月 / 期初不平 / 损益漏结转
- [x] 2.2 balance.AccountTypeOf 导出
- [x] 2.3 实测：e2e 数据 year-close 显示损益漏结转告警（管理费用-干部报酬 56700 等）

## 3. N1/N2 输入防线

- [x] 3.1 N1：ValidateVoucherBalance 同行借贷双非零 warning
- [x] 3.2 N2：parseAmountToCents 全角括号（500）；测试用例补充

## 4. 验证

- [x] 4.1 `go test ./...` 全绿
- [x] 4.2 e2e 全流程 + --keep-json 通过（year-close 告警不阻断不卡流程）

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
