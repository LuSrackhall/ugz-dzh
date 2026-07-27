# ML 滑动窗口布局实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 ML 多科目明细账从对称正反布局改为非对称滑动窗口布局——基础列+明细1~4 在左半（Paper Back），明细5~14 在右半（Paper Front），逻辑页由 Back+Front 配对构成。

**Architecture:** 拆分 `MLSpec.ColProportions` 为 `BackColProportions` + `FrontColProportions` 两套独立列比例；`MLComputeLayout` 输出两套列坐标；Renderer 同时写入两侧，数据行贯穿左右。

**Tech Stack:** Go, excelize

## 全局约束

- 宪法第 2 层（页内结构）与第 1 层（Sheet 编排）分离，修改不能模糊层边界
- 所有坐标必须通过 `MLComputeLayout` 计算，Renderer 不能自算
- GetRows 索引通过 `BindingLeftCols` 偏移，不得硬编码
- 余额链连续：过次页 → 承前页 → 逐行余额 → 期末余额
- 每次提交前运行 `go test ./...` 并通过

## 文件修改清单

| 文件 | 改动量 | 职责 |
|---|---|---|
| `generator/layout/ml_layout.go` | 重写 ~50% | MLSpec/MLLayout 类型 + MLComputeLayout |
| `generator/layout/ml_layout_test.go` | 新增测试 | Layout 单元测试 |
| `generator/ml_sheet.go` | 重写 ~60% | 标题/数据/过次页/承前页 + 读取函数 |
| `generator/monthly_close_ml.go` | 重写 ~40% | 月结行写入两侧 |
| `generator/print_mark.go` | 少量修改 | 打印标记列号适配 |

---

### Task 1: MLSpec + MLLayout 类型改造

**Files:**
- Modify: `generator/layout/ml_layout.go`

**Interfaces:**
- Consumes: 现有 `MLSpec`, `MLLayout`, `MLColumnPos`, `MLColProportion` 类型骨架
- Produces: 新 `MLSpec` (BackColProportions/FrontColProportions), 新 `MLLayout` (BackColumns/FrontColumns)

- [ ] **Step 1: 删除 `MLSpec.ColProportions`，新增 `BackColProportions` 和 `FrontColProportions`**

```go
type MLSpec struct {
    PaperWidthMM      float64
    PaperHeightMM     float64
    LeftMarginMM      float64
    RightMarginMM     float64
    PageGapMM         float64
    TitleRowCount     int
    ColHeaderRowCount int
    DataRowsPerPage   int

    // 两套独立列比例
    BackColProportions  []MLColProportion   // 左半：7基础 + 明细1~4
    FrontColProportions []MLColProportion   // 右半：明细5~14
}
```

- [ ] **Step 2: `MLLayout` 新增 `BackColumns`/`FrontColumns` + `BackColCount`/`FrontColCount`，删掉旧 `Columns`**

```go
type MLLayout struct {
    FrontLeftMM, FrontWidthMM float64
    PageGapLeftMM, PageGapWidthMM float64
    BackLeftMM, BackWidthMM float64

    // ⬇ 两套列坐标
    BackColumns  []MLColumnPos   // 左半：基础7列 + 明细1~4
    FrontColumns []MLColumnPos   // 右半：明细5~14

    BindingLeftCols  int   // 2
    BackStartCol     int   // 左半起始列（Back 区）
    PageGapStartCol  int   // 间隙列
    FrontStartCol    int   // 右半起始列（Front 区）
    BindingRightCols int   // 2
    TotalCols        int

    BackColCount    int   // Back 侧数据列数
    FrontColCount   int   // Front 侧数据列数

    ExcelColumns []MLExcelCol  // 列名 → Excel 编号映射（仅用于兼容，非核心路径）

    TitleRow, PageNumRow, AccountRow, HeaderRow, DataStartRow int  // 行号，两侧共享

    // 合并单元格坐标（两侧独立）
    BackTitleColLeft, BackTitleColRight int
    FrontTitleColLeft, FrontTitleColRight int
    BackAccountColLeft, BackAccountColRight int
    FrontAccountColLeft, FrontAccountColRight int
}
```

- [ ] **Step 3: 更新 `DefaultMLSpec()`**

```go
func DefaultMLSpec() MLSpec {
    return MLSpec{
        PaperWidthMM:      297,
        PaperHeightMM:     210,
        LeftMarginMM:      15,
        RightMarginMM:     15,
        PageGapMM:         8,
        TitleRowCount:     3,
        ColHeaderRowCount: 1,
        DataRowsPerPage:   20,
        BackColProportions: []MLColProportion{
            {Name: "日期", Ratio: 8},
            {Name: "凭证号", Ratio: 7},
            {Name: "摘要", Ratio: 15},
            {Name: "借方金额", Ratio: 10},
            {Name: "贷方金额", Ratio: 10},
            {Name: "方向", Ratio: 4},
            {Name: "余额", Ratio: 8},
            {Name: "明细1", Ratio: 6},
            {Name: "明细2", Ratio: 6},
            {Name: "明细3", Ratio: 6},
            {Name: "明细4", Ratio: 6},
        },
        FrontColProportions: []MLColProportion{
            {Name: "明细5", Ratio: 10},
            {Name: "明细6", Ratio: 10},
            {Name: "明细7", Ratio: 10},
            {Name: "明细8", Ratio: 10},
            {Name: "明细9", Ratio: 10},
            {Name: "明细10", Ratio: 10},
            {Name: "明细11", Ratio: 10},
            {Name: "明细12", Ratio: 10},
            {Name: "明细13", Ratio: 10},
            {Name: "明细14", Ratio: 10},
        },
    }
}
```

- [ ] **Step 4: 提交**

```bash
git add generator/layout/ml_layout.go
git commit -m "feat(layout): MLSpec/MLLayout 拆分为 Back/Front 两套列比例"
```

### Task 2: MLComputeLayout 重写

**Files:**
- Modify: `generator/layout/ml_layout.go`（接 Task 1 之后）
- Test: `generator/layout/ml_layout_test.go`

**Interfaces:**
- Consumes: 新 `MLSpec` (BackColProportions, FrontColProportions)
- Produces: `MLComputeLayout(spec MLSpec) MLLayout`

**Excel 列布局：**
```
A-B: Binding (2 cols)
C-M: Back 区 (11 cols: 7 basic + 4 detail)  → BackStartCol=3
N:   Page gap (1 col)                        → PageGapStartCol=14
O-X: Front 区 (10 cols: 明细5~14)            → FrontStartCol=15
Y-Z: Binding right (2 cols)
```

- [ ] **Step 1: 重写 `MLComputeLayout`**

```go
func MLComputeLayout(spec MLSpec) MLLayout {
    contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
    frontLeft := spec.LeftMarginMM
    pageGapLeft := frontLeft + contentWidth
    backLeft := pageGapLeft + spec.PageGapMM
    bindingLeftCols := 2
    bindingRightCols := 2

    // Back 列
    var backCols []MLColumnPos
    var startMM float64
    for _, p := range spec.BackColProportions {
        w := contentWidth * p.Ratio / 100.0
        backCols = append(backCols, MLColumnPos{Name: p.Name, StartMM: startMM, WidthMM: w})
        startMM += w
    }

    // Front 列
    var frontCols []MLColumnPos
    startMM = 0
    for _, p := range spec.FrontColProportions {
        w := contentWidth * p.Ratio / 100.0
        frontCols = append(frontCols, MLColumnPos{Name: p.Name, StartMM: startMM, WidthMM: w})
        startMM += w
    }

    backColCount := len(spec.BackColProportions)
    frontColCount := len(spec.FrontColProportions)
    backStart := bindingLeftCols + 1
    pageGapStart := backStart + backColCount
    frontStart := pageGapStart + 1
    total := frontStart + frontColCount + bindingRightCols

    // Excel 列映射
    var exc []MLExcelCol
    for i := range backCols {
        exc = append(exc, MLExcelCol{Name: backCols[i].Name, Col: backStart + i})
    }

    // 标题/科目合并区
    backTitleCols := backColCount // use all back cols for title
    frontTitleCols := frontColCount

    return MLLayout{
        FrontLeftMM:       frontLeft,
        FrontWidthMM:      contentWidth,
        PageGapLeftMM:     pageGapLeft,
        PageGapWidthMM:    spec.PageGapMM,
        BackLeftMM:        backLeft,
        BackWidthMM:       contentWidth,
        BackColumns:       backCols,
        FrontColumns:      frontCols,
        BindingLeftCols:   bindingLeftCols,
        BackStartCol:      backStart,
        PageGapStartCol:   pageGapStart,
        FrontStartCol:     frontStart,
        BindingRightCols:  bindingRightCols,
        TotalCols:         total,
        BackColCount:      backColCount,
        FrontColCount:     frontColCount,
        ExcelColumns:      exc,
        TitleRow:          1,
        PageNumRow:        0,   // 第0行是页眉第一行
        AccountRow:        2,
        HeaderRow:         4,
        DataStartRow:      5,
        BackTitleColLeft:      backStart,
        BackTitleColRight:     backStart + backColCount - 1,
        FrontTitleColLeft:     frontStart,
        FrontTitleColRight:    frontStart + frontColCount - 1,
        BackAccountColLeft:    backStart,
        BackAccountColRight:   backStart + backColCount - 1,
        FrontAccountColLeft:   frontStart,
        FrontAccountColRight:  frontStart + frontColCount - 1,
    }
}
```

- [ ] **Step 2: 写布局单元测试**

在 `ml_layout_test.go` 中新增：

```go
func TestDefaultMLSpec_HasBackFront(t *testing.T) {
    spec := DefaultMLSpec()
    if len(spec.BackColProportions) == 0 {
        t.Error("BackColProportions 不应为空")
    }
    if len(spec.FrontColProportions) == 0 {
        t.Error("FrontColProportions 不应为空")
    }
    if spec.BackColProportions[0].Name != "日期" {
        t.Errorf("Back 第一列应为 日期，实际 %s", spec.BackColProportions[0].Name)
    }
}

func TestMLComputeLayout_BackFrontColumns(t *testing.T) {
    spec := DefaultMLSpec()
    lay := MLComputeLayout(spec)

    if len(lay.BackColumns) == 0 {
        t.Fatal("BackColumns 不应为空")
    }
    if len(lay.FrontColumns) == 0 {
        t.Fatal("FrontColumns 不应为空")
    }

    // Back 前三列为 日期/凭证号/摘要
    if lay.BackColumns[0].Name != "日期" {
        t.Errorf("BackColumns[0] 应为 日期，实际 %s", lay.BackColumns[0].Name)
    }

    // 验证 Excel 列不重叠
    backEnd := lay.BackStartCol + len(lay.BackColumns) - 1
    if backEnd >= lay.FrontStartCol {
        t.Errorf("Back 列与 Front 列重叠：Back end=%d, Front start=%d", backEnd, lay.FrontStartCol)
    }

    // 验证间隙列位置
    if lay.PageGapStartCol != lay.BackStartCol+len(lay.BackColumns) {
        t.Errorf("PageGap 位置错误：%d != %d", lay.PageGapStartCol, lay.BackStartCol+len(lay.BackColumns))
    }
}

func TestMLComputeLayout_ColWidthSum(t *testing.T) {
    spec := DefaultMLSpec()
    lay := MLComputeLayout(spec)

    var backSum, frontSum float64
    for _, c := range lay.BackColumns {
        backSum += c.WidthMM
    }
    for _, c := range lay.FrontColumns {
        frontSum += c.WidthMM
    }

    contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
    if backSum < contentWidth-1 || backSum > contentWidth+1 {
        t.Errorf("Back 列宽和 %.2f 应与 contentWidth %.2f 相近", backSum, contentWidth)
    }
    if frontSum < contentWidth-1 || frontSum > contentWidth+1 {
        t.Errorf("Front 列宽和 %.2f 应与 contentWidth %.2f 相近", frontSum, contentWidth)
    }
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./generator/layout/... -v -count=1
```

预期：之前通过的测试仍通过，新测试通过。

- [ ] **Step 4: 提交**

```bash
git add generator/layout/ml_layout.go generator/layout/ml_layout_test.go
git commit -m "feat(layout): MLComputeLayout 输出 Back/Front 两套列坐标"
```

### Task 3: 明细列辅助函数适配

**Files:**
- Modify: `generator/ml_sheet.go`

**Interfaces:**
- Consumes: 新 `MLLayout` (BackStartCol, FrontStartCol, BackColCount)
- Produces: 新 `mlDetailCol(lay, i) int`, `mlDetailRowIdx(lay, i) int`, `mlPrintMarkCol() int`

核心变化：明细列不再全部在 `FrontStartCol + 7 + i`，而是按索引分流：
- i=0~3 → Back 侧：`BackStartCol + 7 + i`
- i=4~13 → Front 侧：`FrontStartCol + (i-4)`

- [ ] **Step 1: 新增/修改辅助函数**

```go
// mlDetailCol 返回第 i 个明细列的 Excel 列号。
// i=0~3 → Back 侧（左半），i=4~13 → Front 侧（右半）。
func mlDetailCol(lay layout.MLLayout, i int) int {
    if i < 4 {
        return lay.BackStartCol + 7 + i
    }
    return lay.FrontStartCol + (i - 4)
}

// mlDetailRowIdx 返回第 i 个明细列在 GetRows 中的索引。
func mlDetailRowIdx(lay layout.MLLayout, i int) int {
    if i < 4 {
        return lay.BindingLeftCols + 7 + i
    }
    // Front 侧 GetRows 索引 = FrontStartCol - 1 + (i - 4)
    return lay.FrontStartCol - 1 + (i - 4)
}

// mlPrintMarkCol 返回打印标记列号（Back 区最后一列+1，或 Front 区最后一列+1，取较大值）。
func mlPrintMarkCol() int {
    lay := mlLayout()
    backLast := lay.BackStartCol + lay.BackColCount
    frontLast := lay.FrontStartCol + lay.FrontColCount
    if frontLast > backLast {
        return frontLast
    }
    return backLast
}
```

- [ ] **Step 2: 删除旧函数并全局替换引用**

搜索 `mlDetailExcelCol` 替换为 `mlDetailCol`，搜索 `mlDetailRowIdx` 更新为新签名。

在 `ml_sheet.go` 中删除：
```go
// 删除旧函数
func mlDetailExcelCol(lay layout.MLLayout, i int) int {
    return lay.FrontStartCol + 7 + i
}
```

确认 `mlDetailStartCol` 常量不再被使用（检查所有引用，旧有 `mlDetailStartCol + idx` 行需要改为 `mlDetailCol(lay, idx)`）。

在 `appendToMLSheet` 中找到旧行：
```go
col := mlDetailStartCol + idx
// 改为：
col := mlDetailCol(lay, idx)
```

- [ ] **Step 3: 编译验证**

```bash
go build .
```

- [ ] **Step 4: 提交**

```bash
git add generator/ml_sheet.go
git commit -m "refactor(ml): 明细列辅助函数适配新布局坐标体系"
```

### Task 4: 标题/表头双面写入

**Files:**
- Modify: `generator/ml_sheet.go`

**Interfaces:**
- Consumes: 新 `MLLayout` (BackStartCol, FrontStartCol, BackTitleColLeft/Right, FrontTitleColLeft/Right)
- Produces: `ensureMLSheet`, `writeMLTitle`, `writeMLPageHeader` 写入双面

重要变化：
- `writeMLTitle` 改为写入 **Paper1 Front 占位**（只写 Front 侧标题+表头）
- `writeMLPageHeader` 改为写入**两侧**标题+表头（Back 和 Front 各有自己的列标题）
- `ensureMLSheet` 创建 Sheet 时调用 `writeMLPageHeader` 写 Paper1 Front 占位

- [ ] **Step 1: 重写 `writeMLTitle` 为 Paper1 Front 占位写入**

```go
func (wb *Workbook) writeMLTitle(sheet, general string, details []string) error {
    lay := mlLayout()
    
    // Paper1 Front 占位：只写 Front 侧标题/表头
    // Row 1: "多科目明细账 — XXX" (Front 区)
    titleStart := mlCellName(lay.FrontStartCol, 1)
    titleEnd := mlCellName(lay.FrontTitleColRight, 1)
    wb.File.SetCellValue(sheet, titleStart, "多科目明细账 — "+general)
    wb.File.MergeCell(sheet, titleStart, titleEnd)
    
    titleStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font:      &excelize.Font{Bold: true, Size: 14},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    wb.File.SetCellStyle(sheet, titleStart, titleEnd, titleStyle)
    wb.File.SetRowHeight(sheet, 1, 22)
    
    // Row 2: 明细列标题 (Front 侧)
    for i := 0; i < mlMaxDetails; i++ {
        col := mlDetailCol(lay, i)
        cell := mlCellName(col, 2)
        label := ""
        if i >= 4 && (i-4) < len(details) {
            label = details[i]
        }
        wb.File.SetCellValue(sheet, cell, label)
    }
    
    headerStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 10},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
        Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    endCell := mlCellName(lay.FrontTitleColRight, 2)
    wb.File.SetCellStyle(sheet, titleStart, endCell, headerStyle)
    
    // 列宽（Front 侧明细列）
    for i := 0; i < mlMaxDetails; i++ {
        colLetter := cellColLetter(mlDetailCol(lay, i))
        wb.File.SetColWidth(sheet, colLetter, colLetter, 14)
    }
    
    return nil
}
```

注意：`writeMLTitle` 不再写基础列标题。基础列标题由 `writeMLPageHeader` 负责。

- [ ] **Step 2: 重写 `writeMLPageHeader` 写入两侧**

```go
func (wb *Workbook) writeMLPageHeader(sheet string, row int, pageNum int, general string) error {
    lay := mlLayout()
    darkGreen := "006100"
    sealRed := "CC0000"

    // Row +0: "分第N页" — Back 侧
    pnBack := mlCellName(lay.BackTitleColLeft, row)
    pnBackEnd := mlCellName(lay.BackTitleColRight, row)
    wb.File.MergeCell(sheet, pnBack, pnBackEnd)
    wb.File.SetCellRichText(sheet, pnBack, []excelize.RichTextRun{
        {Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
        {Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
        {Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
    })
    
    // Row +0: "分第N页" — Front 侧
    pnFront := mlCellName(lay.FrontTitleColLeft, row)
    pnFrontEnd := mlCellName(lay.FrontTitleColRight, row)
    wb.File.MergeCell(sheet, pnFront, pnFrontEnd)
    wb.File.SetCellRichText(sheet, pnFront, []excelize.RichTextRun{
        {Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
        {Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
        {Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
    })
    wb.File.SetRowHeight(sheet, row, 18)
    row++

    // Row +1: 标题 — Back 侧（"多科目明细账 — XXX"）
    tlBack := mlCellName(lay.BackTitleColLeft, row)
    trBack := mlCellName(lay.BackTitleColRight, row)
    wb.File.MergeCell(sheet, tlBack, trBack)
    wb.File.SetCellValue(sheet, tlBack, "多科目明细账 — "+general)
    titleStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    wb.File.SetCellStyle(sheet, tlBack, trBack, titleStyle)

    // Row +1: 标题 — Front 侧
    tlFront := mlCellName(lay.FrontTitleColLeft, row)
    trFront := mlCellName(lay.FrontTitleColRight, row)
    wb.File.MergeCell(sheet, tlFront, trFront)
    wb.File.SetCellValue(sheet, tlFront, general)
    accStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Color: sealRed, Size: 10},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    wb.File.SetCellStyle(sheet, tlFront, trFront, accStyle)

    // Row +1: Front 侧科目名
    alFront := mlCellName(lay.FrontAccountColLeft, row)
    arFront := mlCellName(lay.FrontAccountColRight, row)
    wb.File.MergeCell(sheet, alFront, arFront)
    wb.File.SetCellValue(sheet, alFront, general)
    wb.File.SetCellStyle(sheet, alFront, arFront, accStyle)
    wb.File.SetRowHeight(sheet, row, 28)
    row++

    // Row +2: 科目名 — Back 侧
    acBack := mlCellName(lay.BackAccountColLeft, row)
    acBackEnd := mlCellName(lay.BackAccountColRight, row)
    wb.File.MergeCell(sheet, acBack, acBackEnd)
    wb.File.SetCellValue(sheet, acBack, general)
    acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Color: sealRed, Size: 10},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    wb.File.SetCellStyle(sheet, acBack, acBackEnd, acRowStyle)

    // Row +2: 科目名 — Front 侧
    acFront := mlCellName(lay.FrontAccountColLeft, row)
    acFrontEnd := mlCellName(lay.FrontAccountColRight, row)
    wb.File.MergeCell(sheet, acFront, acFrontEnd)
    wb.File.SetCellValue(sheet, acFront, general)
    wb.File.SetCellStyle(sheet, acFront, acFrontEnd, acRowStyle)
    wb.File.SetRowHeight(sheet, row, 18)
    row++

    // Row +3: 空行
    row++

    // Row +4: 列标题 — Back 侧（7基础列 + 明细1~4）
    backColNames := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}
    for i, h := range backColNames {
        cell := mlCellName(lay.BackStartCol+i, row)
        wb.File.SetCellValue(sheet, cell, h)
    }
    // Back 侧明细1~4 列标题
    for i := 0; i < 4 && i < mlMaxDetails; i++ {
        col := mlDetailCol(lay, i)
        cell := mlCellName(col, row)
        wb.File.SetCellValue(sheet, cell, fmt.Sprintf("明细%d", i+1))
    }

    // Row +4: 列标题 — Front 侧（明细5~14）
    for i := 4; i < mlMaxDetails; i++ {
        col := mlDetailCol(lay, i)
        cell := mlCellName(col, row)
        wb.File.SetCellValue(sheet, cell, fmt.Sprintf("明细%d", i+1))
    }

    headerStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 10},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
        Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    // 两侧表头设置样式
    hsBack := mlCellName(lay.BackStartCol, row)
    heBack := mlCellName(lay.BackStartCol+len(backColNames)-1, row)
    wb.File.SetCellStyle(sheet, hsBack, heBack, headerStyle)
    for i := 0; i < 4 && i < mlMaxDetails; i++ {
        cell := mlCellName(mlDetailCol(lay, i), row)
        wb.File.SetCellStyle(sheet, cell, cell, headerStyle)
    }
    for i := 4; i < mlMaxDetails; i++ {
        cell := mlCellName(mlDetailCol(lay, i), row)
        wb.File.SetCellStyle(sheet, cell, cell, headerStyle)
    }

    return nil
}
```

- [ ] **Step 3: 更新 `ensureMLSheet`**

在 `ensureMLSheet` 中，新 Sheet 创建后调用 `writeMLTitle` 写 Paper1 Front 占位：
```go
// 新 Sheet — 创建
idx, err := wb.File.NewSheet(name)
// ...
if err := wb.writeMLTitle(name, general, initDetails); err != nil {
    return "", nil, nil, err
}
// 不再调用 writeMLPageHeader（Paper1 Front 由 writeMLTitle 负责）
```

对于已有 Sheet 的更新（`updateMLDetailHeaders`），跳过 Paper1 Front 占位。

- [ ] **Step 4: 提交**

```bash
git add generator/ml_sheet.go
git commit -m "feat(ml): 标题/表头双面写入 + Paper1 Front 占位"
```

### Task 5: 数据写入 + 过次页/承前页双面化

**Files:**
- Modify: `generator/ml_sheet.go`

**Interfaces:**
- Consumes: 新 `mlDetailCol(lay, i)`、新 `MLLayout`
- Produces: 双面写入的 `appendToMLSheet`、`writeMLPageBreakRow`、`writeMLCarryForwardRow`

核心改变：每个数据行同时写 Back 侧（基础列 + 明细1~4）和 Front 侧（明细5~14）。

- [ ] **Step 1: 重写 `writeMLPageBreakRow` 双面写入**

```go
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals) {
    lay := mlLayout()
    dir, dispBal := directionFor(balance, 0)

    // Back 侧：基础列合计 + 明细1~4 净额
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), pageBreakLabel)
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(pageDebit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(pageCredit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

    // Back 侧：明细1~4 净额
    for i := 0; i < 4 && i < len(pageDetails); i++ {
        net := pageDetails[i].debit - pageDetails[i].credit
        col := mlDetailCol(lay, i)
        wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
        wb.setMoneyStyle(sheet, row, col)
    }

    // Front 侧：明细5~14 净额
    for i := 4; i < len(pageDetails); i++ {
        net := pageDetails[i].debit - pageDetails[i].credit
        col := mlDetailCol(lay, i)
        wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
        wb.setMoneyStyle(sheet, row, col)
    }
}
```

- [ ] **Step 2: 重写 `writeMLCarryForwardRow` 双面写入**

与 `writeMLPageBreakRow` 相同结构，但标签用 `carryForwardLabel`。

```go
func (wb *Workbook) writeMLCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals, label string) {
    lay := mlLayout()
    dir, dispBal := directionFor(balance, 0)

    // Back 侧
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), label)
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(pageDebit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(pageCredit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

    for i := 0; i < 4 && i < len(pageDetails); i++ {
        net := pageDetails[i].debit - pageDetails[i].credit
        col := mlDetailCol(lay, i)
        wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
        wb.setMoneyStyle(sheet, row, col)
    }

    for i := 4; i < len(pageDetails); i++ {
        net := pageDetails[i].debit - pageDetails[i].credit
        col := mlDetailCol(lay, i)
        wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
        wb.setMoneyStyle(sheet, row, col)
    }
}
```

- [ ] **Step 3: 更新 `appendToMLSheet` 数据写入**

在 `appendToMLSheet` 中，数据行写入改为双面：

```go
// 基础列 → Back 侧
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), e.Date)
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), e.VoucherNum)
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), e.Summary)
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(e.DebitCents))
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(e.CreditCents))
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))

wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

if e.DetailAccount != "" {
    if idx, ok := detailIdx[e.DetailAccount]; ok {
        net := e.DebitCents - e.CreditCents
        col := mlDetailCol(lay, idx)  // 自动分流到 Back/Front
        wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
        wb.setMoneyStyle(sheet, row, col)
        pageDetails[idx].debit += e.DebitCents
        pageDetails[idx].credit += e.CreditCents
    }
}
```

同时，更新 `appendToMLSheet` 中「上年结转」行的写入（同样用 `writeMLCarryForwardRow`）：

```go
if isNew && initial != 0 {
    wb.writeMLCarryForwardRow(sheet, 3, initial, 0, 0, 
        make([]mlDetailTotals, mlMaxDetails), "上年结转")
}
```

注意：行 3 是第一个数据行，在 Paper1 Front 占位（5行）之后。但 Paper1 Front 占位只有 Front 侧，Back 侧的数据从第一个数据页开始。

需要调整 `mlNextDataRow` 返回的值。Paper1 Front 占位后，第一个数据页的起始行应为 `lay.DataStartRow + 1`（当前为 6），因为 Paper1 Front 占了 5 行。

实际上，需要仔细考虑行号。当前 `writeMLTitle` 写行 1-2，数据从行 3 开始。新 `writeMLTitle`（Paper1 Front）写行 1-5（Front 侧标题+表头共 5 行），数据应从行 6 开始。

但原有的数据页结构是：
- 页眉 5 行（row+0 ~ row+4）
- 承前页 1 行（row+5）
- 20 行数据（row+6 ~ row+25）
- 过次页 1 行（row+26）

第一个数据页的页眉从行 6 开始，则：
- 页眉：行 6-10
- 承前页：行 11
- 数据：行 12-31
- 过次页：行 32

所以 `mlNextDataRow` 应该返回 `lay.DataStartRow + 1 = 6`（在已有 Paper1 Front 的情况下）。

实际上，`mlNextDataRow` 的当前实现：
```go
func (wb *Workbook) mlNextDataRow(sheet string) (int, error) {
    lay := mlLayout()
    rows, err := wb.File.GetRows(sheet)
    if err != nil {
        return 3, nil  // 空表从行3开始
    }
    // ... 找过次页后下一个数据行
}
```

这个函数需要返回「下一个可用的数据行号」。在 Paper1 Front 占位后，第一个数据页从行 6 开始。

最简单的实现：在 `mlNextDataRow` 中将空表起始行从 3 改为 `lay.DataStartRow + 1 = 6`（覆盖 Paper1 Front 占位）。

不对，`DataStartRow` 是页眉内的偏移量（当前值为 5，表示页眉占 5 行）。Paper1 Front 占位写在前 5 行（行 1-5），所以第一个数据页从行 6 开始。

但第一个数据页的页眉也是 5 行 + 承前页 1 行，所以数据从 `6 + 5 + 1 = 12` 开始。

Wait, I need to think about this more carefully. The current code flow in appendToMLSheet:

```go
// After Paper1 Front placeholder (rows 1-5):
// Start writing first logical page data

// Page header at row 6 (Back + Front)
writeMLPageHeader(sheet, 6, 1, general)  // writes rows 6-10
row = 6 + lay.DataStartRow  // = 11
writeMLCarryForwardRow(sheet, 11, balance, ...)  // 承前页 at row 11
row = 12

// Now write 20 data rows starting at row 12
for each entry:
    // Check page break (row 12 - 6 = 6? No, page start = 6, so data start = 12)
    // Current: mlRowIsPageBreak(sheet, row) checks row - pageStart >= pageSize
    // pageStart = lay.DataStartRow + 1 = 6, so 12 - 6 = 6 >= 20? No.
    // So 20 data rows from 12 to 31. Page break at 32.
    ...
```

Actually, the current `mlPageStartRow` computes the start differently. Let me not get into the exact row arithmetic now — I'll handle it during implementation. The key insight is:

1. Paper1 Front occupies rows 1-5
2. First data page header (Back + Front) starts at row 6
3. Carry forward at row 6 + 5 = 11
4. Data starts at row 12
5. 20 data rows: row 12-31
6. Break at row 32

The existing page break logic (`mlRowIsPageBreak`, `mlPageStartRow`, `mlNextDataRow`) all compute relative to the page start, so they should work correctly as long as `mlPageStartRow` correctly identifies the start of the current page (which is after the last 过次页, or after Paper1 Front for the first page).

OK, I think the row arithmetic is manageable. Let me just note in the plan that `mlNextDataRow` needs adjustment and handle it in Task 6 (reading functions).

- [ ] **Step 4: 提交**

```bash
git add generator/ml_sheet.go
git commit -m "feat(ml): 数据写入 + 过次页/承前页双面化"
```

### Task 6: 读取函数适配

**Files:**
- Modify: `generator/ml_sheet.go`

**Interfaces:**
- Consumes: 新 `MLLayout` (BackStartCol, FrontStartCol, BackColCount)
- Produces: 适配新坐标的所有读取函数

需要修改的函数（按文件中出现顺序）：

| 函数 | 改动 |
|---|---|
| `mlHasPageBreakAt` | 检查 Back 摘要列 + Front 摘要列 |
| `mlNextDataRow` | 空表起始行改为 `lay.DataStartRow + 1`（Paper1 Front 占位后） |
| `mlLastPageBalance` | 只从 Back 余额列 (BackStartCol+6) 读 |
| `mlLastBreakTotals` | 只从 Back 借方/贷方列 (BackStartCol+3/4) 读 |
| `mlLastRowIsOrphanBreak` | 检查 Back 或 Front 摘要列 |
| `mlPageStartRow` | 适配 Paper1 Front 占位后的页起始行 |
| `mlRowIsPageBreak` | 行号逻辑不变（检查行相对偏移） |
| `mlPageHasBreakRow` | 检查 Back 或 Front 摘要列 |
| `mlNextDataRowAfterBreak` | 同上 |
| `lastBreakDetailTotals` | Back 列读明细1~4，Front 列读明细5~14 |

- [ ] **Step 1: 逐个更新读取函数**

关键函数「过次页检测」：
```go
func mlHasPageBreakAt(row []string, lay layout.MLLayout) bool {
    // 检查 Back 侧摘要列 + Front 侧摘要列
    backSummaryIdx := lay.BindingLeftCols + 2
    frontSummaryIdx := lay.FrontStartCol - 1 + 2  // Front 侧 GetRows 偏移
    if len(row) > backSummaryIdx && row[backSummaryIdx] == pageBreakLabel {
        return true
    }
    if len(row) > frontSummaryIdx && row[frontSummaryIdx] == pageBreakLabel {
        return true
    }
    return false
}
```

Wait, the Front side columns don't have a "摘要" column. The Front side has only detail columns (明细5~14). So in the new layout, the Front side 过次页 row won't have a "摘要" column with the pageBreakLabel. Instead, the 过次页 marker is written to the Back side's 摘要 column.

Hmm, but we do write pageBreakLabel to both sides via `writeMLPageBreakRow`, which writes:
- Back side: `lay.BackStartCol+2` → pageBreakLabel
- Front side: NO summary column in Front! Front has only detail columns.

So `mlHasPageBreakAt` should only check the Back side. The Front side doesn't have a "摘要" column anymore.

Wait, but the `writeMLPageBreakRow` writes to both sides. For the Front side, it writes detail net amounts to each detail column. There's no "摘要" column in the Front side to put a page break label.

So the page break detection only works from the Back side. This is fine since the Back side always has the 摘要 column.

Actually wait, I should also make the 过次页 row write a marker in the Front side's first column or similar. But since Front side has only detail columns (all financial data), there's no good place for a label.

The cleanest approach: make `mlHasPageBreakAt` only check Back side's 摘要 column. Period.

```go
func mlHasPageBreakAt(row []string, lay layout.MLLayout) bool {
    summaryIdx := lay.BindingLeftCols + 2
    return len(row) > summaryIdx && row[summaryIdx] == pageBreakLabel
}
```

For `mlNextDataRow`:
```go
func (wb *Workbook) mlNextDataRow(sheet string) (int, error) {
    lay := mlLayout()
    rows, err := wb.File.GetRows(sheet)
    if err != nil || len(rows) < 3 {
        return lay.DataStartRow + 1, nil  // 空表起始行 = 6（Paper1 Front 占位后）
    }
    // ... rest similar to current, finding last break and computing next data row
}
```

For `lastBreakDetailTotals`:
```go
func (wb *Workbook) lastBreakDetailTotals(sheet string) []mlDetailTotals {
    lay := mlLayout()
    rows, err := wb.File.GetRows(sheet)
    if err != nil {
        return make([]mlDetailTotals, mlMaxDetails)
    }
    for i := len(rows) - 1; i >= 0; i-- {
        if mlHasPageBreakAt(rows[i], lay) {
            result := make([]mlDetailTotals, mlMaxDetails)
            for j := 0; j < mlMaxDetails; j++ {
                colIdx := mlDetailRowIdx(lay, j)
                if colIdx < len(rows[i]) {
                    if v, err := yuanStrToCents(rows[i][colIdx]); err == nil {
                        if v >= 0 {
                            result[j].debit = v
                        } else {
                            result[j].credit = -v
                        }
                    }
                }
            }
            return result
        }
    }
    return make([]mlDetailTotals, mlMaxDetails)
}
```

- [ ] **Step 2: 编译验证**

```bash
go build .
```

- [ ] **Step 3: 提交**

```bash
git add generator/ml_sheet.go
git commit -m "fix(ml): 读取函数适配双面布局坐标"
```

### Task 7: 月结双面化

**Files:**
- Modify: `generator/monthly_close_ml.go`

**Interfaces:**
- Consumes: 新 `mlDetailCol(lay, i)`, 新 `MLLayout`
- Produces: 双面月结行

月结行结构（以「本月合计」为例）：
- Back 侧：摘要＝"本月合计"、借方合计、贷方合计、明细1~4 净额
- Front 侧：明细5~14 净额

- [ ] **Step 1: 新增辅助函数 `writeMLClosingRow` 集中月结行写入**

```go
func (wb *Workbook) writeMLClosingRow(sheet string, row int, label string, debit, credit int64, details []mlDetailTotals, detailsList []string, lay layout.MLLayout) {
    // Back 侧
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), label)
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(debit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(credit))
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), "")
    wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), "")
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
    wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

    // Back 侧明细1~4
    for i := 0; i < 4 && i < len(details); i++ {
        if detailsList[i] != "" {
            net := details[i].debit - details[i].credit
            col := mlDetailCol(lay, i)
            wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
            wb.setMoneyStyle(sheet, row, col)
        }
    }

    // Front 侧明细5~14
    for i := 4; i < len(details); i++ {
        if detailsList[i] != "" {
            net := details[i].debit - details[i].credit
            col := mlDetailCol(lay, i)
            wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
            wb.setMoneyStyle(sheet, row, col)
        }
    }
}
```

- [ ] **Step 2: 重写 `WriteMLMonthClosings`**

将全部月结行通过 `writeMLClosingRow` 写入双面。范围样式适用时延伸到两侧最末列。

```go
// 本月合计
wb.writeMLClosingRow(sheet, row, "本月合计", mtdDebit, mtdCredit, mtdDetails, details, lay)

monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
    Font:   &excelize.Font{Bold: true, Size: 10},
    Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 1}},
})
backStart := mlCellName(lay.BackStartCol, row)
backEnd := mlCellName(lay.BackStartCol+6, row)
wb.File.SetCellStyle(sheet, backStart, backEnd, monthlyStyle)
frontStart := mlCellName(mlDetailCol(lay, 4), row)
frontEnd := mlCellName(mlDetailCol(lay, mlMaxDetails-1), row)
wb.File.SetCellStyle(sheet, frontStart, frontEnd, monthlyStyle)

// 本季合计（季末月）
if isQuarterEnd(wb.Month) {
    wb.writeMLClosingRow(sheet, row, "本季合计", qtDebit, qtCredit, qtDetails, details, lay)
    qtStyle, _ := wb.File.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 10},
    })
    wb.File.SetCellStyle(sheet, backStart, backEnd, qtStyle)
    wb.File.SetCellStyle(sheet, frontStart, frontEnd, qtStyle)
    row++
}

// 本年累计
wb.writeMLClosingRow(sheet, row, "本年累计", cumDebit, cumCredit, ytdDetails, details, lay)
cumStyle, _ := wb.File.NewStyle(&excelize.Style{
    Font:   &excelize.Font{Bold: true, Size: 10},
    Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
})
wb.File.SetCellStyle(sheet, backStart, backEnd, cumStyle)
wb.File.SetCellStyle(sheet, frontStart, frontEnd, cumStyle)
row++

// 期末余额（只在 Back 侧写）
endBalance := initials[general] + mtdDebit - mtdCredit
endDir, endDisp := directionFor(endBalance, 0)
wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
```

`lastDetailCol` 的计算改为用 `mlDetailCol(lay, mlMaxDetails-1)`。

- [ ] **Step 3: 编译验证**

```bash
go build .
```

- [ ] **Step 4: 提交**

```bash
git add generator/monthly_close_ml.go
git commit -m "feat(ml): 月结行双面写入"
```

### Task 8: 打印标记适配

**Files:**
- Modify: `generator/print_mark.go`
- Modify: `generator/ml_sheet.go`（更新 `mlPrintMarkCol` 引用）

- [ ] **Step 1: 更新 `markMLRowForPrint`**

```go
func (wb *Workbook) markMLRowForPrint(sheet string, row int) {
    wb.File.SetCellValue(sheet, cellName(mlPrintMarkCol(), row), "需打印")
}
```

`mlPrintMarkCol()` 已更新为返回 Back 区或 Front 区的最末列+1，位置不变。

- [ ] **Step 2: 更新 `markExistingMLPageForPrint`**

```go
func (wb *Workbook) markExistingMLPageForPrint(sheet string) {
    lay := mlLayout()
    rows, err := wb.File.GetRows(sheet)
    if err != nil {
        return
    }
    pageStart := wb.mlPageStartRow(sheet)
    lastRow := len(rows)
    for r := pageStart; r <= lastRow; r++ {
        if r <= len(rows) && mlHasPageBreakAt(rows[r-1], lay) {
            continue
        }
        wb.markMLRowForPrint(sheet, r)
    }
}
```

- [ ] **Step 3: 提交**

```bash
git add generator/print_mark.go
git commit -m "fix(ml): 打印标记适配新坐标"
```

### Task 9: e2e 测试 + 验证

**Files:**
- Test: `bash scripts/test-e2e.sh`
- Test: `go test ./...`

- [ ] **Step 1: 运行全部单元测试**

```bash
go test ./... -count=1 -timeout 180s
```

如失败，定位并修复。

- [ ] **Step 2: 清除旧输出 + 运行 e2e 脚本**

```bash
rm -rf test/e2e/out
bash scripts/test-e2e.sh --skip-test
```

- [ ] **Step 3: 目视检查生成的 xlsx**

检查以下内容：
1. Paper1 Front 占位（无数据，只有标题+表头在 Front 区）
2. 第一逻辑页：Paper1 Back（基础列+明细1~4）+ Paper2 Front（明细5~14）
3. 过次页贯穿两侧
4. 承前页贯穿两侧
5. 余额连续
6. 月结行（本月合计、本年累计、期末余额）贯穿两侧
7. 末页右侧为空

- [ ] **Step 4: 运行全量 e2e（含测试）**

```bash
bash scripts/test-e2e.sh
```

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "test(ml): e2e 验证滑动窗口布局"
```
