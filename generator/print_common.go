// Package generator — 打印版位格输出：通用变换工具。
//
// 本文件实现"查看版 → 打印版"的核心几何变换：复制查看版 xlsx 为打印版后，
// 对每个 GL/ML Sheet 做列展开——金额列变 12 小列，非金额列原样复制（同文件
// styleID 共享，DeleteSheet 不裁剪样式表，故 styleID 跨重建有效）。
//
// 设计要点：
//   - 查看版生成代码零改动（仅操作已落盘文件的副本）
//   - 数据来自查看版已计算结果（读单元格值→分→拆位），不重算
//   - 非 InsertCols：用 DeleteSheet + NewSheet + MoveSheet 整表重建
//   - 表头大标题/明细名等竖向合并经"合并区重映射"自动展开为 12 列宽
package generator

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// colMap 把查看版列号映射到打印版列号。金额列占 12 列，其余 1:1。
type colMap struct {
	start      []int        // start[viewCol] = 打印版起始列（1-indexed）；长度 totalViewCols+1
	amount     map[int]bool // 金额列集合
	totalPrint int          // 打印版总列数
}

// buildColMap 按"金额列展开 12、其余 1"构建列映射。
func buildColMap(totalViewCols int, amountCols []int) colMap {
	aset := make(map[int]bool, len(amountCols))
	for _, c := range amountCols {
		aset[c] = true
	}
	start := make([]int, totalViewCols+1)
	pc := 0
	for c := 1; c <= totalViewCols; c++ {
		pc++
		start[c] = pc
		if aset[c] {
			pc += 11
		}
	}
	return colMap{start: start, amount: aset, totalPrint: pc}
}

func (m colMap) isAmount(viewCol int) bool   { return m.amount[viewCol] }
func (m colMap) startCol(viewCol int) int    { return m.start[viewCol] }
func (m colMap) endCol(viewCol int) int      { s := m.start[viewCol]; if m.amount[viewCol] { return s + 11 }; return s }

// metaCell 记录一个查看版单元格的值与样式 ID。
type metaCell struct {
	r, c  int
	val   string
	style int
}

// metaMerge 记录一个合并区（含起止行列）。
type metaMerge struct{ r1, c1, r2, c2 int }

// sheetMeta 记录重建所需的全部查看版 Sheet 元数据。
type sheetMeta struct {
	cells     []metaCell
	merges    []metaMerge
	colWidth  []float64 // [1..maxCol]
	rowHeight []float64 // [1..maxRow]
	maxRow    int
	maxCol    int
}

// readSheetMeta 读取查看版 Sheet 的全部单元格（值+样式）、合并区、列宽、行高。
// 对所有 1..maxCol × 1..maxRow 单元格读取样式，确保空格样式（合并区内部、
// 金额空格的边框）不丢失。
func readSheetMeta(f *excelize.File, sheet string, maxCol int) (*sheetMeta, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 行: %w", sheet, err)
	}
	maxRow := len(rows)
	// excelize GetRows 会裁剪末尾"仅有样式、无值"的行（如末块下边距行——只有红双线
	// 边框没有文字）。这些行的样式/行高不能丢，否则打印版末块下边距行整行缺失
	// （"装订侧红双线未延伸至下边距行"，且只在最后一张逻辑表出现）。
	for r := maxRow + 1; r <= maxRow+5; r++ {
		hasStyle := false
		for c := 1; c <= maxCol; c++ {
			axis, _ := excelize.CoordinatesToCellName(c, r)
			if sid, _ := f.GetCellStyle(sheet, axis); sid != 0 {
				hasStyle = true
				break
			}
		}
		if !hasStyle {
			break
		}
		maxRow = r
	}
	meta := &sheetMeta{
		colWidth:  make([]float64, maxCol+1),
		rowHeight: make([]float64, maxRow+1),
		maxRow:    maxRow,
		maxCol:    maxCol,
	}
	for r := 1; r <= maxRow; r++ {
		var row []string
		if r-1 < len(rows) {
			row = rows[r-1]
		}
		for c := 1; c <= maxCol; c++ {
			val := ""
			if c-1 < len(row) {
				val = row[c-1]
			}
			cell, _ := excelize.CoordinatesToCellName(c, r)
			sid, _ := f.GetCellStyle(sheet, cell)
			meta.cells = append(meta.cells, metaCell{r: r, c: c, val: val, style: sid})
		}
		if h, err := f.GetRowHeight(sheet, r); err == nil {
			meta.rowHeight[r] = h
		}
	}
	for c := 1; c <= maxCol; c++ {
		if w, err := f.GetColWidth(sheet, colLetter(c)); err == nil {
			meta.colWidth[c] = w
		}
	}
	mcs, err := f.GetMergeCells(sheet)
	if err == nil {
		for _, m := range mcs {
			c1, r1, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			c2, r2, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if err1 == nil && err2 == nil {
				meta.merges = append(meta.merges, metaMerge{r1: r1, c1: c1, r2: r2, c2: c2})
			}
		}
	}
	return meta, nil
}

// colLetter 返回列号对应的 Excel 列字母（1→A）。
func colLetter(col int) string {
	name, _ := excelize.ColumnNumberToName(col)
	return name
}

// recreateSheet 删除并重建同名 Sheet（返回原索引以便归位）。
// DeleteSheet 不裁剪工作簿级样式表，故重建后原 styleID 仍有效。
func recreateSheet(f *excelize.File, name string) (origIdx int, err error) {
	origIdx, err = f.GetSheetIndex(name)
	if err != nil {
		return -1, err
	}
	if err = f.DeleteSheet(name); err != nil {
		return -1, err
	}
	if _, err = f.NewSheet(name); err != nil {
		return -1, err
	}
	return origIdx, nil
}

// restoreSheetOrder 把重建后（位于末尾）的 Sheet 移回原索引位置。
// MoveSheet(source, target) 将 source 移到 target 之前。
func restoreSheetOrder(f *excelize.File, name string, origIdx int) {
	curIdx, _ := f.GetSheetIndex(name)
	if curIdx == origIdx {
		return
	}
	list := f.GetSheetList()
	if origIdx >= len(list) {
		origIdx = len(list) - 1
	}
	if origIdx < 0 {
		return
	}
	target := list[origIdx]
	_ = f.MoveSheet(name, target)
}

// printSheetConfig 描述一个 Sheet 的打印变换参数。
type printSheetConfig struct {
	totalViewCols   int                                      // 查看版总列数
	amountCols      []int                                    // 金额列号（1-indexed）
	isLabelRow      func(r int) bool                         // 12 小列标签行（GL: SubHeaderRow+1；ML: 每 block 的 h4）
	isDataRow       func(r int) bool                         // 数据区（数据行+月结/过次页行，拆位生成分组竖线）；表头/标题/下边距区铺"仅继承边界"样式
	breakViewCol    int                                      // 查看版垂直分页符所在列
	applyPageLayout func(f *excelize.File, sheet string, breakPrintCol, lastRow int)
}

// transformSheet 对单个 Sheet 执行列展开变换。
//
// 流程：读取元数据 → 构建列映射 → 删除+重建 Sheet → 写列宽/行高/单元格/合并区 → 重应用页布局。
// 单元格处理（金额列按"内容+样式"统一分类，仅标签行用行号）：
//   - 非金额：原样复制（值 + styleID）到新列位置；空且无样式则跳过
//   - 金额 + 标签行 + 有样式：写 12 小列标签（分组竖线 + 继承原上下边框）
//   - 金额 + 标签行 + 无样式：跳过（如 ML Paper1 未渲染的 Back 侧）
//   - 金额 + money 样式（#,##0.00）：数据行——拆位填 12 格（空值=12 空格，仍带分组竖线）
//   - 金额 + 文本标签（"借方"等）：值写首格 + 12 列铺样式 + 不在已有合并区则新建 12 列合并
//   - 金额 + 空 + 表头样式：12 子格铺样式（合并区内部由重映射合并）
//   - 金额 + 空 + 无样式：跳过
func transformSheet(f *excelize.File, sheet string, cfg printSheetConfig) error {
	meta, err := readSheetMeta(f, sheet, cfg.totalViewCols)
	if err != nil {
		return err
	}
	cm := buildColMap(cfg.totalViewCols, cfg.amountCols)

	// 预计算"被合并区覆盖"的单元格集合，用于判断文本标签是否需新建 12 列合并
	covered := make(map[[2]int]bool, len(meta.merges)*4)
	for _, mg := range meta.merges {
		for r := mg.r1; r <= mg.r2; r++ {
			for c := mg.c1; c <= mg.c2; c++ {
				covered[[2]int{r, c}] = true
			}
		}
	}

	origIdx, err := recreateSheet(f, sheet)
	if err != nil {
		return fmt.Errorf("重建 Sheet %s: %w", sheet, err)
	}

	// 列宽：金额列 ÷12，其余原宽
	for c := 1; c <= cfg.totalViewCols; c++ {
		w := meta.colWidth[c]
		if w <= 0 {
			continue
		}
		if cm.isAmount(c) {
			sub := w / 12.0
			for k := 0; k < 12; k++ {
				pc := cm.startCol(c) + k
				_ = f.SetColWidth(sheet, colLetter(pc), colLetter(pc), sub)
			}
		} else {
			pc := cm.startCol(c)
			_ = f.SetColWidth(sheet, colLetter(pc), colLetter(pc), w)
		}
	}

	// 行高：原样回填（行不变）
	for r := 1; r <= meta.maxRow; r++ {
		if h := meta.rowHeight[r]; h > 0 {
			_ = f.SetRowHeight(sheet, r, h)
		}
	}

	// 单元格
	digitCache := make(map[[2]int]int) // (styleID,k) → 金额小格样式（数据数字格/标签格共用）
	var extraMerges []metaMerge        // 文本标签新建的 12 列合并
	for _, cell := range meta.cells {
		r, c, val, sid := cell.r, cell.c, cell.val, cell.style
		if !cm.isAmount(c) {
			if val == "" && sid == 0 {
				continue
			}
			pc := cm.startCol(c)
			if val != "" {
				_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
			}
			if sid != 0 {
				_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), sid)
			}
			continue
		}
	// 金额列分类（按"内容+样式+行区"判定，仅标签行/数据区用行号）：
	//   - 标签行 + 有样式：写 12 小列标签
	//   - 文本（"借方"/明细名/标题，非数值）+ 非空：值写首格 + 12 列铺样式 + 合并
	//   - 数值（任意行）或 数据区空值格：拆位填 12 格——有值写数字、空值=12 空格，均带分组竖线+继承上下/左右边框
	//   - 非数据区空值 + 有样式（表头/标题/下边距区）：12 子格铺"仅继承边界"样式——top/bottom 继承原样式铺满，
	//     左右仅 k=0 继承原左边框、k=11 继承原右边框，中间格无边框（不生成分组竖线、不复制原红双线）
	//   - 空 + 无样式：跳过
	// 关键：数据区空值金额格（如某分录未涉及的明细列、月结空金额）必须拆位（12 空格+分组竖线），
	// 而非铺原样式（否则原红双线左右边框污染所有小格，造成"双线溢出"）；而表头/标题/下边距区
	// 空值金额格必须"仅继承边界"（既不能拆位生成分组竖线——溢出到标题区，也不能整体铺原样式——
	// 会把原红双线复制 12 条）。
	switch {
	case cfg.isLabelRow(r):
		// 12 小列标签行（仅对有样式格写入，未渲染侧如 ML Paper1 Back 跳过）
		if sid != 0 {
			for k := 0; k < 12; k++ {
				pc := cm.startCol(c) + k
				lid := amountSubStyle(f, sid, k, digitCache)
				_ = f.SetCellValue(sheet, cellAxis(pc, r), digitColLabels[k])
				_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), lid)
			}
		}
	case val != "" && !isNumericAmount(val):
		// 文本标签（"借方"/明细名/标题等）：值写首格 + 12 列铺样式 + 不在已有合并区则新建合并
		pc := cm.startCol(c)
		_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
		if sid != 0 {
			_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(cm.endCol(c), r), sid)
		}
		if !covered[[2]int{r, c}] {
			extraMerges = append(extraMerges, metaMerge{r1: r, c1: c, r2: r, c2: c})
		}
	case (val != "" && isNumericAmount(val)) || (cfg.isDataRow != nil && cfg.isDataRow(r) && sid != 0):
		// 金额格拆位（数值任意行 / 数据区空值格）：有值写数字、空值=12 空格，均带分组竖线+继承上下/左右边框
		cents := int64(0)
		if val != "" {
			cents, _ = yuanStrToCents(val)
		}
		if cents < 0 {
			cents = -cents
		}
		digits := splitCNY(cents)
		for k := 0; k < 12; k++ {
			pc := cm.startCol(c) + k
			did := amountSubStyle(f, sid, k, digitCache)
			if digits[k] != "" {
				_ = f.SetCellValue(sheet, cellAxis(pc, r), digits[k])
			}
			_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), did)
		}
	case sid != 0:
		// 非数据区空值金额格（表头/标题/下边距区）：仅继承边界样式（无分组竖线、无红双线复制）
		for k := 0; k < 12; k++ {
			pc := cm.startCol(c) + k
			eid := amountEdgeStyle(f, sid, k, digitCache)
			_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), eid)
		}
	default:
		// 空 + 无样式：跳过
		}
	}

	// 合并区：重映射现有合并 + 文本标签新建合并
	for _, mg := range meta.merges {
		nc1 := cm.startCol(mg.c1)
		nc2 := cm.endCol(mg.c2)
		_ = f.MergeCell(sheet, cellAxis(nc1, mg.r1), cellAxis(nc2, mg.r2))
	}
	for _, mg := range extraMerges {
		nc1 := cm.startCol(mg.c1)
		nc2 := cm.endCol(mg.c2)
		_ = f.MergeCell(sheet, cellAxis(nc1, mg.r1), cellAxis(nc2, mg.r2))
	}

	// 页布局 + 分页符
	if cfg.applyPageLayout != nil {
		cfg.applyPageLayout(f, sheet, cm.startCol(cfg.breakViewCol), meta.maxRow)
	}

	restoreSheetOrder(f, sheet, origIdx)
	return nil
}

// isNumericAmount 判断字符串是否可解析为金额（分）。
// 数据行金额格恒为数值（含 "0"）；表头文本标签（"借方"/明细名/标题）非数值。
// 空串由调用方先行排除（空串属表头/标题区）。
func isNumericAmount(val string) bool {
	_, err := yuanStrToCents(val)
	return err == nil
}

// cellAxis 返回 (col,row) 的单元格地址。
func cellAxis(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// amountSubStyle 构建并缓存金额小格样式（数据数字格 / 标签格共用，按 (styleID,k) 缓存）。
// 字体：7pt 居中，颜色取原样式（金额格无字体=默认黑；表头标签格=绿色）。
// 边框：上/下继承原样式；左 = (k==0? 原左 : 分组线[k-1])；右 = (k==11? 原右 : 分组线[k])。
func amountSubStyle(f *excelize.File, origStyleID, k int, cache map[[2]int]int) int {
	key := [2]int{origStyleID, k}
	if id, ok := cache[key]; ok {
		return id
	}
	st := &excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	}
	// 取原样式字体颜色（金额格通常无字体即黑色；表头标签格取绿色）
	if def, err := f.GetStyle(origStyleID); err == nil && def.Font != nil {
		if def.Font.Color != "" {
			st.Font.Color = def.Font.Color
		}
	}
	if topB := borderOf(f, origStyleID, "top"); topB != nil {
		st.Border = append(st.Border, *topB)
	}
	if botB := borderOf(f, origStyleID, "bottom"); botB != nil {
		st.Border = append(st.Border, *botB)
	}
	if k == 0 {
		if leftB := borderOf(f, origStyleID, "left"); leftB != nil {
			st.Border = append(st.Border, *leftB)
		}
	} else {
		dc, ds := dividerBorder(dividerStyles[k-1])
		st.Border = append(st.Border, excelize.Border{Type: "left", Color: dc, Style: ds})
	}
	if k == 11 {
		if rightB := borderOf(f, origStyleID, "right"); rightB != nil {
			st.Border = append(st.Border, *rightB)
		}
	} else {
		dc, ds := dividerBorder(dividerStyles[k])
		st.Border = append(st.Border, excelize.Border{Type: "right", Color: dc, Style: ds})
	}
	id, _ := f.NewStyle(st)
	cache[key] = id
	return id
}

// amountEdgeStyle 构建并缓存"非数据区金额格"样式（表头/标题/下边距区，按 (styleID,k) 缓存）。
// 与 amountSubStyle 的区别：中间格（k=1..10）左右**无边框**——不生成分组竖线
// （避免溢出到标题区），也不复制原红双线（避免 12 条双线）。
// 边框：上/下继承原样式（水平线铺满 12 格）；左 = k==0 ? 原左边框 : 无；右 = k==11 ? 原右边框 : 无。
func amountEdgeStyle(f *excelize.File, origStyleID, k int, cache map[[2]int]int) int {
	key := [2]int{origStyleID, k}
	if id, ok := cache[key]; ok {
		return id
	}
	st := &excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	}
	if def, err := f.GetStyle(origStyleID); err == nil && def.Font != nil {
		if def.Font.Color != "" {
			st.Font.Color = def.Font.Color
		}
	}
	if topB := borderOf(f, origStyleID, "top"); topB != nil {
		st.Border = append(st.Border, *topB)
	}
	if botB := borderOf(f, origStyleID, "bottom"); botB != nil {
		st.Border = append(st.Border, *botB)
	}
	if k == 0 {
		if leftB := borderOf(f, origStyleID, "left"); leftB != nil {
			st.Border = append(st.Border, *leftB)
		}
	}
	if k == 11 {
		if rightB := borderOf(f, origStyleID, "right"); rightB != nil {
			st.Border = append(st.Border, *rightB)
		}
	}
	id, _ := f.NewStyle(st)
	cache[key] = id
	return id
}

// applyGLPrintPageLayout 重应用 GL 打印页布局：B5 横向、74% 缩放、0 边距、垂直分页符（已按展开量右移）。
func applyGLPrintPageLayout(f *excelize.File, sheet string, breakPrintCol, lastRow int) {
	paperSize := 13 // B5 (JIS)
	scale := uint(74)
	fw, fh := 0, 0
	fp := false
	_ = f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Orientation: stringPtr("landscape"),
		Size:        &paperSize,
		AdjustTo:    &scale,
		FitToWidth:  &fw,
		FitToHeight: &fh,
	})
	_ = f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
		Top: float64Ptr(0), Bottom: float64Ptr(0), Left: float64Ptr(0), Right: float64Ptr(0),
	})
	_ = f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fp})
	pbCell := colLetter(breakPrintCol)
	_ = f.InsertPageBreak(sheet, pbCell+"1")
}

// applyMLPrintPageLayout 重应用 ML 打印页布局：B5 横向、74%、0 边距、垂直分页符（右移）+
// 水平分页符（每页块 30 行起始前）。与查看版 setMLSheetPageLayout 等价，仅垂直分页列右移。
func applyMLPrintPageLayout(f *excelize.File, sheet string, breakPrintCol, lastRow int) {
	paperSize := 13
	scale := uint(74)
	fw, fh := 0, 0
	fp := false
	_ = f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Orientation: stringPtr("landscape"),
		Size:        &paperSize,
		AdjustTo:    &scale,
		FitToWidth:  &fw,
		FitToHeight: &fh,
	})
	_ = f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
		Top: float64Ptr(0), Bottom: float64Ptr(0), Left: float64Ptr(0), Right: float64Ptr(0),
	})
	_ = f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fp})
	pbCell := colLetter(breakPrintCol)
	_ = f.InsertPageBreak(sheet, pbCell+"1")
	// 水平分页：每页块（上边距+页头+20数据+过次页+下边距 = DataStartRow+pageSize+1+BottomMarginRows 行）
	lay := mlLayout()
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	for start := 1 + blockRows; start <= lastRow; start += blockRows {
		_ = f.InsertPageBreak(sheet, fmt.Sprintf("A%d", start))
	}
}
