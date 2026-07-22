## Context

当前总分类账所有数据（标题+数据行）全部写入 `FrontStartCol` 区域（列 C~J，装订列之后）。反面区域（`BackStartCol` 起的列）已经预留了列宽，但未被写入任何内容。正反面并排的 Layout 已经就绪，只差 Renderer 按 pageNum 奇偶性选择写入区域。

之前 design.md 已确定"反面页二期实现"，现在正式实施。

## Goals / Non-Goals

**Goals:**
- 奇数页数据写入 FrontStartCol 区域（不变）
- 偶数页数据写入 BackStartCol 区域（新增）
- 偶数页标题（分第 n 页、总分类账、科目名称、列标题）写入 BackStartCol 区
- 页码连续：正反交替（1=正, 2=反, 3=正, 4=反…）
- 完整 e2e 验证（含跨年）

**Non-Goals:**
- 不改 ML 正反面（change 4）
- 不改 LayoutSpec（现有 Front/Back/PageGap 坐标已就位）
- 不改 year_close 和打印标记（change 5）

## Decisions

### 决策 1：写入区域选择

`appendToGLSheet` 和 `writePageHeader` 中，根据 pageNum 奇偶选择写入起始列：

```go
func dataCol(lay layout.Layout, pageNum, offset int) int {
    if pageNum%2 == 1 { // 奇数页→正面
        return lay.FrontStartCol + offset
    }
    return lay.BackStartCol + offset // 偶数页→反面
}
```

### 决策 2：列宽一致性

反面区列宽已在 `writeGLTitle` 中设置（与正面相同的 `avgWidth`），无需额外设置。

### 决策 3：偶数页合并总分类账标题列范围

偶数页标题使用 `BackStartCol` 起始列，`TitleColLeft/Right` 等标题分区定义保持不变（基于 FrontStartCol 计算），偶数页时需要重新计算：

```go
backTitleLeft := lay.BackStartCol
backTitleRight := lay.BackStartCol + lay.TitleColSpan - 1
backAccountLeft := lay.BackStartCol + lay.TitleColSpan
backAccountRight := lay.BackStartCol + lay.TitleColSpan + lay.AccountColSpan - 1
```

### 决策 4：影响范围

- `gl_sheet.go`：`appendToGLSheet` 数据写入 + `writePageHeader` 标题写入
- `merge_gl_sheet.go`：`appendToMergeGLSheet` 数据写入（使用了相同的 Layout 坐标模式）
- `monthly_close.go`：`WriteMonthClosings` 关账行写入
- `print_mark.go`：`markRowForPrint` 打印标记列
- `workbook.go`：`ExtractLastMonthFinals` 读取不受影响（读的是"期末余额"文本位置，列已由 BindingLeftCols 偏移）

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| 页面间隙列（PageGapStartCol）数据写入错误 | 偶数页从 BackStartCol 起始，天然跳过间隙列 |
| 偶数页数据跨页写入覆盖到下一正面区 | TotalCols 限制 + 数据列数 nCol 固定，不超界 |
| 打印时选反面区域不直观 | 操作流程中给出选区打印指引 |
