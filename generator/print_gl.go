// Package generator — 打印版位格转换：GL/MergeGL 账页。
//
// 前置条件：输入是查看版生成器写出的 xlsx 副本（内存中），布局常量与
// layout.DefaultGLSpec 一致。转换只动金额栏（拆 12 小列）及其表头/边框，
// 其余区域依赖 excelize 插列自动平移。
package generator

import (
	"fmt"
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// glMoneyOffsets GL 每面区的金额列 offset（相对 FrontStartCol/BackStartCol）。
var glMoneyOffsets = []int{glColDebit, glColCredit, glColBalance}

// convertGLSheet 将一个总分类账 Sheet 转换为打印版位格格式。
// f 是查看版 xlsx 的内存副本；转换在内存中完成后由调用方 SaveAs。
func convertGLSheet(f *excelize.File, sheet string) error {
	lay := glLayout()
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("读取 %s: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil
	}

	frontMoney := make([]moneyColumn, 0, len(glMoneyOffsets))
	for _, off := range glMoneyOffsets {
		frontMoney = append(frontMoney, moneyColumn{col: lay.FrontStartCol + off})
	}
	backMoney := make([]moneyColumn, 0, len(glMoneyOffsets))
	for _, off := range glMoneyOffsets {
		backMoney = append(backMoney, moneyColumn{col: lay.BackStartCol + off})
	}

	// 从右往左：Back 区先转，Front 区后转
	if _, err := splitMoneyColumns(f, sheet, backMoney); err != nil {
		return fmt.Errorf("反面金额列插列: %w", err)
	}
	if _, err := splitMoneyColumns(f, sheet, frontMoney); err != nil {
		return fmt.Errorf("正面金额列插列: %w", err)
	}

	insertsBack := glInsertPoints(lay.BackStartCol)
	insertsFront := glInsertPoints(lay.FrontStartCol)

	// 数据行拆位：快照 rows 是插列前的值，坐标经 shiftColsByInserts 映射到新列
	dataStart := lay.DataStartRow + 1 + lay.TopMarginRows
	for r := dataStart; r <= len(rows); r++ {
		snap := rows[r-1]
		rewriteAmountCells(f, sheet, r, snap, lay.BindingLeftCols, glMoneyOffsets, insertsFront)
		rewriteAmountCells(f, sheet, r, snap, lay.BackStartCol-1, glMoneyOffsets, insertsBack)
	}

	// 表头改造 + 分组边框（两侧独立执行）
	if err := convertGLHeader(f, sheet, lay.FrontStartCol, insertsFront); err != nil {
		return err
	}
	if err := convertGLHeader(f, sheet, lay.BackStartCol, insertsBack); err != nil {
		return err
	}
	if err := applyDataGroupBorders(f, sheet, lay.SubHeaderRow+1+1, len(rows), baseWithShifts(insertsFront, insertsBack)); err != nil {
		return err
	}
	return rebuildGLPageBreaks(f, sheet, lay)
}

// glInsertPoints 返回一侧区三个金额列的原始插入点（右侧各插 11 列）。
func glInsertPoints(base int) []int {
	out := make([]int, 0, len(glMoneyOffsets))
	for _, off := range glMoneyOffsets {
		out = append(out, base+off)
	}
	return out
}

// rewriteAmountCells 将一行内若干原金额格改写为 12 小格数字。
// snapIdxBase 为该区在 GetRows 中的基准索引：Front 区传 BindingLeftCols，
// Back 区传 BackStartCol-1；offsets 为区内的金额列偏移。
func rewriteAmountCells(f *excelize.File, sheet string, row int, snap []string, snapIdxBase int, offsets []int, inserts []int) {
	for _, off := range offsets {
		snapIdx := snapIdxBase + off
		if snapIdx >= len(snap) {
			continue
		}
		valStr := strings.TrimSpace(snap[snapIdx])
		if valStr == "" {
			continue
		}
		cents, err := yuanStrToCents(valStr)
		if err != nil {
			continue // 非数字内容不拆位
		}
		srcCol := shiftColsByInserts(snapIdx+1, inserts) // GetRows 索引 → Excel 列号
		writeDigits(f, sheet, row, srcCol, splitCNY(cents))
	}
}

// writeDigits 将 12 位数字写入以 firstCol 起的 12 个小格。
// 数字为空的格子写空字符串；全部小格统一小号居中样式并保留原字体颜色。
func writeDigits(f *excelize.File, sheet string, row, firstCol int, digits [12]string) {
	styleID, _ := f.GetCellStyle(sheet, cellName(firstCol, row))
	font := &excelize.Font{Size: printDigitFontSize}
	if styleID > 0 {
		if st, err := f.GetStyle(styleID); err == nil && st.Font != nil && st.Font.Color != "" {
			font.Color = st.Font.Color
		}
	}
	newStyle, _ := f.NewStyle(&excelize.Style{
		Font:      font,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	for k, d := range digits {
		c := cellName(firstCol+k, row)
		f.SetCellValue(sheet, c, d)
		f.SetCellStyle(sheet, c, c, newStyle)
	}
}

// convertGLHeader 改造一侧区表头：金额大标题跨 12 列合并 + SubHeaderRow 小列标签。
func convertGLHeader(f *excelize.File, sheet string, base int, inserts []int) error {
	headerRow := glLayout().HeaderRow + 1
	subRow := glLayout().SubHeaderRow + 1

	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize, Color: mlGreen},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Color: "#006100", Style: 1},
			{Type: "right", Color: "#006100", Style: 1},
			{Type: "bottom", Color: "#006100", Style: 1},
			{Type: "left", Color: "#006100", Style: 1},
		},
	})

	for _, off := range glMoneyOffsets {
		firstCol := shiftColsByInserts(base+off, inserts)
		lastCol := firstCol + 11

		// 大标题跨 12 列合并（原文字在首格，MergeCell 后仍显示）
		titleCell := cellName(firstCol, headerRow)
		f.MergeCell(sheet, titleCell, cellName(lastCol, headerRow))

		// SubHeaderRow 原本只有月/日/字/号占用，金额区为空位行——写入小列标签
		for k, lbl := range digitColLabels {
			c := cellName(firstCol+k, subRow)
			f.SetCellValue(sheet, c, lbl)
			f.SetCellStyle(sheet, c, c, labelStyle)
		}
	}
	return nil
}

// applyDataGroupBorders 对数据区每行金额栏的 12 小格应用分组竖线与继承边框。
// cols 为各金额栏首列（插列后）；上下边框从第 1 小格读原语义复制到全部 12 格。
func applyDataGroupBorders(f *excelize.File, sheet string, startRow, lastRow int, cols [][]int) error {
	for r := startRow; r <= lastRow; r++ {
		for _, firstColSet := range cols {
			for _, firstCol := range firstColSet {
				orig := readBorderStyle(f, sheet, firstCol, r)
				for k := 0; k < 12; k++ {
					colK := firstCol + k
					st := &excelize.Style{}
					// 上下边框：继承原格语义（每5行加粗、过次页红双线底边等）
					if orig.top != nil {
						st.Border = append(st.Border, *orig.top)
					}
					if orig.bottom != nil {
						st.Border = append(st.Border, *orig.bottom)
					}
					// 左边框：首格继承原左边框；其余格左边框 = 前一分隔线
					if k == 0 && orig.leftColor != "" {
						st.Border = append(st.Border, excelize.Border{Type: "left", Color: orig.leftColor, Style: orig.leftStyle})
					} else if k > 0 {
						color, style := dividerBorder(dividerStyles[k-1])
						st.Border = append(st.Border, excelize.Border{Type: "left", Color: color, Style: style})
					}
					// 右边框：末格继承原右边框；其余格右边框 = 自身分隔线
					if k == 11 && orig.rightColor != "" {
						st.Border = append(st.Border, excelize.Border{Type: "right", Color: orig.rightColor, Style: orig.rightStyle})
					} else if k < 11 {
						color, style := dividerBorder(dividerStyles[k])
						st.Border = append(st.Border, excelize.Border{Type: "right", Color: color, Style: style})
					}
					sid, _ := f.NewStyle(st)
					f.SetCellStyle(sheet, cellName(colK, r), cellName(colK, r), sid)
				}
			}
		}
	}
	return nil
}

// baseWithShifts 汇总两侧所有金额栏的首列集合（插列后坐标）。
func baseWithShifts(frontIns, backIns []int) [][]int {
	lay := glLayout()
	var out [][]int
	var fCols []int
	for _, off := range glMoneyOffsets {
		fCols = append(fCols, shiftColsByInserts(lay.FrontStartCol+off, frontIns))
	}
	var bCols []int
	for _, off := range glMoneyOffsets {
		bCols = append(bCols, shiftColsByInserts(lay.BackStartCol+off, backIns))
	}
	out = append(out, fCols, bCols)
	return out
}

// readBorderStyle 读取单元格现有边框语义。
func readBorderStyle(f *excelize.File, sheet string, col, row int) borderSpec {
	var spec borderSpec
	sid, err := f.GetCellStyle(sheet, cellName(col, row))
	if err != nil || sid == 0 {
		return spec
	}
	st, err := f.GetStyle(sid)
	if err != nil || st == nil {
		return spec
	}
	for _, b := range st.Border {
		switch b.Type {
		case "left":
			spec.leftColor, spec.leftStyle = b.Color, b.Style
		case "right":
			spec.rightColor, spec.rightStyle = b.Color, b.Style
		case "top":
			t := b
			spec.top = &t
		case "bottom":
			bt := b
			spec.bottom = &bt
		}
	}
	return spec
}

// rebuildGLPageBreaks 重建 GL 垂直分页符：旧位置删除、右移后重插。
// GL 的垂直分页符固定写在 PageGapStartCol+1（见 workbook.go setAllSheetPageLayout），
// 插列后需右移 Front 区 3 个金额列 × 11 = 33 列。
func rebuildGLPageBreaks(f *excelize.File, sheet string, lay layout.GLLayout) error {
	oldCol := lay.PageGapStartCol + 1
	inserts := glInsertPoints(lay.FrontStartCol)
	newCol := shiftColsByInserts(oldCol, inserts)
	oldCell, _ := excelize.ColumnNumberToName(oldCol)
	newCell, _ := excelize.ColumnNumberToName(newCol)
	if oldCell != newCell {
		if err := f.RemovePageBreak(sheet, oldCell+"1"); err != nil {
			return fmt.Errorf("移除旧垂直分页符: %w", err)
		}
	}
	return f.InsertPageBreak(sheet, newCell+"1")
}
