# audit-second-review Tasks

## 1. H1 期初回退

- [x] 1.1 `GetInitBalanceForGenerate` 去掉 `mb.Final != 0` 条件（取最近月期末，含 0）
- [x] 1.2 测试：TestGetInitBalanceZeroBalanceRecent（结平跨年=0 / 非结平取最近月）

## 2. H2 红字全链路

- [x] 2.1 `parseAmountToCents`：支持 ASCII `-`、括号 `(500)`、全角 `－`、Unicode `−` 四种格式统一转负数；括号内已有负号不重复取反
- [x] 2.2 测试：TestParseAmountRedInkFormats（8 格式 + 空串）
- [x] 2.3 打印版：`amountSubStyle` 加 red 参数（红字 #CC0000），缓存 key 扩为 [4]int 防串用；金额格负数标记
- [x] 2.4 实测：括号 `(11,700.00)` 凭证 → 查看版贷方 -11700、打印版拆位数字红色字体

## 3. H3 合并累计

- [x] 3.1 `WriteMergeGLClosings` 本年/本季累计遍历 Tree 全叶子（isChildOf），activity 零值安全
- [x] 3.2 代码确认：原实现只遍历 activity，无活动子科目历史累计被漏

## 4. 验证

- [x] 4.1 `go test ./...` 全绿
- [x] 4.2 e2e 全流程 + --keep-json 通过
- [x] 4.3 打印版正常账本回归（红字样式不影响蓝字/其他样式）

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
