## 1. 修改 HTML 输出路径

- [x] 1.1 修改 `generator/html_print.go` 的 `generateAccountHTML` 函数，将输出路径改为 `filepath.Join(outputDir, "html")`
- [x] 1.2 在 `generateAccountHTML` 函数中添加 `os.MkdirAll(htmlDir, 0o755)` 自动创建 `html/` 子目录
- [x] 1.3 更新 `cmd/generate.go` 中的 verbose 日志，显示新的 HTML 输出路径

## 2. 验证与测试

- [ ] 2.1 运行 `go build ./...` 确认编译通过
- [ ] 2.2 运行 `go test ./...` 确认测试通过
- [ ] 2.3 手动运行 `ledger generate` 验证 HTML 文件输出到 `html/` 子目录

---

## Post-Implementation Workflow

After completing ALL tasks above, follow this sequence strictly:

1. **Verify**: Run `/opsx:verify` to produce verify.md
2. **User Acceptance**: Present change summary, ask user to confirm the problem is solved
3. **Merge**: After user accepts, go to main branch and merge (must ask user)
4. **Archive**: Run `/opsx:archive` on main
5. **Cleanup**: `git worktree remove .worktrees/change/<name>`

**Iteration**: If user does not accept, analyze the issue and recommend:
fix in place / new change / git reset + stash / git reset / abandon.
