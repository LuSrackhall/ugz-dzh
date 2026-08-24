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
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// colMap 把查看版列号映射到打印版列号。金额列占 split[viewCol] 列，其余 1:1。
type colMap struct {
	start      []int        // start[viewCol] = 打印版起始列（1-indexed）；长度 totalViewCols+1
	split      map[int]int  // viewCol → 该金额列展开的小列数（如 12/11/10）
	amount     map[int]bool // 金额列集合
	totalPrint int          // 打印版总列数
}

// buildColMap 按"金额列展开 split[viewCol]、其余 1"构建列映射。
// split 未覆盖的金额列默认 12。
func buildColMap(totalViewCols int, amountCols []int, split map[int]int) colMap {
	aset := make(map[int]bool, len(amountCols))
	sp := make(map[int]int, len(amountCols))
	for _, c := range amountCols {
		aset[c] = true
		n := 12
		if split != nil {
			if v, ok := split[c]; ok && v > 0 {
				n = v
			}
		}
		sp[c] = n
	}
	start := make([]int, totalViewCols+1)
	pc := 0
	for c := 1; c <= totalViewCols; c++ {
		pc++
		start[c] = pc
		if aset[c] {
			pc += sp[c] - 1
		}
	}
	return colMap{start: start, split: sp, amount: aset, totalPrint: pc}
}

func (m colMap) isAmount(viewCol int) bool { return m.amount[viewCol] }
func (m colMap) startCol(viewCol int) int  { return m.start[viewCol] }
func (m colMap) splitCols(viewCol int) int {
	if n, ok := m.split[viewCol]; ok {
		return n
	}
	return 1
}
func (m colMap) endCol(viewCol int) int {
	s := m.start[viewCol]
	if m.amount[viewCol] {
		return s + m.splitCols(viewCol) - 1
	}
	return s
}

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
	cells         []metaCell
	merges        []metaMerge
	colWidth      []float64     // [1..maxCol] 显式列宽（含 0 宽列）
	zeroWidthCols map[int]bool  // 显式 width=0 的列（GL 分页列 c15 / ML 书口列 c29 等）
	rowHeight     []float64     // [1..maxRow]
	maxRow        int
	maxCol        int
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
	// 列宽：raw XML 解析（GetColWidth 把 width=0 的零宽列返回为默认 9.140625，
	// 无法区分"零宽列"与"未定义列"；零宽列必须保持 0 宽，否则打印版总宽多出默认列宽）
	if rawW, zeroCols, err := rawSheetColWidths(f, sheet); err == nil {
		meta.zeroWidthCols = zeroCols
		for c := 1; c <= maxCol; c++ {
			if w, ok := rawW[c]; ok {
				meta.colWidth[c] = w
			}
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

// rawSheetColWidths 解析查看版 sheet XML 的 <cols>，返回显式列宽表与零宽列集合。
// excelize GetColWidth 把 width=0 的列返回为默认宽 9.140625（无法区分"零宽"与"未定义"），
// 而零宽列（GL c15 分页列 / ML c29 书口列等）必须保持 0 宽，否则打印版总宽多出默认列宽。
func rawSheetColWidths(f *excelize.File, sheet string) (map[int]float64, map[int]bool, error) {
	widths := map[int]float64{}
	zero := map[int]bool{}
	path, err := sheetXMLPath(f, sheet)
	if err != nil {
		return nil, nil, err
	}
	v, ok := f.Pkg.Load(path)
	if !ok {
		return nil, nil, fmt.Errorf("找不到 %s 的 XML %s", sheet, path)
	}
	data, _ := v.([]byte)
	var cols struct {
		Cols []struct {
			Min   int     `xml:"min,attr"`
			Max   int     `xml:"max,attr"`
			Width float64 `xml:"width,attr"`
		} `xml:"cols>col"`
	}
	if err := xml.Unmarshal(data, &cols); err != nil {
		return nil, nil, fmt.Errorf("解析 %s 列宽: %w", sheet, err)
	}
	for _, c := range cols.Cols {
		for i := c.Min; i <= c.Max; i++ {
			widths[i] = c.Width
			if c.Width <= 0 {
				zero[i] = true
			}
		}
	}
	return widths, zero, nil
}

// sheetXMLPath 返回 sheet 对应的 worksheet XML 路径（xl/worksheets/sheetN.xml）。
func sheetXMLPath(f *excelize.File, sheet string) (string, error) {
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			// r:id 的真实命名空间是完整 URI；Go 的 xml 包 tag 语法为 "URI local,attr"（空格分隔），
			// 用 "r:id" 前缀形式无法匹配（RID 解析为空）。
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	wbData, _ := f.Pkg.Load("xl/workbook.xml")
	if wbData == nil {
		return "", fmt.Errorf("无法读取 xl/workbook.xml")
	}
	if err := xml.Unmarshal(wbData.([]byte), &wb); err != nil {
		return "", fmt.Errorf("解析 workbook.xml: %w", err)
	}
	rid := ""
	for _, s := range wb.Sheets {
		if s.Name == sheet {
			rid = s.RID
			break
		}
	}
	if rid == "" {
		return "", fmt.Errorf("workbook.xml 找不到 sheet %q", sheet)
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	relsData, _ := f.Pkg.Load("xl/_rels/workbook.xml.rels")
	if relsData == nil {
		return "", fmt.Errorf("无法读取 xl/_rels/workbook.xml.rels")
	}
	if err := xml.Unmarshal(relsData.([]byte), &rels); err != nil {
		return "", fmt.Errorf("解析 workbook.xml.rels: %w", err)
	}
	for _, r := range rels.Rels {
		if r.ID == rid {
			t := r.Target
			if !strings.HasPrefix(t, "xl/") {
				t = "xl/" + t
			}
			return t, nil
		}
	}
	return "", fmt.Errorf("rels 找不到 rId %s", rid)
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
	splitCols       map[int]int                              // 金额列 → 展开小列数（未覆盖默认 12；ML: 借/贷/余 11、明细 10）
	isLabelRow      func(r int) bool                         // 12 小列标签行（GL: SubHeaderRow+1；ML: 每 block 的 h4）
	isDataRow       func(r int) bool                         // 数据区（数据行+月结/过次页行，拆位生成分组竖线）；表头/标题/下边距区铺"仅继承边界"样式
	breakViewCol    int                                      // 查看版垂直分页符所在列
	applyPageLayout func(f *excelize.File, sheet string, breakPrintCol, lastRow int)
	// amountColPixel 金额小列的目标渲染像素宽（>0 时启用，0 = 按查看版列宽均分/字符守恒）。
	// 折中B（ML）：设 10px（列宽 0.714 字符）→ 借/贷/余区域≈101+9%、明细区域≈100px(-1%)，
	// 缓解 Excel 每列 +5px 像素取整导致的区域膨胀。配合 labelFontSize=5 使标签放得下。
	amountColPixel float64
	// labelFontSize 表头单位行标签字号（pt）。0 = 沿用 printDigitFontSize(7)。
	// 折中B（ML）设 5：列宽 10px 时 7pt 汉字(≈9.3px)贴边，5pt(≈6.7px)才放得下。
	labelFontSize float64
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
	cm := buildColMap(cfg.totalViewCols, cfg.amountCols, cfg.splitCols)

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

	// 列宽：金额列 ÷n（n=该列展开数），其余原宽；零宽列显式设置 0（保持总宽守恒）
	for c := 1; c <= cfg.totalViewCols; c++ {
		w := meta.colWidth[c]
		if w <= 0 {
			// 查看版显式 width=0 的列（GL 分页列 / ML 书口列）：打印版同样设 0，
			// 否则该列未设置宽度 → Excel 用默认 9.14，总宽多出 ~9.14
			if meta.zeroWidthCols[c] {
				pc := cm.startCol(c)
				_ = f.SetColWidth(sheet, colLetter(pc), colLetter(pc), 0)
			}
			continue
		}
		if cm.isAmount(c) {
			n := cm.splitCols(c)
			var sub float64
			if cfg.amountColPixel > 0 {
				// 折中B：按目标像素宽反推列宽（像素 = 字符×7+5 → 字符 = (px-5)/7），
				// 使金额区域渲染宽度接近查看版（缓解 Excel 每列 +5px 像素取整的膨胀）
				sub = (cfg.amountColPixel - 5) / 7
			} else {
				sub = w / float64(n)
			}
			for k := 0; k < n; k++ {
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
	digitCache := make(map[[3]int]int) // (styleID,k,n) → 数据数字格样式（7pt）
	labelCache := make(map[[3]int]int) // (styleID,k,n) → 表头单位行标签格样式（5pt，折中B）
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
	// 该金额列展开的小列数
	n := cm.splitCols(c)
	labels := digitColLabels(n)
	switch {
	case cfg.isLabelRow(r):
		// n 小列标签行（仅对有样式格写入，未渲染侧如 ML Paper1 Back 跳过）。
		// 标签用独立 labelCache（字体 5pt，与数据数字 7pt 区分，缓存互不串用）
		if sid != 0 {
			for k := 0; k < n; k++ {
				pc := cm.startCol(c) + k
				lid := amountSubStyle(f, sid, k, labelCache, n, cfg.labelFontSize)
				_ = f.SetCellValue(sheet, cellAxis(pc, r), labels[k])
				_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), lid)
			}
		}
	case val != "" && !isNumericAmount(val):
		// 文本标签（"借方"/明细名/标题等）：值写首格 + n 列铺样式 + 不在已有合并区则新建合并
		pc := cm.startCol(c)
		_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
		if sid != 0 {
			_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(cm.endCol(c), r), sid)
		}
		if !covered[[2]int{r, c}] {
			extraMerges = append(extraMerges, metaMerge{r1: r, c1: c, r2: r, c2: c})
		}
	case (val != "" && isNumericAmount(val)) || (cfg.isDataRow != nil && cfg.isDataRow(r) && sid != 0):
		// 金额格拆位（数值任意行 / 数据区空值格）：有值写数字、空值=n 空格，均带分组竖线+继承上下/左右边框
		cents := int64(0)
		if val != "" {
			cents, _ = yuanStrToCents(val)
		}
		if cents < 0 {
			cents = -cents
		}
		digits := splitCNY(cents, n)
		for k := 0; k < n; k++ {
			pc := cm.startCol(c) + k
			did := amountSubStyle(f, sid, k, digitCache, n, 0)
			if digits[k] != "" {
				_ = f.SetCellValue(sheet, cellAxis(pc, r), digits[k])
			}
			_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), did)
		}
	case sid != 0:
		// 非数据区空值金额格（表头/标题/下边距区）：仅继承边界样式（无分组竖线、无红双线复制）
		for k := 0; k < n; k++ {
			pc := cm.startCol(c) + k
			eid := amountEdgeStyle(f, sid, k, digitCache, n)
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
// 字体：标签格 5pt（折中B，配合 10px 列宽）、数据数字格 7pt；颜色取原样式（金额格无字体=默认黑；表头标签格=绿色）。
// 边框：上/下继承原样式；左 = (k==0? 原左 : 分组线[k-1])；右 = (k==n-1? 原右 : 分组线[k])。
func amountSubStyle(f *excelize.File, origStyleID, k int, cache map[[3]int]int, n int, labelSize float64) int {
	// key 含 n：同一 styleID 在 GL(12列)/ML(11,10列) 下 k 相同但含义不同（元位置不同），
	// 若不含 n 会缓存串用，导致红细线/加粗线错位、列内部出现不该有的红线。
	// 标签/数字用各自独立 cache（labelCache / digitCache），字体 5pt vs 7pt 互不串用。
	key := [3]int{origStyleID, k, n}
	if id, ok := cache[key]; ok {
		return id
	}
	size := printDigitFontSize
	if labelSize > 0 {
		size = labelSize
	}
	st := &excelize.Style{
		Font:      &excelize.Font{Size: size},
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
	dv := dividerStyles(n)
	if k == 0 {
		if leftB := borderOf(f, origStyleID, "left"); leftB != nil {
			st.Border = append(st.Border, *leftB)
		}
	} else {
		dc, ds := dividerBorder(dv[k-1])
		st.Border = append(st.Border, excelize.Border{Type: "left", Color: dc, Style: ds})
	}
	if k == n-1 {
		if rightB := borderOf(f, origStyleID, "right"); rightB != nil {
			st.Border = append(st.Border, *rightB)
		}
	} else {
		dc, ds := dividerBorder(dv[k])
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
func amountEdgeStyle(f *excelize.File, origStyleID, k int, cache map[[3]int]int, n int) int {
	key := [3]int{origStyleID, k, n}
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
	if k == n-1 {
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
