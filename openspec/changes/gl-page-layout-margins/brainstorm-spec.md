# GL 页面布局重构：边距 + A4 尺寸 + 外边框

## Context

当前 GL 总分类账的页面布局存在三个问题：

1. **边距不完整**：没有独立的上/下边距行，左右边距列也不够清晰。A4 纸的四边空白全部通过空行/空列模拟，但缺乏标准化的结构。
2. **A4 打印尺寸偏差**：`GLMMToExcelColWidth` 中 `pxPerColUnit = 3.5` 是正确值 `7.0` 的一半，导致列宽翻倍。Front+间隙+Back 总宽达 289 Excel 单位，远超 A4 横向可打印范围（~155 单位），打印时内容被切割到页外。
3. **外边缘边框缺失**：正面页外边缘（左侧）和背面页外边缘（右侧）缺少红色双线边框，且已有代码可能残留了不需要的边框。

## Goals

- **规范化四周边距**：通过空行（上/下各 3 行）和空列（左/右各 2 列）模拟 A4 四边
- **修正 A4 宽度**：`pxPerColUnit: 3.5 → 7.0`，添加 `SetPageLayout`（横向，A4），页边距设 0
- **外侧边缘红色双线贯穿整页**：正面页最左侧边框红色双线，背面页最右侧边框红色双线；对面侧主动清除边框

## Non-Goals

- 不调整行高/列宽的具体数值（仅修正换算系数，比例保持不变）
- 不改动 ML 布局
- 不改动数据流逻辑

## Decisions

### 1. 边距结构

| 边距 | 规格 |
|---|---|
| 上边距 | 3 空行（新增） |
| 下边距 | 3 空行（新增） |
| 左边距（正面页） | BindingLeftCols=2（列 A-B） |
| 右边距（背面页） | BindingRightCols=2（末 2 列） |
| 页间隙 | 列 O（保持现状） |

新增常量 `TopMarginRows = 3`、`BottomMarginRows = 3`。所有行偏移量（`DataStartRow`、`HeaderRow`、`SubHeaderRow`）加 `TopMarginRows`。每页新增 6 行。

### 2. A4 横向打印尺寸

```go
// gl_layout.go
const pxPerColUnit = 7.0  // 原 3.5

// workbook.go - 初始化时调用
f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
    Orientation: stringPtr("landscape"),
    PaperSize:   stringPtr(9),  // 9 = A4
})
f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
    Top:    float64Ptr(0),
    Bottom: float64Ptr(0),
    Left:   float64Ptr(0),
    Right:  float64Ptr(0),
})
```

行高换算不变（`mm × 72/25.4`）。

### 3. 外侧边缘红色双线

新增 `setNoBorder` 辅助函数，用于主动清除指定侧的边框。

实现流程（在 `finalizeAllGLSheets` 中逐页处理）：

```
正面页（奇数）:
  setRedDoubleBorder(dataColStart+0) → "月"列左边框红色双线
  setNoBorder(dataColStart+glColCount-1) → 末列✓清除右边框

背面页（偶数）:
  setNoBorder(dataColStart+0) → "月"列清除左边框
  setRedDoubleBorder(dataColStart+glColCount-1) → 末列✓右边框红色双线
```

红色双线纵向范围：从上边距第 1 行到底边距最后 1 行。

## Risks / Trade-offs

- **行偏移量变化影响范围大**：`DataStartRow` 从 5 变为 8，所有依赖此常量的函数（`appendToGLSheet`、`WriteMonthClosings`、`pageStartRow` 等）都需要同步调整。 → 一次性全局搜索替换。
- **后续页 offset 同步**：`writePageHeader` 中后续页的偏移量也需要加 `TopMarginRows`。
- **A4 兼容性**：设为 0 边距后，部分打印机可能仍需要最小边距。如果打印时内容被裁剪，再相应调整空行/空列的宽度。
- **列宽比例不变**：`pxPerColUnit` 修正后所有列按比例缩小，不改变数据展示效果。
