# audit-voucher-balance Tasks

## 1. voucher 包校验实现

- [x] 1.1 新增 `voucher/balance_check.go`：`ValidateVoucherBalance(entries) (warnings []string, err error)`——按（日期,凭证号）分组绝对值平衡校验；红字 warning；VoucherNum<=0 跳过并 warning
- [x] 1.2 新增 `voucher/balance_check_test.go`：平衡通过/不平衡拒绝（含差额断言）/红字绝对值口径/红字不修改/未解析凭证号跳过

## 2. generate 内建校验

- [x] 2.1 `cmd/generate.go`：CollectEntries 后调用 `ValidateVoucherBalance`；err → 拒绝生成；warnings → 打印

## 3. 验证

- [x] 3.1 `go test ./...` 全绿
- [x] 3.2 e2e 回归：`bash scripts/test-e2e.sh --skip-test` 通过（测试数据凭证逐张平衡，校验无误伤）；实测：不平衡凭证被拒（含凭证号+差额 200.00），平衡凭证通过校验

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
