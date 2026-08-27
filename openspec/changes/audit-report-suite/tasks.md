# audit-report-suite Tasks

## 1. 报表套件

- [x] 1.1 资产负债表（资产/费用→左，负债/权益/收入→右，|期末|，方向相反显负，合计+差额）
- [x] 1.2 收支结余表（收入累计-支出累计=本年收益）
- [x] 1.3 科目汇总表（本月借/贷发生额试算平衡）
- [x] 1.4 凭证序时簿（每凭证一行：日期/字号/摘要/借贷合计）
- [x] 1.5 实测：4 sheet 生成、资产负债表 -7750=-7750、科目汇总 223323.51=223323.51

## 2. 结账标记

- [x] 2.1 GlobalSettings.ClosingMonth；generate 校验（<=结账月无 -f 拒绝）
- [x] 2.2 `ledger lock -m` 命令设置/解锁
- [x] 2.3 实测：lock 后 generate 报错"已结账"

## 3. 未知科目未分类 + add-manual 提示

- [x] 3.1 inferPropertyByType 未知→"未分类"（测试更新）
- [x] 3.2 add-manual 提示期初锚定建账月、-m 仅记录

## 4. 验证

- [x] 4.1 `go test ./...` 全绿
- [x] 4.2 e2e 全流程通过

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
