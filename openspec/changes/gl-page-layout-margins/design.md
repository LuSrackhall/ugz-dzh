## Implementation Architecture

### 1. 边距结构

**新增常量**（`gl_sheet.go` 或 `workbook.go`）：
```go
const TopMarginRows = 3
const BottomMarginRows = 3
```

**行偏移映射表：**

| 角色 | 当前值 | 新值 |
|---|---|---|
| `DataStartRow` | 5 | `5 + TopMarginRows = 8` |
| `HeaderRow`（0-indexed） | 3 | `3 + TopMarginRows = 6` |
| `SubHeaderRow`（0-indexed） | 4 | `4 + TopMarginRows = 7` |
| 首页标题行 | Row 1 | Row 1 | ← 保持（属于第 1 层编排） |
| 上边距行 | 无 | Row 2-4 | ← 新增 3 行 |
| 首页列标题 | Row 4-5 | Row 7-8 |
| 数据起始行 | Row 6 | Row 9 |
| 过次页行 | Row 26 | Row 29 |
| 下边距行 | 无 | Row 30-32 |

后续页同步偏移 +3。

**涉及文件：**
- `generator/gl_sheet.go` — `writeGLTitle`、`writePageHeader`、`appendToGLSheet`、`pageStartRow`、`finalizeGLSheet` 等所有使用行偏移的函数
- `generator/monthly_close.go` — `WriteMonthClosings`、`nextDataRowAfterBreak`
- `generator/merge_gl_sheet.go` — 同步调整

### 2. A4 横向打印尺寸

**`gl_layout.go` — 列宽换算：**
```go
func GLMMToExcelColWidth(mm float64) float64 {
    const pxPerMM = 96.0 / 25.4
    const pxPerColUnit = 7.0  // 原 3.5 → ECMA-376 标准值
    return mm * pxPerMM / pxPerColUnit
}
```

`MLMMToExcelColWidth` 已使用 `7.0`，不需要改。

**`workbook.go` — 页面设置：**
在 `NewWorkbook` 中创建/复制 xlsx 后，对每个新 sheet 添加：
```go
f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
    Orientation: stringPtr("landscape"),
    PaperSize:   stringPtr(9),
})
f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
    Top:    float64Ptr(0),
    Bottom: float64Ptr(0),
    Left:   float64Ptr(0),
    Right:  float64Ptr(0),
})
```

### 3. 外侧边缘红色双线

**新增 `setNoBorder` 辅助函数**（`gl_sheet.go`）：
```go
func (wb *Workbook) setNoBorder(sheet string, col, row int) {
    cell := cellName(col, row)
    styleID, _ := wb.File.GetCellStyle(sheet, cell)
    if styleID == 0 { return }
    style, _ := wb.File.GetStyle(styleID)
    // 过滤掉指定侧的边框
    filtered := make([]excelize.Border, 0)
    for _, b := range style.Border {
        if b.Type != "left" && b.Type != "right" {
            filtered = append(filtered, b)
        }
    }
    style.Border = filtered
    newStyleID, _ := wb.File.NewStyle(style)
    wb.File.SetCellStyle(sheet, cell, cell, newStyleID)
}
```

**`finalizeAllGLSheets` 中逐页逻辑：**
```
正面页（奇数）:
  setRedDoubleBorder(dataColStart+0) → "月"列左边框红色双线
  setNoBorder(dataColStart+glColCount-1) → 末列✓右边框清除

背面页（偶数）:
  setNoBorder(dataColStart+0) → "月"列左边框清除
  setRedDoubleBorder(dataColStart+glColCount-1) → 末列✓右边框红色双线
```

纵向范围：从上边距第 1 行（Row 2）到底边距最后 1 行（每页末行）。

## Module Boundaries

| 模块 | 职责 | 改动 |
|---|---|---|
| `generator/layout/gl_layout.go` | 布局规格定义、坐标计算 | `pxPerColUnit` 常数 |
| `generator/gl_sheet.go` | GL 渲染器（标题、页头、数据写入、过次页、finalize） | 行偏移 + 外边框 |
| `generator/workbook.go` | 工作簿初始化、页面设置 | 新增 `SetPageLayout` |
| `generator/monthly_close.go` | 月末结账行 | 行偏移同步 |
| `generator/merge_gl_sheet.go` | 合并总分类账 | 行偏移同步 |

## Error Handling

- `SetPageLayout` / `SetPageMargins` 错误：打印时可能不符合 A4 → 日志警告
- `setNoBorder` 无样式时跳过（`styleID == 0`），不崩溃

## Testing Strategy

- 现有单元测试全绿验证偏移量正确
- 生成后目测打印预览是否符合 A4 横向
- 验证正面页/背面页外边缘边框是否存在且正确
