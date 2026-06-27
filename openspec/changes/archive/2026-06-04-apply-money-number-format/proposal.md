## Why

`fix-xlsx-amount-numeric-type` 已将金额单元格从文本改为数字类型，但缺少数字格式控制。当前 `4000` 显示为 `4000`，不符合会计惯例要求的千分位和两位小数显示（`4,000.00`）。此变更在数字类型基础上补齐显示格式，使 xlsx 输出达到正式会计报表的呈现标准。

## What Changes

**金额列数字格式**
- From: 金额单元格无格式控制，`float64` 值按默认格式显示（无千分位、无强制两位小数）
- To: 所有金额列应用 `#,##0.00` 数字格式，始终显示千分位分隔符和两位小数
- Reason: 会计惯例要求金额必须显示到分（两位小数）并带千分位以提升可读性
- Impact: 非破坏性 — 单元格值仍为 `float64`，仅显示格式变化

## Capabilities

### New Capabilities
- `money-number-format`: 所有通过 `centsToYuan` 写入的金额单元格自动应用 `#,##0.00` 数字格式

### Modified Capabilities


## Impact

- **generator/workbook.go**: 新增 `moneyStyleID` 字段和样式创建逻辑，暴露 `setMoneyStyle` 辅助方法
- **generator/*.go** (6 文件): 金额写入点追加 `SetCellStyle` 调用
- **go test ./generator/...**: 现有测试应全部通过
