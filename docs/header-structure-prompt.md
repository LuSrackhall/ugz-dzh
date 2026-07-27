# GL/ML 表头两行两列结构规范

## 结构说明

账页表头由两行组成，第一行合并列，第二行列出子列名：

```
Row 4 (HeaderRow):  [2025年      ] [凭证  ] [摘要] [借方金额] [贷方金额] [方向] [余额]
Row 5 (SubHeaderRow): [月] [日] [字] [号]
```

## 合并规则

| 第一行合并单元格 | 覆盖的子列 | 合并方式 |
|---|---|---|
| `年`（如 "2025年"） | 月、日 | `MergeCell(col, col+1)` |
| 凭证 | 字、号 | `MergeCell(col+2, col+3)` |
| 摘要 ~ 余额 | 各自独立 | 不合并，单列 |

## 代码示例（GL 写法）

```go
// Row 4: 顶层列标题
year := wb.Month[:4]
yearLeft := cellName(lay.FrontStartCol, lay.HeaderRow+1)
yearRight := cellName(lay.FrontStartCol+1, lay.HeaderRow+1)
wb.File.MergeCell(sheet, yearLeft, yearRight)
wb.File.SetCellValue(sheet, yearLeft, year+"年")

// 凭证合并字+号两列
vouchLeft := cellName(lay.FrontStartCol+2, lay.HeaderRow+1)
vouchRight := cellName(lay.FrontStartCol+3, lay.HeaderRow+1)
wb.File.MergeCell(sheet, vouchLeft, vouchRight)
wb.File.SetCellValue(sheet, vouchLeft, "凭证")

// 摘要/借方/贷方/方向/余额 各占单列
otherCols := []string{"摘要", "借方金额", "贷方金额", "方向", "余额"}
for i, h := range otherCols {
    cell := cellName(lay.FrontStartCol+4+i, lay.HeaderRow+1)
    wb.File.SetCellValue(sheet, cell, h)
}

// Row 5: 子表头
wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol, lay.SubHeaderRow+1), "月")
wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+1, lay.SubHeaderRow+1), "日")
wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+2, lay.SubHeaderRow+1), "字")
wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+3, lay.SubHeaderRow+1), "号")
```

## 样式

- 字体：绿色 `#006100`，加粗，10pt
- 对齐：水平居中，垂直居中
- 背景：浅蓝 `#D9E1F2`
- 边框：底部灰色 `#808080` 实线

## 关键 Layout 字段

| 字段 | 含义 | GL 默认值 |
|---|---|---|
| `HeaderRow` | 第一行表头行号（0-indexed） | 3 |
| `SubHeaderRow` | 第二行子表头行号 | 4 |
| `FrontStartCol` | 数据区起始列（1-indexed Excel 列号） | 3 |
| `DataStartRow` | 数据区起始行号 | 5 |

## 注意事项

- 合并单元格只在第一行（HeaderRow），第二行不合并
- 第二行只有前 4 列（月/日/字/号）有内容，摘要~余额列在第二行为空
- 年份从 `wb.Month[:4]` 截取，不硬编码
- Back 区的写法相同，只需在列号上加 `colOffset`
