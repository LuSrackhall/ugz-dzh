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

// buildColMap 按"金额列/拆分列展开 split[viewCol]、其余 1"构建列映射。
// split 未覆盖的金额列默认 12；splitNA 为非金额列的拆分（如 GL 摘要列 4 格），
// 拆分后文本行"首格+合并回单格"、空值行"仅继承边界"，为标题行局部边框提供列粒度。
func buildColMap(totalViewCols int, amountCols []int, split map[int]int, splitNA map[int]int) colMap {
	aset := make(map[int]bool, len(amountCols))
	sp := make(map[int]int, len(amountCols)+len(splitNA))
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
	for c, n := range splitNA {
		if n > 0 {
			sp[c] = n
		}
	}
	start := make([]int, totalViewCols+1)
	pc := 0
	for c := 1; c <= totalViewCols; c++ {
		pc++
		start[c] = pc
		if n, ok := sp[c]; ok {
			pc += n - 1
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
	if n, ok := m.split[viewCol]; ok {
		return s + n - 1
	}
	return s
}

// metaCell 记录一个查看版单元格的值、类型与样式 ID。
type metaCell struct {
	r, c  int
	val   string
	style int
	isStr bool                   // 单元格是否为字符串类型（金额列中字符串=标签/页码，不能拆位）
	rich  []excelize.RichTextRun // 富文本 runs（重建 sheet 前读取，否则原格已删读不到）
}

// metaMerge 记录一个合并区（含起止行列）。
type metaMerge struct{ r1, c1, r2, c2 int }

// sheetMeta 记录重建所需的全部查看版 Sheet 元数据。
type sheetMeta struct {
	cells         []metaCell
	merges        []metaMerge
	colWidth      []float64    // [1..maxCol] 显式列宽（含 0 宽列）
	zeroWidthCols map[int]bool // 显式 width=0 的列（GL 分页列 c15 / ML 书口列 c29 等）
	rowHeight     []float64    // [1..maxRow]
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
			mc := metaCell{r: r, c: c, val: val, style: sid}
			if val != "" {
				// 单元格类型：数值=CellTypeUnset（无 t 属性），字符串=SharedString/InlineString/Formula。
				// 金额列里的"字符串数值"（如 ML Front 逻辑页码 "1"）必须整段保留不能拆位——
				// 拆位会把 "1" 当 1 元拆成 元1角0分0（显示"100"）。判别用类型而非"长得像数字"。
				if t, err := f.GetCellType(sheet, cell); err == nil {
					mc.isStr = t == excelize.CellTypeSharedString || t == excelize.CellTypeInlineString || t == excelize.CellTypeFormula
				}
				// 富文本必须在重建 sheet 前读取（重建后原格已删）
				if runs, err := f.GetCellRichText(sheet, cell); err == nil && len(runs) > 0 {
					mc.rich = runs
				}
			}
			meta.cells = append(meta.cells, mc)
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
	totalViewCols     int              // 查看版总列数
	amountCols        []int            // 金额列号（1-indexed）
	splitCols         map[int]int      // 金额列 → 展开小列数（未覆盖默认 12；ML: 借/贷/余 11、明细 10）
	splitNA           map[int]int      // 非金额列 → 展开列数（如 GL 摘要列 4 格；文本合并回单格、空值仅继承边界）
	splitNAPixelDelta map[int]float64  // 拆分非金额列的总像素增量（可为负；如 GL 摘要列 -3px）
	dataAlignCols     map[int]string   // 非金额列 → 数据区（isDataRow，不含表头）横向对齐覆盖（如 GL 号列/借或贷列 "center"）
	isLabelRow        func(r int) bool // 12 小列标签行（GL: SubHeaderRow+1；ML: 每 block 的 h4）
	isDataRow         func(r int) bool // 数据区（数据行+月结/过次页行，拆位生成分组竖线）；表头/标题/下边距区铺"仅继承边界"样式
	breakViewCol      int              // 查看版垂直分页符所在列
	applyPageLayout   func(f *excelize.File, sheet string, breakPrintCol, lastRow int)
	// amountColPixel 金额小列的目标渲染像素宽（>0 时启用，0 = 按查看版列宽均分/字符守恒）。
	// 用户定值（ML）= 14：缓解 Excel 每列 +5px 像素取整导致的区域膨胀。
	amountColPixel float64
	// edgePixel (金额列,k) → 该小列的像素宽（覆盖基础宽；k 为组内下标 0..n-1）。
	// 用户定值（ML）：
	//   借/贷/余（n=11）：分列 k=10 → 15px
	//   明细1-4（n=10）：分列 k=9 → 16px
	//   明细5-14（n=10）：千列 k=0(千万位)、百 k=1(百万位)、千 k=4(千位)、分 k=9 → 16px
	edgePixel map[[2]int]float64
	// edgePixelDelta (金额列,k) → 该小列的像素宽**增量**（叠加在基础宽上，可为负）。
	// 与 edgePixel 的区别：edgePixel 是绝对值覆盖，delta 是相对基础宽的加减（px）。
	// 应用：sub = base + delta/7。用户定值（GL）：借/贷/余额 12 小列中，除十亿位 k=0
	// （表头"十"）外，其余 k=1..11 各减 1px。
	edgePixelDelta map[[2]int]float64
	// nonAmountPixel 非金额列 → 目标像素宽（覆盖查看版原始列宽；key=查看版列号）。
	// 用户定值（ML）：借或贷列（查看版 col9）28.1px → 26.1px（减 2px）。
	nonAmountPixel map[int]float64
	// nonAmountPixelDelta 非金额列 → 像素宽**增量**（叠加在查看版原始宽上，可为负）。
	// 应用：w = w + delta/7。用户定值（GL）：月/日/字/号、借或贷、借方旁/贷方旁对号
	// 各 +0.5px（正反面 14 列）。
	nonAmountPixelDelta map[int]float64
	// labelFontSize 表头单位行标签字号（pt）。0 = 沿用 printDigitFontSize(7)。ML 设 6。
	// labelSizeOverride=true 时（print-config.json fonts.labelSize 已配置）labelSize 仅作用于
	// 摘要/借/贷/余额表头（labelCols），其余金额列表头（如 ML 明细区）用 labelFontSizeDefault；
	// 未配置时所有金额列表头用 labelFontSize（与变更前一致）。
	labelFontSize       float64
	labelSizeOverride   bool
	labelFontSizeDefault float64
	// dataFontFamily/dataFontSize 数据区金额数字字体（仅数据格，不含表头标签）。
	// Family 非空时 applyPrintFont 跳过（不统一宋体）。ML：Noteworthy / 6pt。
	// dataFontSize 可被 fonts.digitSize 覆盖（金额区域列数字字号）。
	dataFontFamily string
	dataFontSize   float64
	// digitBold 金额区域列数字加粗（nil=现状不加粗；apply 在金额数据格样式）。
	digitBold *bool
	// labelBold 摘要/借/贷/余额表头加粗（nil=现状加粗；false 时这些标签格不加粗）。
	labelBold *bool
	// labelCols 摘要/借/贷/余额 表头目标列（查看版列号集合；labelBold 应用范围）。
	labelCols map[int]bool
	// postProcess 列展开变换后的额外后处理（如 ML 标题区合并/字体覆盖）。
	// cm 为该 sheet 的列映射；maxRow 为变换后最大行号。
	postProcess func(f *excelize.File, sheet string, cm colMap, maxRow int)
}

// transformSheet 对单个 Sheet 执行列展开变换。
//
// 流程：读取元数据 → 构建列映射 → 删除+重建 Sheet → 写列宽/行高/单元格/合并区 → 重应用页布局。
// 单元格处理（金额列按"值类型+行区+样式"分类，仅标签行/空值用行号）：
//   - 非金额：原样复制（值 + styleID）到新列位置；空且无样式则跳过
//   - 金额 + 标签行 + 有样式：写 n 小列标签（分组竖线 + 继承原上下边框）
//   - 金额 + 标签行 + 无样式：跳过（如 ML Paper1 未渲染的 Back 侧）
//   - 金额 + 数值型（查看版金额=数值存储）：拆位填 n 格（分组竖线 + 继承上下/左右边框）
//   - 金额 + 字符串型（"借方"/明细名/标题/页码"1"）：值写首格 + n 列铺样式 + 不在已有合并区则新建合并
//   - 金额 + 空 + 数据行 + 有样式：拆位填 n 空格（分组竖线 + 继承边框）
//   - 金额 + 空 + 非数据行 + 有样式：n 子格铺"仅继承边界"样式（不生成分组竖线、不复制红双线）
//   - 金额 + 空 + 无样式：跳过
func transformSheet(f *excelize.File, sheet string, cfg printSheetConfig) error {
	meta, err := readSheetMeta(f, sheet, cfg.totalViewCols)
	if err != nil {
		return err
	}
	cm := buildColMap(cfg.totalViewCols, cfg.amountCols, cfg.splitCols, cfg.splitNA)

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
	// 平台补偿：Windows 渲染偏小，列宽值 ×colScale（Mac=1）；GL/ML 可分账本独立系数
	colScale, rowScale := sheetCompensate()
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
			var base float64
			if cfg.amountColPixel > 0 {
				// 按目标像素宽反推列宽（像素 = 字符×7+5 → 字符 = (px-5)/7）
				base = (cfg.amountColPixel - 5) / 7
			} else {
				base = w / float64(n)
			}
			for k := 0; k < n; k++ {
				sub := base
				// 组内任意位置独立像素（按 (金额列,k) 查表，绝对值覆盖）
				if v, ok := cfg.edgePixel[[2]int{c, k}]; ok {
					sub = (v - 5) / 7
				}
				// 像素增量（叠加在基础宽上，可为负；如 GL 除十亿位外各减 1px）
				if d, ok := cfg.edgePixelDelta[[2]int{c, k}]; ok {
					sub = base + d/7
				}
				pc := cm.startCol(c) + k
				_ = f.SetColWidth(sheet, colLetter(pc), colLetter(pc), sub*colScale)
			}
		} else {
			// 拆分非金额列（如 GL 摘要列 4 格）：n 子列按"原字符数 ÷ n"等分——
			// 字符数显示与未拆列时一致（每列仍自带 5px 内边距，总像素比原列略宽）。
			// 总像素增量 splitNAPixelDelta 施加在真实总量（w×7+5n，含 n 份内边距）上：
			// subW = (w×7 + delta) / (7n)，delta=0 时即 w/n（字符等分）。
			if n := cm.splitCols(c); n > 1 {
				subW := (w*7 + cfg.splitNAPixelDelta[c]) / (7 * float64(n))
				for k := 0; k < n; k++ {
					_ = f.SetColWidth(sheet, colLetter(cm.startCol(c)+k), colLetter(cm.startCol(c)+k), subW*colScale)
				}
				continue
			}
			pc := cm.startCol(c)
			// 非金额列特例：命中 nonAmountPixel 时按目标像素覆盖查看版原始宽度
			if px, ok := cfg.nonAmountPixel[c]; ok {
				w = (px - 5) / 7
			}
			// 非金额列像素增量（叠加在查看版/覆盖宽上，可为负；如 GL 月日字号等 +0.5px）
			if d, ok := cfg.nonAmountPixelDelta[c]; ok {
				w = w + d/7
			}
			_ = f.SetColWidth(sheet, colLetter(pc), colLetter(pc), w*colScale)
		}
	}

	// 行高：原样回填（行不变）；平台补偿：Windows 渲染行高偏小，×rowScale（Mac=1）
	for r := 1; r <= meta.maxRow; r++ {
		if h := meta.rowHeight[r]; h > 0 {
			_ = f.SetRowHeight(sheet, r, h*rowScale)
		}
	}

	// 单元格
	digitCache := make(map[[5]int]int) // (styleID,k,n,red,bold) → 数据数字格样式（7pt）
	labelCache := make(map[[5]int]int) // (styleID,k,n,red,bold) → 表头单位行标签格样式（5pt，折中B）
	alignCache := make(map[string]int) // "sid|对齐" → 数据区横向对齐覆盖样式
	var extraMerges []metaMerge        // 文本标签新建的 12 列合并
	for _, cell := range meta.cells {
		r, c, val, sid := cell.r, cell.c, cell.val, cell.style
		if !cm.isAmount(c) {
			if val == "" && sid == 0 {
				continue
			}
			n := cm.splitCols(c)
			if n > 1 {
				// 拆分非金额列（GL 摘要列 4 格，为标题行"双线底边框从摘要列 3/4 处开始"提供列粒度）：
				//   - 文本（摘要内容/表头"摘要"标签）：值写首格 + 整段铺原样式 + 合并 n 格（外观=单格，内部边框被合并隐藏）
				//   - 空值 + 有样式（数据行空摘要）：n 子格铺"仅继承边界"（top/bottom 铺满、k=0 继承左、k=n-1 继承右），不合并
				pc := cm.startCol(c)
				if val != "" {
					// 富文本优先（与 1:1 非金额分支一致）
					if len(cell.rich) > 0 {
						_ = f.SetCellRichText(sheet, cellAxis(pc, r), cell.rich)
					} else {
						_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
					}
				}
				if sid != 0 {
					if val != "" {
						_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc+n-1, r), sid)
						if !covered[[2]int{r, c}] {
							extraMerges = append(extraMerges, metaMerge{r1: r, c1: c, r2: r, c2: c})
						}
					} else {
						for k := 0; k < n; k++ {
							eid := amountEdgeStyle(f, sid, k, digitCache, n)
							_ = f.SetCellStyle(sheet, cellAxis(pc+k, r), cellAxis(pc+k, r), eid)
						}
					}
				}
				continue
			}
			pc := cm.startCol(c)
			if val != "" {
				// 富文本优先：查看版年份"2026年"等是富文本（分段字体），SetCellValue 会
				// 用纯文本覆盖导致颜色丢失。meta.cells 已在重建前缓存 runs。
				if len(cell.rich) > 0 {
					_ = f.SetCellRichText(sheet, cellAxis(pc, r), cell.rich)
				} else {
					_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
				}
			}
			if sid != 0 {
				nid := sid
				if ha, ok := cfg.dataAlignCols[c]; ok && cfg.isDataRow != nil && cfg.isDataRow(r) {
					// 数据区横向对齐覆盖（如 GL 号列/借或贷列 → 居中，不含表头）
					nid = dataAlignStyle(f, sid, ha, alignCache)
				}
				_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), nid)
			}
			continue
		}
		// 金额列分类（按"值类型+行区+样式"判定，仅标签行/空值用行号）：
		//   - 标签行 + 有样式：写 n 小列标签
		//   - 字符串型 + 非空（"借方"/明细名/标题/页码"1"）：值写首格 + n 列铺样式 + 合并
		//   - 数值型（查看版金额=数值存储，任意行）或 数据区空值格：拆位填 n 格——有值写数字、
		//     空值=n 空格，均带分组竖线+继承上下/左右边框
		//   - 非数据区空值 + 有样式（表头/标题/下边距区）：n 子格铺"仅继承边界"样式——top/bottom 继承
		//     原样式铺满，左右仅 k=0 继承原左边框、k=n-1 继承原右边框，中间格无边框（不生成分组竖线、
		//     不复制原红双线）
		//   - 空 + 无样式：跳过
		// 关键1：金额列里"字符串型数值"（ML Front 逻辑页码 "1"/"2"）绝不能拆位——拆位会把 "1" 当
		// 1 元拆成 元1角0分0（显示"100"）。判别用单元格类型（数值=CellTypeUnset、字符串=SharedString），
		// 与行区/金额格式（GL=General、ML=#,##0.00）无关，GL/ML 通吃。
		// 关键2：数据区空值金额格（如某分录未涉及的明细列、月结空金额）必须拆位（n 空格+分组竖线），
		// 而非铺原样式（否则原红双线左右边框污染所有小格，造成"双线溢出"）；而表头/标题/下边距区
		// 空值金额格必须"仅继承边界"（既不能拆位生成分组竖线——溢出到标题区，也不能整体铺原样式——
		// 会把原红双线复制 n 条）。
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
					// labelSize 仅作用于 摘要/借/贷/余额 表头（labelCols）；未配置时全量用
					// cfg.labelFontSize（与变更前一致：GL 7 / ML 6）；配置过但非目标列 → 原默认
					ls := cfg.labelFontSize
					if cfg.labelSizeOverride && !cfg.labelCols[c] {
						ls = cfg.labelFontSizeDefault
					}
					lid := amountSubStyle(f, sid, k, labelCache, n, ls, 0, "", false, false)
					_ = f.SetCellValue(sheet, cellAxis(pc, r), labels[k])
					_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(pc, r), lid)
				}
			}
		case val != "" && cell.isStr:
			// 字符串型（"借方"/明细名/标题/页码"1"等）：值写首格 + n 列铺样式 + 不在已有合并区则新建合并
			// ⚠️ 字符串数值（Front 逻辑页码"1"）不能拆位——拆位会把页码"1"当 1 元拆成"100"
			pc := cm.startCol(c)
			_ = f.SetCellValue(sheet, cellAxis(pc, r), val)
			if sid != 0 {
				_ = f.SetCellStyle(sheet, cellAxis(pc, r), cellAxis(cm.endCol(c), r), sid)
			}
			if !covered[[2]int{r, c}] {
				extraMerges = append(extraMerges, metaMerge{r1: r, c1: c, r2: r, c2: c})
			}
		case (val != "" && !cell.isStr) || (val == "" && cfg.isDataRow != nil && cfg.isDataRow(r) && sid != 0):
			// 金额格拆位（数值=金额 任意行 / 数据区空值格）：有值写数字、空值=n 空格，均带分组竖线+继承上下/左右边框
			cents := int64(0)
			if val != "" {
				cents, _ = yuanStrToCents(val)
			}
			red := cents < 0 // 红字（负数）：数字用红色字体标记（审计二审 H2，手工账红笔惯例）
			if cents < 0 {
				cents = -cents
			}
			digits := splitCNY(cents, n)
			for k := 0; k < n; k++ {
				pc := cm.startCol(c) + k
				did := amountSubStyle(f, sid, k, digitCache, n, 0, cfg.dataFontSize, cfg.dataFontFamily, red, cfg.digitBold != nil && *cfg.digitBold)
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
	// 注意：跳过单格"合并区"（r1==r2 && c1==c2）——excelize 允许这种无意义合并，
	// 但重映射会把金额列的单格合并展开为 n 格大合并，合并时左上角之外的值被清空，
	// 导致拆位写在元位小格的内容丢失（如 Front 侧逻辑页码数字）。
	for _, mg := range meta.merges {
		if mg.r1 == mg.r2 && mg.c1 == mg.c2 {
			continue
		}
		nc1 := cm.startCol(mg.c1)
		nc2 := cm.endCol(mg.c2)
		_ = f.MergeCell(sheet, cellAxis(nc1, mg.r1), cellAxis(nc2, mg.r2))
	}
	for _, mg := range extraMerges {
		nc1 := cm.startCol(mg.c1)
		nc2 := cm.endCol(mg.c2)
		_ = f.MergeCell(sheet, cellAxis(nc1, mg.r1), cellAxis(nc2, mg.r2))
	}

	// 额外后处理（ML 标题区合并/字体覆盖等）
	if cfg.postProcess != nil {
		cfg.postProcess(f, sheet, cm, meta.maxRow)
	}

	// 页布局 + 分页符
	if cfg.applyPageLayout != nil {
		cfg.applyPageLayout(f, sheet, cm.startCol(cfg.breakViewCol), meta.maxRow)
	}

	// 字体统一（宋体+加粗；labelBold=false 时摘要/借/贷/余额表头标签不加粗）。
	// 放在最后：postProcess 创建的标题样式带显式 Family（applyPrintFont 跳过），
	// 金额数字格带显式 Family/加粗（跳过），其余区域统一宋体。
	applyPrintFont(f, sheet, cm, cfg)

	restoreSheetOrder(f, sheet, origIdx)
	return nil
}

// cellAxis 返回 (col,row) 的单元格地址。
func cellAxis(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// amountSubStyle 构建并缓存金额小格样式（数据数字格 / 标签格共用，按 (styleID,k) 缓存）。
// 字体：标签格 labelSize（ML 5pt）、数据数字格 dataSize（ML 6pt，dataFamily=Noteworthy；
// GL 0 → printDigitFontSize 7pt 由 applyPrintFont 统一宋体）；颜色取原样式（金额格无字体=默认黑；表头标签格=绿色）。
// 边框：上/下继承原样式；左 = (k==0? 原左 : 分组线[k-1])；右 = (k==n-1? 原右 : 分组线[k])。
// amountSubStyle 构建并缓存"金额小格"样式（含分组竖线/继承边框；red 时数字红色=红字标记）。
func amountSubStyle(f *excelize.File, origStyleID, k int, cache map[[5]int]int, n int, labelSize, dataSize float64, dataFamily string, red, digitBold bool) int {
	// key 含 n 与 red：同一 styleID 在 GL(12列)/ML(11,10列) 下 k 相同但含义不同（元位置不同），
	// 红/黑字体也必须区分（审计二审 H2：红字打印红色标记，防缓存串用）。
	// 标签/数字用各自独立 cache（labelCache / digitCache），字体 5pt vs 7pt 互不串用。
	redFlag := 0
	if red {
		redFlag = 1
	}
		boldFlag := 0
	if digitBold {
		boldFlag = 1
	}
	key := [5]int{origStyleID, k, n, redFlag, boldFlag}
	if id, ok := cache[key]; ok {
		return id
	}
	size := printDigitFontSize
	if labelSize > 0 {
		size = labelSize
	} else if dataSize > 0 {
		size = dataSize
	}
	st := &excelize.Style{
		Font:      &excelize.Font{Size: size},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	}
	// 数据数字格（labelSize==0）可指定字体族（如 ML Noteworthy）与加粗（fonts.digitBold）；
	// 标签格恒用默认（宋体由 applyPrintFont 统一，加粗同样由 applyPrintFont 按 labelBold 处理）
	if labelSize == 0 {
		if dataFamily != "" {
			st.Font.Family = dataFamily
		}
		if digitBold {
			st.Font.Bold = true
		}
	}
	// 取原样式字体颜色（金额格通常无字体即黑色；表头标签格取绿色）；红字强制 #CC0000
	if red {
		st.Font.Color = "#CC0000"
	} else if def, err := f.GetStyle(origStyleID); err == nil && def.Font != nil {
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
func amountEdgeStyle(f *excelize.File, origStyleID, k int, cache map[[5]int]int, n int) int {
	key := [5]int{origStyleID, k, n, 0, 0}
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

// dataAlignStyle 返回 origStyleID 的"横向对齐覆盖"变体：仅改 Alignment.Horizontal，
// 保留字体/边框/竖对齐等其余属性；按 (sid,ha) 缓存。
func dataAlignStyle(f *excelize.File, origStyleID int, ha string, cache map[string]int) int {
	key := fmt.Sprintf("%d|%s", origStyleID, ha)
	if id, ok := cache[key]; ok {
		return id
	}
	st, err := f.GetStyle(origStyleID)
	if err != nil {
		return origStyleID
	}
	if st.Alignment == nil {
		st.Alignment = &excelize.Alignment{}
	}
	st.Alignment.Horizontal = ha
	id, err := f.NewStyle(st)
	if err != nil {
		return origStyleID
	}
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
