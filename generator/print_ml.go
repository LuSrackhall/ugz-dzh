// Package generator — 打印版位格转换：多科目明细账（ML）账页。
//
// 与 GL 的差异：
//   - 四行表头：借/贷/余额大标题占 h1-h3（纵向），明细科目名占 h2-h3，
//     小列标签写 h4 行；「( )方金 / 额 分析」合并区随明细列同步扩展
//   - 首块为 Paper1 Front 占位页（仅明细5-14）
//   - 垂直分页符在 PageGapStartCol+2，插列后右移 Back 侧 7 个金额列 × 11 = 77 列
package generator

import (
	"fmt"
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// convertMLSheet 将一个多科目明细账 Sheet 转换为打印版位格格式。
func convertMLSheet(f *excelize.File, sheet string) error {
	lay := mlLayout()
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("读取 %s: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil
	}

	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return fmt.Errorf("读取合并区: %w", err)
	}

	var backMoney, frontMoney []moneyColumn
	for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
		backMoney = append(backMoney, moneyColumn{col: lay.BackStartCol + off})
	}
	for i := 0; i < 4; i++ {
		backMoney = append(backMoney, moneyColumn{col: mlDetailCol(lay, i)})
	}
	for i := 4; i < mlMaxDetails; i++ {
		frontMoney = append(frontMoney, moneyColumn{col: mlDetailCol(lay, i)})
	}

	insertsBack := mlInsertPoints(lay, false)
	insertsFront := mlInsertPoints(lay, true)

	if _, err := splitMoneyColumns(f, sheet, frontMoney); err != nil {
		return fmt.Errorf("正面金额列插列: %w", err)
	}
	if _, err := splitMoneyColumns(f, sheet, backMoney); err != nil {
		return fmt.Errorf("反面金额列插列: %w", err)
	}

	dataStart := lay.DataStartRow + 1
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	for r := dataStart; r <= len(rows); r++ {
		inFirstBlock := (r-1)/blockRows == 0 // Paper1 占位块只有 Front 明细5-14
		snap := rows[r-1]
		if !inFirstBlock {
			rewriteAmountCells(f, sheet, r, snap, lay.BindingLeftCols,
				[]int{mlOffDebit, mlOffCredit, mlOffBalance}, insertsBack)
			for i := 0; i < 4; i++ {
				rewriteOneMLDetail(f, sheet, r, snap, mlDetailCol(lay, i), insertsBack)
			}
		}
		for i := 4; i < mlMaxDetails; i++ {
			rewriteOneMLDetail(f, sheet, r, snap, mlDetailCol(lay, i), insertsFront)
		}
	}

	allInserts := append(append([]int{}, insertsBack...), insertsFront...)
	if err := rebuildMLMerges(f, sheet, merges, allInserts); err != nil {
		return err
	}
	if err := convertMLHeaders(f, sheet, lay, insertsBack, insertsFront); err != nil {
		return err
	}
	allFirsts := [][]int{
		mlShiftedFirstCols(lay, insertsBack, false),
		mlShiftedFirstCols(lay, insertsFront, true),
	}
	if err := applyDataGroupBorders(f, sheet, dataStart, len(rows), allFirsts); err != nil {
		return err
	}
	return rebuildMLPageBreaks(f, sheet)
}

// mlInsertPoints 返回 ML 一侧的金额列原始插入点。front=true 为明细5-14。
func mlInsertPoints(lay layout.MLLayout, front bool) []int {
	var out []int
	if !front {
		for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
			out = append(out, lay.BackStartCol+off)
		}
		for i := 0; i < 4; i++ {
			out = append(out, mlDetailCol(lay, i))
		}
		return out
	}
	for i := 4; i < mlMaxDetails; i++ {
		out = append(out, mlDetailCol(lay, i))
	}
	return out
}

// rewriteOneMLDetail 改写一个明细列的净额为位格。origCol 为原始 Excel 列号。
func rewriteOneMLDetail(f *excelize.File, sheet string, row int, snap []string, origCol int, inserts []int) {
	idx := origCol - 1
	if idx >= len(snap) {
		return
	}
	valStr := strings.TrimSpace(snap[idx])
	if valStr == "" {
		return
	}
	cents, err := yuanStrToCents(valStr)
	if err != nil {
		return
	}
	srcCol := shiftColsByInserts(origCol, inserts)
	writeDigits(f, sheet, row, srcCol, splitCNY(cents))
}

// convertMLHeaders 改造 ML 四行表头：
// 借/贷/余额大标题 h1-h3 × 12 小列矩形合并、明细名 h2-h3 × 12 小列扩展、
// h4 行写入小列标签、「( )方金 / 额 分析」行合并区扩展。
func convertMLHeaders(f *excelize.File, sheet string, lay layout.MLLayout, backIns, frontIns []int) error {
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	rows, _ := f.GetRows(sheet)
	lastRow := len(rows)

	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize, Color: mlGreen},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for start := 1; start <= lastRow; start += blockRows {
		isPaper1 := start == 1
		hStart := start + 4 // h1 = 表内区域顶（4行表头第一行）
		h4 := hStart + 3

		// Back 侧：借/贷/余额 大标题扩展为 12 列宽矩形合并；h4 写小列标签
		if !isPaper1 {
			backBase := func(col int) int { return shiftColsByInserts(col, backIns) }
			moneyHeads := map[int]string{} // 原首列 → 标题文字
			for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
				firstCol := backBase(lay.BackStartCol + off)
				v, _ := f.GetCellValue(sheet, cellName(firstCol, hStart))
				moneyHeads[firstCol] = v
			}
			for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
				firstCol := backBase(lay.BackStartCol + off)
				lastCol := firstCol + 11
				topLeft := cellName(firstCol, hStart)
				f.MergeCell(sheet, topLeft, cellName(lastCol, hStart+2))
				if v := moneyHeads[firstCol]; strings.TrimSpace(v) != "" {
					f.SetCellValue(sheet, topLeft, v)
				}
				writeDigitLabels(f, sheet, firstCol, h4, labelStyle)
			}
			// 明细1-4：h2-h3 合并扩展为 12 列宽；h4 写小列标签
			for i := 0; i < 4; i++ {
				firstCol := backBase(mlDetailCol(lay, i))
				lastCol := firstCol + 11
				topLeft := cellName(firstCol, hStart+1)
				f.MergeCell(sheet, topLeft, cellName(lastCol, hStart+2))
				writeDigitLabels(f, sheet, firstCol, h4, labelStyle)
			}
		}

		// Front 侧（Paper1 也执行）：明细5-14 同 Back 明细处理
		for i := 4; i < mlMaxDetails; i++ {
			firstCol := shiftColsByInserts(mlDetailCol(lay, i), frontIns)
			lastCol := firstCol + 11
			topLeft := cellName(firstCol, hStart+1)
			f.MergeCell(sheet, topLeft, cellName(lastCol, hStart+2))
			writeDigitLabels(f, sheet, firstCol, h4, labelStyle)
		}

		// 「( )方金 / 额 分析」h1 合并区扩展：
		// 原 Back 左4列(明细1-4)合并 → 扩展为覆盖其 12×4 小列；
		// 原 Front 右10列合并 → 扩展为覆盖其全部小列
		if !isPaper1 {
			l := shiftColsByInserts(mlDetailCol(lay, 0), backIns)
			r := shiftColsByInserts(mlDetailCol(lay, 3), backIns) + 11
			txt, _ := f.GetCellValue(sheet, cellName(shiftColsByInserts(mlDetailCol(lay, 0), backIns), hStart))
			f.MergeCell(sheet, cellName(l, hStart), cellName(r, hStart))
			if strings.TrimSpace(txt) != "" {
				f.SetCellValue(sheet, cellName(l, hStart), txt)
			}
		}
		lf := shiftColsByInserts(mlDetailCol(lay, 4), frontIns)
		rf := shiftColsByInserts(mlDetailCol(lay, mlMaxDetails-1), frontIns) + 11
		txtF, _ := f.GetCellValue(sheet, cellName(lf, hStart))
		f.MergeCell(sheet, cellName(lf, hStart), cellName(rf, hStart))
		if strings.TrimSpace(txtF) != "" {
			f.SetCellValue(sheet, cellName(lf, hStart), txtF)
		}
	}
	return nil
}

// writeDigitLabels 在 h4 行的 12 个小格写入小列标签。
func writeDigitLabels(f *excelize.File, sheet string, firstCol, row int, style int) {
	for k, lbl := range digitColLabels {
		c := cellName(firstCol+k, row)
		f.SetCellValue(sheet, c, lbl)
		f.SetCellStyle(sheet, c, c, style)
	}
}

// rebuildMLMerges 对跨金额区的既有合并区按插列偏移重建：
// 先 Unmerge 全部旧区域，再把每个区域的起止列经 shiftColsByInserts 映射后重新 Merge。
func rebuildMLMerges(f *excelize.File, sheet string, merges []excelize.MergeCell, inserts []int) error {
	type rng struct{ c1, r1, c2, r2 int }
	var toRebuild []rng
	for _, m := range merges {
		c1, r1, err := excelize.CellNameToCoordinates(m.GetStartAxis())
		if err != nil {
			continue
		}
		c2, r2, err := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err != nil {
			continue
		}
		toRebuild = append(toRebuild, rng{c1, r1, c2, r2})
	}
	for _, m := range merges {
		if err := f.UnmergeCell(sheet, m.GetStartAxis(), m.GetEndAxis()); err != nil {
			return fmt.Errorf("取消合并 %s: %w", m.GetStartAxis(), err)
		}
	}
	for _, g := range toRebuild {
		nc1 := shiftColsByInserts(g.c1, inserts)
		nc2 := shiftColsByInserts(g.c2, inserts)
		if err := f.MergeCell(sheet, cellName(nc1, g.r1), cellName(nc2, g.r2)); err != nil {
			return fmt.Errorf("重设合并 %s: %w", cellName(nc1, g.r1), err)
		}
	}
	return nil
}

// mlShiftedFirstCols 返回一侧全部金额栏的首列集合（插列后坐标）。
func mlShiftedFirstCols(lay layout.MLLayout, inserts []int, front bool) []int {
	var out []int
	if !front {
		for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
			out = append(out, shiftColsByInserts(lay.BackStartCol+off, inserts))
		}
		for i := 0; i < 4; i++ {
			out = append(out, shiftColsByInserts(mlDetailCol(lay, i), inserts))
		}
		return out
	}
	for i := 4; i < mlMaxDetails; i++ {
		out = append(out, shiftColsByInserts(mlDetailCol(lay, i), inserts))
	}
	return out
}

// rebuildMLPageBreaks 重建 ML 垂直分页符：原位置 PageGapStartCol+2，
// 插列后右移 Back 侧 7 个金额列 × 11 = 77 列。
func rebuildMLPageBreaks(f *excelize.File, sheet string) error {
	lay := mlLayout()
	oldCol := lay.PageGapStartCol + 2
	newCol := shiftColsByInserts(oldCol, mlInsertPoints(lay, false))
	oldCell, _ := excelize.ColumnNumberToName(oldCol)
	newCell, _ := excelize.ColumnNumberToName(newCol)
	if oldCell != newCell {
		if err := f.RemovePageBreak(sheet, oldCell+"1"); err != nil {
			return fmt.Errorf("移除旧垂直分页符: %w", err)
		}
	}
	return f.InsertPageBreak(sheet, newCell+"1")
}
