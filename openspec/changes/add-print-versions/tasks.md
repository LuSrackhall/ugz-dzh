## 1. 基础建设

- [x] 1.1 新增 `generator/amount.go`，实现 `centsToDigits` 纯函数
- [x] 1.2 新增 `generator/amount_test.go`，编写单元测试（覆盖 0、正数、大额、负数）
- [x] 1.3 新增 `generator/styles.go`，实现 `TableStyles` 结构体和共享样式函数
- [x] 1.4 实现 `writeAmountCells` 函数，将金额写入 Excel 12 个单元格
- [x] 1.5 实现 `formatAmountForDisplay` 函数，用于调试输出

## 2. 打印版 Excel

- [x] 2.1 新增 `generator/print_gl_sheet.go`，实现打印版总分类账 Sheet 生成
- [x] 2.2 实现打印版总分类账的列布局（A-AN，共 38 列）
- [x] 2.3 实现打印版总分类账的金额分栏写入（借方/贷方/余额）
- [x] 2.4 新增 `generator/print_ml_sheet.go`，实现打印版多科目明细账 Sheet 生成
- [x] 2.5 实现打印版多科目明细账的左右页布局
- [x] 2.6 实现打印版 Sheet 的样式设置（表头背景、边框、字号）
- [x] 2.7 实现打印版 Sheet 的页面设置（横向、FitToWidth、重复表头）
- [x] 2.8 实现打印版 Sheet 的月结处理（本月合计、本年累计）

## 3. HTML 打印版

- [ ] 3.1 新增 `generator/templates/` 目录，创建 HTML 模板文件
- [ ] 3.2 设计 HTML 模板结构（左右页布局、表格、金额分栏）
- [ ] 3.3 编写 CSS 打印样式（@page、@media print、边框、字体）
- [ ] 3.4 新增 `generator/html_print.go`，实现 HTML 模板渲染函数
- [ ] 3.5 实现 `embed.FS` 嵌入 HTML 模板
- [ ] 3.6 实现 HTML 版的月结处理（本月合计、本年累计）
- [ ] 3.7 测试浏览器打印效果（Chrome、Firefox、Edge）

## 4. CLI 集成

- [ ] 4.1 修改 `cmd/generate.go`，添加 `-view-only`、`-print-only`、`-html-only` 参数
- [ ] 4.2 实现按需生成逻辑，根据参数调用不同的生成函数
- [ ] 4.3 实现默认行为：无参数时生成所有三套输出
- [ ] 4.4 更新帮助文档，说明新参数的用法

## 5. 集成测试

- [ ] 5.1 编写端到端测试，验证三套输出的数据一致性
- [ ] 5.2 测试 3 个月凭证的完整生成流程
- [ ] 5.3 验证打印版 Excel 的列宽和打印效果
- [ ] 5.4 验证 HTML 版的浏览器打印效果
- [ ] 5.5 更新 README.md，说明新的打印功能和用法

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
