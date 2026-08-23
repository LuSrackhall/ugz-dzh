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

	return updateGLHeadersForExpanded(f, sheet, cols)
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

		for k := 0; k < 12; k++ {
			st := &excelize.Style{
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

			sid, _ := f.NewStyle(st)
			cell := cellName(col+k, r)
			f.SetCellStyle(sheet, cell, cell, sid)

			if isNumeric {
				f.SetCellValue(sheet, cell, digits[k])
			} else if k > 0 {
				// 非数字内容保留在首格，其余格清空
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

	return updateMLHeadersForExpanded(f, sheet, lay, cols)
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

		// Step 1: 拆除本块表头四行（hStart ~ h4）内的全部既有合并区，
		// 记录 (起列, 起行-块内偏移, 值) 供重建。
		type oldMerge struct {
			col    int // 平移后起列（当前坐标）
			rowOff int // 相对 hStart 的行偏移 0..2
			span   int // 横向跨度
			val    string
		}
		var olds []oldMerge
		ms, _ := f.GetMergeCells(sheet)
		for _, m := range ms {
			sc, sr, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			ec, er, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if err1 != nil || err2 != nil {
				continue
			}
			if sr >= hStart && er <= h4 && sr == er-(er-sr) {
				// 纵向/矩形合并：只关心起止行都在表头区内
			}
			if sr >= hStart && er <= h4 {
				olds = append(olds, oldMerge{
					col: sc, rowOff: sr - hStart, span: ec - sc + 1,
					val: m.GetCellValue(),
				})
				f.UnmergeCell(sheet, cellName(sc, sr), cellName(ec, er))
			}
		}

		// Step 2: 判断旧合并区的语义角色并按新布局重建。
		// 角色判定用「块内相对行」：
		//   rowOff==0 且横跨整侧 → 分析行（方金/额分析），重建为跨该侧全部小列
		//   rowOff==1 → 明细科目名，扩展为 12 列宽
		//   rowOff==0 单列跨 0..2 行 → 借/贷/余额大标题，扩展为 12×3 矩形
		for _, om := range olds {
			switch {
			case om.rowOff == 0 && om.span > 1:
				// 分析行：保持原跨度（其下的明细列已各自展开，
				// 分析行的合并区按原比例扩展——直接用平移后的首末列）
				endCol := mlShiftedCol(om.col+om.span-1+countShiftsBefore(om.col+om.span-1, expandedCols), expandedCols)
				f.MergeCell(sheet, cellName(om.col, hStart), cellName(endCol, hStart))
				if strings.TrimSpace(om.val) != "" {
					f.SetCellValue(sheet, cellName(om.col, hStart), om.val)
				}
			case om.rowOff == 1:
				// 明细科目名：12 列宽 × h2:h3
				f.MergeCell(sheet, cellName(om.col, hStart+1), cellName(om.col+11, hStart+2))
				if strings.TrimSpace(om.val) != "" {
					f.SetCellValue(sheet, cellName(om.col, hStart+1), om.val)
				}
				writeDigitLabelsAt(f, sheet, om.col, h4, labelStyle)
			case om.rowOff == 0 && om.span == 1:
				// 借/贷/余额大标题：12×3 矩形
				f.MergeCell(sheet, cellName(om.col, hStart), cellName(om.col+11, hStart+2))
				if strings.TrimSpace(om.val) != "" {
					f.SetCellValue(sheet, cellName(om.col, hStart), om.val)
				}
				writeDigitLabelsAt(f, sheet, om.col, h4, labelStyle)
			default:
				// 其他（如年份/凭证等非金额区合并）：按原样恢复
				f.MergeCell(sheet,
					cellName(om.col, hStart+om.rowOff),
					cellName(mlShiftedCol(om.col, expandedCols)+0, hStart+om.rowOff))
			}
		}

		if isPaper1 {
			continue
		}
		// Back 侧借/贷/余额与明细1-4 在上面的 olds 循环中已覆盖；
		// 若某列原本无内容（空明细列无合并区），补写 h4 小列标签。
		for i := 0; i < 4; i++ {
			col := mlShiftedCol(mlDetailCol(lay, i), expandedCols)
			writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
		}
		for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
			col := mlShiftedCol(lay.BackStartCol+off, expandedCols)
			writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
		}
		// Front 侧空明细列同样补标签
		for i := 4; i < mlMaxDetails; i++ {
			col := mlShiftedCol(mlDetailCol(lay, i), expandedCols)
			writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
		}
	}
	return nil
}

// countShiftsBefore 统计 expandedCols 中小于 col 的插入点数量（辅助分析行末端列计算）。
func countShiftsBefore(col int, expandedCols []int) int {
	n := 0
	for _, at := range expandedCols {
		if col > at {
			n++
		}
	}
	return n
}

// writeDigitLabelsAt 在指定行的 12 个小格写入小列标签。
func writeDigitLabelsAt(f *excelize.File, sheet string, firstCol, row int, style int) {
	for k, lbl := range digitColLabels {
		c := cellName(firstCol+k, row)
		f.SetCellValue(sheet, c, lbl)
		f.SetCellStyle(sheet, c, c, style)
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
