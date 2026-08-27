# auto-close-voucher Tasks

## 1. 结转凭证生成

- [x] 1.1 `ledger gen-close` 命令：损益科目余额→结转分录（借/贷 本年收益 按余额方向），写 output/{year}/closing/
- [x] 1.2 幂等：已结转科目跳过（closed 全路径匹配——修复：总账+明细组合，验收发现）
- [x] 1.3 编号自动递增避免覆盖；凭证格式标准（可被解析器识别）
- [x] 1.4 `本年收益` 加入权益类科目类别

## 2. generate 自动并入

- [x] 2.1 generate 扫描 output/{year}/closing/*.md 并入，提示"已并入 N 张自动结转凭证"
- [x] 2.2 平衡校验在并入后执行（含自动凭证）

## 3. year-close 提示

- [x] 3.1 损益未结转告警处补充"可用 ledger gen-close 生成结转凭证"

## 4. 验证

- [x] 4.1 实测：gen-close 生成（21 科目，借贷 5453447.93 平衡）；幂等（再次无待结转）；generate 并入 1 张；12 月末损益科目全部归零；year-close 无损益告警
- [x] 4.2 `go test ./...` 全绿

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
