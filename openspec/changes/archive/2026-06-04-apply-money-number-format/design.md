## Context

`fix-xlsx-amount-numeric-type` 已将 `centsToYuan` 从返回字符串改为返回 `float64`，使金额单元格变为数字类型。但缺少数字格式控制，导致 `4000` 显示为 `4000` 而非会计惯例的 `4,000.00`。

## Goals / Non-Goals

**Goals:**
- 所有金额列应用 `#,##0.00` 数字格式，确保始终显示千分位和两位小数
- 保持单元格为数字类型（float64），不退回文本方案
- 变更集中在一个文件（workbook.go），其他文件仅追加 `SetCellStyle` 调用

**Non-Goals:**
- 不修改 `centsToYuan` 函数签名或实现
- 不改变任何计算逻辑
- 不添加货币符号或自定义格式字符串

## Decisions

### 数字格式字符串选择 `#,##0.00`

- **理由**：千分位分隔符是会计标配，提升大额金额可读性。`0.00` 同样有效但缺乏千分位。
- **替代方案**：`0.00`（无千分位）、`¥#,##0.00`（带货币符号，过度约束）

### 样式在 Workbook 初始化时创建一次

- **理由**：`NewWorkbook()` 中调用 `excelize.NewStyle` 创建一次，所有金额写入点复用 `moneyStyleID`。避免每个单元格重复创建样式。
- **实现**：`Workbook` 结构体新增 `moneyStyleID int` 字段

### 通过辅助方法应用样式

- **理由**：暴露 `setMoneyStyle(sheet, row, col)` 方法封装 `SetCellStyle` + `moneyStyleID`，减少调用点重复代码

## Risks / Trade-offs

- **[Risk] 遗漏金额写入点** → 通过 grep 审计所有 `centsToYuan` 调用点，确保每个 `SetCellValue` 后紧跟 `SetCellStyle`
- **[Risk] 数字格式对零值的显示** → `#,##0.00` 格式下 `0` 显示为 `0.00`，符合会计预期
