# audit-production Tasks

## 1. 投产门槛

- [x] 1.1 失败原子性：generate 先 wb.Save() 再 SaveConfig（调换顺序）
- [x] 1.2 凭证号未解析阻断：ValidateVoucherBalance unparsed → error（测试更新）
- [x] 1.3 跳月告警：GenerateWorkbook 非首月 + 上月 xlsx 缺失 → 告警
- [x] 1.4 README 备份与恢复章节
- [x] 1.5 year-close 期初锚定提示

## 2. 验证

- [x] 2.1 `go test ./...` 全绿
- [x] 2.2 e2e 全流程 + --keep-json 通过
- [x] 2.3 实测：跳月告警生效（无 01 生成 02 → 告警不阻断）

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
