# audit-e2e-resume Tasks

## 1. 脚本改造

- [x] 1.1 参数解析：`--keep-json`（与 --skip-test 并列）
- [x] 1.2 阶段 2 分支：keep-json 跳过 rm -rf + init，校验 JSON 存在（缺失报错）；否则原逻辑
- [x] 1.3 配置化完善：test/e2e/mappings.json（map add，幂等跳过）+ adjustments.json（add-manual，幂等跳过）
- [x] 1.4 用法注释更新

## 2. 实测验证

- [x] 2.1 全流程正常；--keep-json 续跑成功（跳过 rm+init）
- [x] 2.2 --keep-json 再次执行幂等
- [x] 2.3 无 JSON 报错"请先运行完整流程"
- [x] 2.4 配置化映射：首次执行"映射: 测试-映射验证科目 -> 库存现金"，再次"已存在，跳过"；配置化期初调整幂等跳过
- [x] 2.5 `go test ./...` 全绿（脚本改动不影响 CLI 测试）

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
