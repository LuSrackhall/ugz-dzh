## 1. 修改输出路径逻辑

- [x] 1.1 修改 `cmd/generate.go`，创建月度目录 `monthDir := filepath.Join(yearDir, month)`
- [x] 1.2 修改 `cmd/generate.go`，将 xlsx 文件写入月度目录
- [x] 1.3 修改 `cmd/generate.go`，将 CSV/XLSX 分录汇总写入月度目录
- [x] 1.4 修改 `cmd/generate.go`，将余额表写入月度目录
- [x] 1.5 修改 `cmd/generate.go`，将 HTML 打印版输出到月度目录的 html 子目录

## 2. 修改上月 xlsx 查找逻辑

- [ ] 2.1 修改 `generator/workbook.go` 的 `prevMonthPath` 函数，从上月目录查找
- [ ] 2.2 修改 `generator/workbook.go` 的 `currentPath` 函数，返回月度目录中的路径

## 3. 修改 force 级联删除逻辑

- [ ] 3.1 修改 `cmd/generate.go` 的 force 级联删除逻辑，删除月度目录而非单个文件

## 4. 修改 HTML 输出路径

- [ ] 4.1 修改 `generator/html_print.go` 的 `GenerateHTMLPrint` 函数，输出到月度目录的 html 子目录

## 5. 验证与测试

- [ ] 5.1 运行 `go build ./...` 确认编译通过
- [ ] 5.2 运行 `go test ./...` 确认测试通过
- [ ] 5.3 端到端测试：生成 4 个月份，验证目录结构正确
- [ ] 5.4 更新 README.md，说明新的输出目录结构

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
