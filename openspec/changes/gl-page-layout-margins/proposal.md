## Why

GL 总分类账的 Excel 页面无法在 A4 横向纸上正确打印：列宽换算系数错误导致内容被切割到页外，且页面缺少完整四周边距和外边缘边框。这些问题使生成结果无法直接交付打印，需要每次手动调整。

## What Changes

1. **添加上下边距空行** — 每页新增上边距 3 空行 + 下边距 3 空行，完善 A4 四边模拟
2. **修正 A4 列宽换算系数** — `pxPerColUnit: 3.5 → 7.0`，解决打印宽度偏差
3. **设置 Excel 页面布局** — 添加 `SetPageLayout`（A4 横向）和 `SetPageMargins(0)`，页边距由模拟行列控制
4. **新增 `setNoBorder` 辅助函数** — 主动清除对面侧的残留边框
5. **外侧边缘红色双线贯穿整页** — 正面页最左侧边框红色双线，背面页最右侧边框红色双线

## Capabilities

### New Capabilities

- `page-margins`: 统一的页面四周边距结构（上/下各 3 空行，左/右各 2 空列）
- `a4-print-layout`: A4 横向可打印布局，包含正确的列宽换算和页面设置

### Modified Capabilities

<!-- No existing spec requirements change — the gl-column-layout spec's data mapping and offset rules are unaffected -->

## Impact

- `generator/layout/gl_layout.go` — `GLMMToExcelColWidth` 的 `pxPerColUnit` 常数
- `generator/workbook.go` — 新增 `SetPageLayout` / `SetPageMargins` 调用
- `generator/gl_sheet.go` — `DataStartRow` 等行偏移量调整，`finalizeAllGLSheets` 中新增外边缘边框逻辑，新增 `setNoBorder` 函数
- `generator/monthly_close.go` — 所有依赖 `DataStartRow` 的行偏移量同步
- 行高/列宽具体值不变，仅换算系数修正
