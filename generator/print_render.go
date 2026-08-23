// Package generator — 打印版位格输出：复制查看版 + 变换金额列。
//
// 每月生成流程：
//   1. GenerateWorkbook 产出查看版 xlsx（不变）
//   2. 复制查看版到 print/ 子目录（若上月 print 文件存在则先复制上月，实现跨月累积）
//   3. 打开 print 副本，对每个账页 Sheet 的金额列执行位格化变换
//   4. 保存 print 副本
//
// 非金额内容（标题/表头/日期/摘要/方向/边框/合并区/行高/列宽）与查看版 100% 一致。
// 金额值从查看版读取（权威源），拆位写入 12 小列。
package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// RenderPrintVersion 从查看版 xlsx 生成打印版。
// 每月从当月查看版复制（未展开状态），变换金额列后保存。
// 累积性由查看版本身保证（查看版每月复制上月再追加）。
func RenderPrintVersion(viewPath, printDir, month string) error {
	printPath := filepath.Join(printDir, month+".xlsx")

	if err := os.MkdirAll(printDir, 0o755); err != nil {
		return fmt.Errorf("创建 print 目录: %w", err)
	}
	// 始终从查看版复制（保证未展开状态）
	if err := copyFile(viewPath, printPath); err != nil {
		return fmt.Errorf("复制查看版: %w", err)
	}

	// 打开 print 副本进行变换
	f, err := excelize.OpenFile(printPath)
	if err != nil {
		return fmt.Errorf("打开 print 副本: %w", err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		switch {
		case strings.HasPrefix(sheet, sheetPrefixGL):
			if err := transformGLMoneyCols(f, sheet); err != nil {
				return fmt.Errorf("变换 GL %s: %w", sheet, err)
			}
		case strings.HasPrefix(sheet, sheetPrefixML):
			if err := transformMLMoneyCols(f, sheet); err != nil {
				return fmt.Errorf("变换 ML %s: %w", sheet, err)
			}
		}
	}

	return f.SaveAs(printPath)
}

// transformGLMoneyCols 将 GL Sheet 的金额列展开为 12 小列。
// 处理 Front 和 Back 两侧，每侧 3 个金额列（借/贷/余额）。
// 注意：GL 是多页结构（过次页后每页有自己的表头），表头处理需逐页执行。
func transformGLMoneyCols(f *excelize.File, sheet string) error {
	lay := glLayout()
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil
	}

	// 展开前快照标题区（row1 ~ HeaderRow）的全部非空单元格值。
	// excelize InsertCols 平移合并区内的值时存在丢失缺陷（实测页码"1"
	// 从 K2 平移后消失），因此展开前记录、展开后按平移坐标回填。
	type titleCell struct {
		row, col int
		val      string
	}
	var titleSnap []titleCell
	for r := 1; r < lay.HeaderRow && r <= len(rows); r++ {
		for c := 1; c <= lay.TotalCols+40; c++ { // 上限覆盖平移余量
			if c-1 >= len(rows[r-1]) {
				break
			}
			v := strings.TrimSpace(rows[r-1][c-1])
			if v != "" {
				titleSnap = append(titleSnap, titleCell{row: r, col: c, val: v})
			}
		}
	}

	// 收集全部金额列（Front + Back），去重后从右往左展开
	cols := make([]int, 0, 6)
	seen := make(map[int]bool)
	for _, side := range []int{lay.BackStartCol, lay.FrontStartCol} {
		for _, off := range glPrintMoneyOffsets {
			c := side + off
			if !seen[c] {
				seen[c] = true
				cols = append(cols, c)
			}
		}
	}
	sortIntsDesc(cols)
	for _, col := range cols {
		if err := expandMoneyColumn(f, sheet, col, len(rows)); err != nil {
			return err
		}
	}

	if err := updateGLHeadersForExpanded(f, sheet, cols); err != nil {
		return err
	}

	// 回填标题区丢失的值：按 shifted 坐标定位新位置，仅当目标为空时写入
	shifted := func(col int) int {
		for _, at := range cols {
			if col > at {
				col += 11
			}
		}
		return col
	}
	for _, tc := range titleSnap {
		nc := shifted(tc.col)
		cell := cellName(nc, tc.row)
		cur, _ := f.GetCellValue(sheet, cell)
		if strings.TrimSpace(cur) == "" {
			f.SetCellValue(sheet, cell, tc.val)
		}
	}
	return nil
}

// updateGLHeadersForExpanded 更新 GL 每一页的表头（GL 多页结构，每页过次页后有自己的两行表头）。
//
// 关键：excelize MergeCell 在与既有纵向合并区（✓列/摘要/借或贷 跨 HeaderRow:SubHeaderRow）
// 交叠时会自动扩展矩形（复现：请求 H5:S5 得到 H5:S6）。因此流程为：
//  1. Unmerge 表头两行内所有与金额展开区交叠的既有合并区（记录其内容与样式）
//  2. 金额大标题做单行横向合并；小列标签写入 SubHeaderRow
//  3. 将被拆掉的纵向合并区恢复到平移后的新位置
func updateGLHeadersForExpanded(f *excelize.File, sheet string, expandedCols []int) error {
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return nil
	}
	lay := glLayout()

	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize, Color: "006100"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	colSet := make(map[int]bool)
	for _, c := range expandedCols {
		colSet[c] = true
	}

	// 展开位移映射：原始列号 → 插列后实际列号
	shifted := func(col int) int {
		for _, at := range expandedCols {
			if col > at {
				col += 11
			}
		}
		return col
	}

	// 扫描每一行：找到「月」所在行 = SubHeaderRow，其上一行为 HeaderRow
	for i, row := range rows {
		r := i + 1
		for _, base := range []int{lay.FrontStartCol, lay.BackStartCol} {
			idx := base - 1 // GetRows 索引
			if idx >= len(row) || strings.TrimSpace(row[idx]) != "月" {
				continue
			}
			headerRow := r - 1
			subRow := r
			if headerRow < 1 {
				continue
			}

			// 该页该侧的金额展开区范围（原始坐标）
			moneyCols := make(map[int]bool)
			for _, off := range glPrintMoneyOffsets {
				if colSet[base+off] {
					moneyCols[base+off] = true
				}
			}
			if len(moneyCols) == 0 {
				continue
			}

			// Step 1（稳健版）: 拆除本页本侧表头两行内的【全部】纵向合并区，
			// 不做影响范围预判——excelize 的 mergeOverlapCells 会在任何
			// MergeCell/GetMergeCells 调用时就地重组交叠区，逐个判断不可靠。
			// 记录内容和列号，稍后按平移坐标重建。
			type vMerge struct {
				startCol int
				val      string
			}
			var toRestore []vMerge
			ms, _ := f.GetMergeCells(sheet)
			for _, m := range ms {
				sc, sr, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
				ec, er, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
				if err1 != nil || err2 != nil {
					continue
				}
				if sr == headerRow && er == subRow && sc >= base && sc <= base+glColCount-1 && sc != ec {
					// 横向合并区（如年份 C5:D5、凭证 E5:F5）不在纵向重建范围，
					// 但若与金额区交叠也要拆——本侧 glColCount 内 sc!=ec 的只有
					// 年份/凭证（位于金额区左侧，不交叠），保留。
					continue
				}
				if sr == headerRow && er == subRow && sc >= base && sc <= base+glColCount-1 {
					toRestore = append(toRestore, vMerge{startCol: sc, val: m.GetCellValue()})
					f.UnmergeCell(sheet, cellName(sc, sr), cellName(ec, er))
				}
			}

			// Step 2: 金额大标题单行横向合并 + 小列标签
			// 注意：必须用【平移后】的首格列号——原始列号在展开后已指向错误位置
			for _, off := range glPrintMoneyOffsets {
				col := shifted(base + off)
				f.MergeCell(sheet, cellName(col, headerRow), cellName(col+11, headerRow))
				writeDigitLabelsAt(f, sheet, col, subRow, labelStyle)
			}

			// Step 3: 恢复纵向合并区到平移后的位置
			for _, vm := range toRestore {
				nc := shifted(vm.startCol)
				top := cellName(nc, headerRow)
				bot := cellName(nc, subRow)
				f.MergeCell(sheet, top, bot)
				if strings.TrimSpace(vm.val) != "" {
					f.SetCellValue(sheet, top, vm.val)
				}
			}
		}
	}
	return repairGLTitleMerges(f, sheet, lay, expandedCols)
}

// repairGLTitleMerges 修复 GL 标题区（row1~DataStartRow-1）的合并区：
// 拆除后按【当前坐标】原样重建。注意 GetMergeCells 返回的已是插列平移后的
// 坐标——不能再做 shifted 叠加，否则双重位移。
func repairGLTitleMerges(f *excelize.File, sheet string, lay layout.GLLayout, expandedCols []int) error {
	rows, _ := f.GetRows(sheet)
	titleEnd := lay.HeaderRow // 标题区 = row1 ~ HeaderRow（表头两行之前）
	if titleEnd > len(rows) {
		titleEnd = len(rows)
	}
	type geoMerge struct {
		c1, r1, c2, r2 int
		val            string
	}
	var olds []geoMerge
	ms, _ := f.GetMergeCells(sheet)
	for _, m := range ms {
		sc, sr, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		ec, er, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 != nil || err2 != nil {
			continue
		}
		if sr >= 1 && er <= titleEnd {
			olds = append(olds, geoMerge{c1: sc, r1: sr, c2: ec, r2: er, val: m.GetCellValue()})
			f.UnmergeCell(sheet, cellName(sc, sr), cellName(ec, er))
		}
	}
	for _, g := range olds {
		top := cellName(g.c1, g.r1)
		bot := cellName(g.c2, g.r2)
		f.MergeCell(sheet, top, bot)
		if strings.TrimSpace(g.val) != "" {
			f.SetCellValue(sheet, top, g.val)
		}
	}
	return nil
}

// truncateVal 截断字符串便于日志显示。
func truncateVal(s string) string {
	r := []rune(s)
	if len(r) > 10 {
		return string(r[:10]) + "…"
	}
	return s
}

// expandMoneyColumn 将单个金额列展开为 12 小列。
// 在原列右侧插入 11 列，读取原列值，拆位写入 12 小格。
// 边框派生：每行读取原格完整样式 → 首格继承 left+top+bottom、
// 末格继承 right+top+bottom、中间格 top+bottom+分组竖线。
// 字号统一缩小；金额数字格式（#,##0.00）清除。
func expandMoneyColumn(f *excelize.File, sheet string, col, lastRow int) error {
	// 展开前快照：每行的原格样式 ID 与值
	type rowSnap struct {
		styleID int
		val     string
	}
	snaps := make([]rowSnap, lastRow+1)
	for r := 1; r <= lastRow; r++ {
		cell := cellName(col, r)
		sid, _ := f.GetCellStyle(sheet, cell)
		v, _ := f.GetCellValue(sheet, cell)
		snaps[r] = rowSnap{styleID: sid, val: strings.TrimSpace(v)}
	}

	// 读取原列宽
	colLetter, _ := excelize.ColumnNumberToName(col)
	origWidth, _ := f.GetColWidth(sheet, colLetter)

	// 在原列右侧插入 11 列
	insertCol, _ := excelize.ColumnNumberToName(col + 1)
	if err := f.InsertCols(sheet, insertCol, 11); err != nil {
		return fmt.Errorf("插入列 %s: %w", insertCol, err)
	}

	// 设置 12 小列宽
	unitWidth := origWidth / 12.0
	for k := 0; k < 12; k++ {
		cl, _ := excelize.ColumnNumberToName(col + k)
		f.SetColWidth(sheet, cl, cl, unitWidth)
	}

	// 为每一行派生 12 小格样式并写入拆位数据
	for r := 1; r <= lastRow; r++ {
		snap := snaps[r]

		// 解析原格样式
		var orig *excelize.Style
		if snap.styleID > 0 {
			if st, err := f.GetStyle(snap.styleID); err == nil {
				orig = st
			}
		}
		borderOf := func(side string) *excelize.Border {
			if orig == nil {
				return nil
			}
			for i, b := range orig.Border {
				if b.Type == side {
					t := orig.Border[i]
					return &t
				}
			}
			return nil
		}
		leftB, rightB := borderOf("left"), borderOf("right")
		topB, bottomB := borderOf("top"), borderOf("bottom")

		// 分组竖线只应出现在表格结构内（表头标签行 + 数据区）。
		// 判据：原格拥有表格网格线型（细1/粗2/双6）的 top 或 bottom 边框。
		// 标题区的装饰性下划线是虚线（Style 4），不算表格结构；
		// 完全无边框的空白行也不算。
		gridBorder := func(b *excelize.Border) bool {
			return b != nil && (b.Style == 1 || b.Style == 2 || b.Style == 6)
		}
		inTable := gridBorder(topB) || gridBorder(bottomB)

		// 原字体：保留颜色，字号改小；清除金额数字格式
		fontColor := ""
		bold := false
		if orig != nil && orig.Font != nil {
			fontColor = orig.Font.Color
			bold = orig.Font.Bold
		}

		isNumeric := false
		var digits [12]string
		if snap.val != "" {
			if cents, err := yuanStrToCents(snap.val); err == nil && snap.val != "0" {
				digits = splitCNY(cents)
				isNumeric = true
			}
		}
		_ = isNumeric

		for k := 0; k < 12; k++ {
			var st *excelize.Style
			if !inTable {
				// 表格外：仅缩字号+居中，不写任何边框、不动内容位置
				st = &excelize.Style{
					Font:      &excelize.Font{Size: printDigitFontSize, Color: fontColor, Bold: bold},
					Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
				}
			} else {
				st = &excelize.Style{
					Font:      &excelize.Font{Size: printDigitFontSize, Color: fontColor, Bold: bold},
					Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
				}
				// 左边框：首格继承原左边框；其余格左边框 = 前一分隔线
				if k == 0 {
					if leftB != nil {
						st.Border = append(st.Border, *leftB)
					}
				} else {
					dc, ds := dividerBorder(dividerStyles[k-1])
					st.Border = append(st.Border, excelize.Border{Type: "left", Color: dc, Style: ds})
				}
				// 右边框：末格继承原右边框；其余格右边框 = 自身分隔线
				if k == 11 {
					if rightB != nil {
						st.Border = append(st.Border, *rightB)
					}
				} else {
					dc, ds := dividerBorder(dividerStyles[k])
					st.Border = append(st.Border, excelize.Border{Type: "right", Color: dc, Style: ds})
				}
				// 上下边框：全部小格继承原格语义（每5行加粗、过次页红双线底边等）
				if topB != nil {
					st.Border = append(st.Border, *topB)
				}
				if bottomB != nil {
					st.Border = append(st.Border, *bottomB)
				}
			}

			sid, _ := f.NewStyle(st)
			cell := cellName(col+k, r)
			f.SetCellStyle(sheet, cell, cell, sid)

			if isNumeric {
				f.SetCellValue(sheet, cell, digits[k])
			} else if k > 0 && snap.val == "" {
				// 空格：非首格清空内容
				f.SetCellValue(sheet, cell, "")
			}
		}
	}

	return nil
}

// transformMLMoneyCols 将 ML Sheet 的金额列展开为 12 小列。
// ML 布局按块（每块 blockRows 行）：Paper1 首块只有 Front 侧明细5-14；
// 其余块 Back 侧（借/贷/余额+明细1-4）与 Front 侧都有。
// mlDetailCol 返回绝对 Excel 列号，直接使用。
func transformMLMoneyCols(f *excelize.File, sheet string) error {
	lay := mlLayout()
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil
	}

	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	_ = blockRows

	// 逐块处理，从最后一块往前（右侧块的插列不影响左侧块的原始坐标——
	// 但插列是全列生效的：在 col X 右侧插列影响所有行的该列之后内容。
	// 因此按「从右往左」的全局列序处理一次即可，行无关。）

	// 收集全部需要展开的绝对列号（去重）
	cols := make([]int, 0, 17)
	seen := make(map[int]bool)
	addCol := func(c int) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	// Front 侧明细5-14（所有块都有）
	for i := 4; i < mlMaxDetails; i++ {
		addCol(mlDetailCol(lay, i))
	}
	// Back 侧借/贷/余额 + 明细1-4（Paper1 首块没有 Back 表结构，
	// 但插列是列级操作——若首块 Back 区为空，展开这些列对空单元格无副作用，
	// 且后续块需要。统一处理。）
	for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
		addCol(lay.BackStartCol + off)
	}
	for i := 0; i < 4; i++ {
		addCol(mlDetailCol(lay, i))
	}

	// 从右往左展开
	sortIntsDesc(cols)
	for _, col := range cols {
		if err := expandMoneyColumn(f, sheet, col, len(rows)); err != nil {
			return err
		}
	}

	if err := updateMLHeadersForExpanded(f, sheet, lay, cols); err != nil {
		return err
	}
	// ML 标题区（每块的 row1~hStart-1：分第 n 页(左)/(右)、科目名等）：
	// 1) 按当前坐标拆掉重建被插列破坏的合并区
	if err := repairMLTitleMerges(f, sheet, lay, rows); err != nil {
		return err
	}
	// 2) 全格值回填——excelize InsertCols 会丢失部分标题单元格的值（实测
	//    「页(左)」F32、「分第(右)」Z32 等），展开前快照全部非空标题格，
	//    按 shifted 坐标回填到空位。
	shifted := func(col int) int { return mlShiftedCol(col, cols) }
	for start := 1; start <= len(rows); start += blockRows {
		titleEnd := start + 3
		for r := start; r <= titleEnd && r <= len(rows); r++ {
			for c := 1; c <= len(rows[r-1]); c++ {
				v := strings.TrimSpace(rows[r-1][c-1])
				if v == "" {
					continue
				}
				cell := cellName(shifted(c), r)
				cur, _ := f.GetCellValue(sheet, cell)
				if strings.TrimSpace(cur) == "" {
					f.SetCellValue(sheet, cell, v)
				}
			}
		}
	}
	return nil
}

// repairMLTitleMerges 修复 ML 每块标题区的合并区：按【当前坐标】拆除后
// 原样重建（GetMergeCells 返回的已是平移后坐标，不做二次位移）。
// 同时用 GetRows 快照回填丢失的首格值。
func repairMLTitleMerges(f *excelize.File, sheet string, lay layout.MLLayout, snap [][]string) error {
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	rows, _ := f.GetRows(sheet)

	for start := 1; start <= len(rows); start += blockRows {
		titleEnd := start + 3 // 标题区 = 块首 ~ h1 前一行（上边距+标题+科目+空行）
		type geoMerge struct {
			c1, r1, c2, r2 int
			val            string
		}
		var olds []geoMerge
		ms, _ := f.GetMergeCells(sheet)
		for _, m := range ms {
			sc, sr, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			ec, er, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if err1 != nil || err2 != nil {
				continue
			}
			if sr >= start && er <= titleEnd {
				olds = append(olds, geoMerge{c1: sc, r1: sr, c2: ec, r2: er, val: m.GetCellValue()})
				f.UnmergeCell(sheet, cellName(sc, sr), cellName(ec, er))
			}
		}
		for _, g := range olds {
			top := cellName(g.c1, g.r1)
			bot := cellName(g.c2, g.r2)
			f.MergeCell(sheet, top, bot)
			if strings.TrimSpace(g.val) == "" && g.r1-1 < len(snap) && g.c1-1 < len(snap[g.r1-1]) {
				// 合并区值为空时尝试从展开前快照回填
				v := strings.TrimSpace(snap[g.r1-1][g.c1-1])
				if v != "" {
					f.SetCellValue(sheet, top, v)
				}
			} else if strings.TrimSpace(g.val) != "" {
				f.SetCellValue(sheet, top, g.val)
			}
		}
	}
	return nil
}

// mlShiftedCol 计算原始列号在全部展开完成后的实际列号。
// inserts 为各插入点的原始列号（在其右侧插 11 列）；
// 比较基于原始列号，避免级联重复位移。
func mlShiftedCol(col int, inserts []int) int {
	for _, at := range inserts {
		if col > at {
			col += 11
		}
	}
	return col
}

// sortIntsDesc 降序排序。
func sortIntsDesc(a []int) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] > a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

// updateMLHeadersForExpanded 更新 ML 四行表头以适应展开的金额列。
//
// 与 GL 相同的全拆全建策略：excelize 的 InsertCols 已把旧合并区平移到新位置，
// 但平移后的明细名合并仍是 1 列宽（需扩展为 12 列宽）、分析行 h1 合并也需扩展。
// 先 Unmerge 本块表头区内的全部既有合并区，再按最终布局重建。
func updateMLHeadersForExpanded(f *excelize.File, sheet string, lay layout.MLLayout, expandedCols []int) error {
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return nil
	}
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows

	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize, Color: "006100"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for start := 1; start <= len(rows); start += blockRows {
		isPaper1 := start == 1
		hStart := start + 4 // h1 行号
		h4 := hStart + 3

		// Step 1: 拆除本块表头四行（hStart ~ h4）内的全部既有合并区。
		// 记录两类信息：
		//   texts — 首格文字（金额区重建时取回）
		//   nonMoney — 非金额区合并的完整几何（起列/行偏移/跨度/值），
		//     摘要、借或贷、年份、凭证等在金额展开后必须原样重建
		type geoMerge struct {
			startCol int // 平移后坐标
			rowOff   int // 相对 hStart
			rowSpan  int
			colSpan  int
			val      string
		}
		texts := make(map[string]string)
		var nonMoney []geoMerge
		ms, _ := f.GetMergeCells(sheet)
		for _, m := range ms {
			sc, sr, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			ec, er, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if err1 != nil || err2 != nil {
				continue
			}
			if sr < hStart || er > h4 {
				continue
			}
			if strings.TrimSpace(m.GetCellValue()) != "" {
				texts[fmt.Sprintf("%d:%d", sr-hStart, sc)] = m.GetCellValue()
			}
			nonMoney = append(nonMoney, geoMerge{
				startCol: sc, rowOff: sr - hStart,
				rowSpan: er - sr + 1, colSpan: ec - sc + 1,
				val: m.GetCellValue(),
			})
			f.UnmergeCell(sheet, cellName(sc, sr), cellName(ec, er))
		}

		// Step 2（主动构建）：金额相关合并按已知结构重建。
		rebuildDetail := func(col int) {
			f.MergeCell(sheet, cellName(col, hStart+1), cellName(col+11, hStart+2))
			writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
		}
		if !isPaper1 {
			for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
				col := mlShiftedCol(lay.BackStartCol+off, expandedCols)
				f.MergeCell(sheet, cellName(col, hStart), cellName(col+11, hStart+2))
				writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
			}
			for i := 0; i < 4; i++ {
				rebuildDetail(mlShiftedCol(mlDetailCol(lay, i), expandedCols))
			}
		}
		for i := 4; i < mlMaxDetails; i++ {
			rebuildDetail(mlShiftedCol(mlDetailCol(lay, i), expandedCols))
		}

		// 分析行「( )方金 / 额 分析」：h1 行横跨该侧全部明细小列。
		if !isPaper1 {
			l := mlShiftedCol(mlDetailCol(lay, 0), expandedCols)
			r := mlShiftedCol(mlDetailCol(lay, 3), expandedCols) + 11
			f.MergeCell(sheet, cellName(l, hStart), cellName(r, hStart))
			if v, ok := texts[fmt.Sprintf("0:%d", l)]; ok && strings.TrimSpace(v) != "" {
				f.SetCellValue(sheet, cellName(l, hStart), v)
			}
		}
		{
			l := mlShiftedCol(mlDetailCol(lay, 4), expandedCols)
			r := mlShiftedCol(mlDetailCol(lay, mlMaxDetails-1), expandedCols) + 11
			f.MergeCell(sheet, cellName(l, hStart), cellName(r, hStart))
			if v, ok := texts[fmt.Sprintf("0:%d", l)]; ok && strings.TrimSpace(v) != "" {
				f.SetCellValue(sheet, cellName(l, hStart), v)
			}
		}

		// Step 3: 非金额区合并原样重建——摘要、借或贷、年份、凭证等。
		// 判定：起列不属于任何已重建的金额列集合。
		moneyColSet := make(map[int]bool)
		for _, at := range expandedCols {
			moneyColSet[at] = true
		}
		for _, g := range nonMoney {
			if moneyColSet[g.startCol] {
				continue // 已由 Step 2 重建
			}
			// 跨越多个非金额列的分析行（span>1 且起点非金额列）也已排除——
			// 分析行起点是明细首格，属于金额列集合。
			top := cellName(g.startCol, hStart+g.rowOff)
			bot := cellName(g.startCol+g.colSpan-1, hStart+g.rowOff+g.rowSpan-1)
			f.MergeCell(sheet, top, bot)
			if strings.TrimSpace(g.val) != "" {
				f.SetCellValue(sheet, top, g.val)
			}
		}
	}
	return nil
}

// writeDigitLabelsAt 在指定行的 12 个小格写入小列标签。
// 竖线按分组规则派生（与数据行一致）：组界绿粗、元|角红细、其余绿细。
func writeDigitLabelsAt(f *excelize.File, sheet string, firstCol, row int, style int) {
	for k, lbl := range digitColLabels {
		c := cellName(firstCol+k, row)
		f.SetCellValue(sheet, c, lbl)

		st := &excelize.Style{
			Font:      &excelize.Font{Size: printDigitFontSize, Color: "006100"},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		}
		// 左边框：首格无特殊语义用普通细线；其余 = 前一分隔线
		if k == 0 {
			st.Border = append(st.Border, excelize.Border{Type: "left", Color: "#006100", Style: 1})
		} else {
			dc, ds := dividerBorder(dividerStyles[k-1])
			st.Border = append(st.Border, excelize.Border{Type: "left", Color: dc, Style: ds})
		}
		// 右边框：末格普通细线；其余 = 自身分隔线
		if k == 11 {
			st.Border = append(st.Border, excelize.Border{Type: "right", Color: "#006100", Style: 1})
		} else {
			dc, ds := dividerBorder(dividerStyles[k])
			st.Border = append(st.Border, excelize.Border{Type: "right", Color: dc, Style: ds})
		}
		st.Border = append(st.Border,
			excelize.Border{Type: "top", Color: "#006100", Style: 1},
			excelize.Border{Type: "bottom", Color: "#006100", Style: 1},
		)
		sid, _ := f.NewStyle(st)
		f.SetCellStyle(sheet, c, c, sid)
	}
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
