## Why

所有 xlsx 金额单元格当前以字符串写入，导致 Excel 将其识别为文本格式。用户无法使用 SUM 求和、按数值筛选或创建图表。根因是 `centsToYuanStr()` 返回 `string` 类型，仅需将返回值改为 `float64` 即可解决，改动最小、效果全面。

## What Changes

**`centsToYuanStr` → `centsToYuan` — 分转元函数返回类型**
- From: `func centsToYuanStr(c int64) string` 返回 `"1234.56"`（文本）
- To: `func centsToYuan(c int64) float64` 返回 `1234.56`（数字）
- Reason: `excelize.SetCellValue` 对 string 写入文本单元格，对 float64 写入数字单元格
- Impact: 非破坏性 — 所有 33 处调用点无需修改，仅函数定义和测试断言需更新

## Capabilities

### New Capabilities
- `xlsx-numeric-amount`: 所有通过 `SetCellValue` 写入 xlsx 的金额单元格均为数字类型，支持 Excel 原生数值运算、筛选和图表

### Modified Capabilities
<!-- None — 现有行为无规范级别变更 -->

## Impact

- **generator/workbook.go**: 修改 `centsToYuanStr` 函数签名和实现
- **generator/generator_test.go**: 更新测试调用和断言类型（string → float64）
- **所有调用点**（`gl_sheet.go`, `ml_sheet.go`, `monthly_close.go`, `monthly_close_ml.go`, `initial_sheet.go`, `final_sheet.go`）: 仅函数名重命名
