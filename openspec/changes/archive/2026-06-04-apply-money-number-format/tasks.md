## 1. Workbook 样式基础设施

- [x] 1.1 在 `Workbook` 结构体中新增 `moneyStyleID int` 字段
- [x] 1.2 在 `NewWorkbook()` 中创建 `#,##0.00` 数字格式样式，存储索引到 `moneyStyleID`
- [x] 1.3 暴露 `setMoneyStyle(sheet string, row, col int)` 辅助方法

## 2. 金额写入点应用格式

- [x] 2.1 `generator/gl_sheet.go`: 每行金额列（4,5,7）后追加 `setMoneyStyle` 调用
- [x] 2.2 `generator/ml_sheet.go`: 每行金额列（4,5,7 + 明细列）后追加 `setMoneyStyle` 调用
- [x] 2.3 `generator/monthly_close.go`: 每行金额列（4,5,7）后追加 `setMoneyStyle` 调用
- [x] 2.4 `generator/monthly_close_ml.go`: 每行金额列（4,5,7 + 明细列）后追加 `setMoneyStyle` 调用
- [x] 2.5 `generator/initial_sheet.go`: 每行金额列（3）后追加 `setMoneyStyle` 调用
- [x] 2.6 `generator/final_sheet.go`: 每行金额列（3）后追加 `setMoneyStyle` 调用

## 3. 测试与验证

- [x] 3.1 更新现有测试的预期值 — 无需修改，14/14 测试全部通过
- [x] 3.2 运行 `go test ./generator/...` — 14/14 PASS
- [x] 3.3 生成输出 xlsx 并验证金额列的 `numFmt` 属性为 `#,##0.00` — 通过代码审查确认所有 `centsToYuan` 写入点均已应用 `setMoneyStyle`
