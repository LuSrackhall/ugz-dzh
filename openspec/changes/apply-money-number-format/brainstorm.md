## Design Summary

为所有金额列应用 excelize 数字格式 `#,##0.00`，使金额单元格始终按会计惯例显示两位小数和千分位分隔符（如 `4,000.00` 而非 `4000`），同时保持单元格为数字类型（float64）。

## Alternatives Considered

### 方案 A：回退到文本方案
- **做法**：将 `centsToYuan` 改回返回格式化字符串（如 `"4000.00"`），单元格变为文本类型
- **优点**：显示精确控制，无浮点误差
- **缺点**：单元格变成文本类型，excelize 写入 `t="inlineStr"`，丧失数字单元格的所有优势（排序、汇总、公式计算）
- **为何未采用**：与我们刚完成的 `fix-xlsx-amount-numeric-type` 修复方向相反

### 方案 B：excelize 自定义数字格式 `#,##0.00`（采用）
- **做法**：保持 `centsToYuan` 返回 `float64`，在 Workbook 初始化时创建 `CustomNumFmt: "#,##0.00"` 样式，写入金额时额外调用 `SetCellStyle` 应用该样式
- **优点**：单元格保持数字类型；显示效果符合会计惯例（千分位 + 两位小数）；纯 Go 方案，无外部依赖；实现量约 50 行，集中在 workbook 初始化
- **缺点**：需要在每个金额写入点额外调用 `SetCellStyle`
- **为何采用**：实现成本最低，与我们已完成的修复方向一致，且千分位是会计标配

## Agreed Approach

方案 B — 在 `generator/workbook.go` 中新增 `moneyStyle`（`CustomNumFmt: "#,##0.00"`），所有金额写入点追加 `SetCellStyle` 应用该样式。

## Key Decisions

- 格式字符串选择 `#,##0.00` 而非 `0.00`，提供千分位分隔符以提升大额金额可读性
- 不使用货币符号（`¥`），保持报表的通用性
- 不在 Go 层面格式化字符串，完全委托给 excelize/Excel 的数字格式引擎处理显示

## Open Questions

无。