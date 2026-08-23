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
			// 该页该侧的三个金额列（原始列号；插列后首格仍在原列位置）
			for _, off := range glPrintMoneyOffsets {
				col := base + off
				if !colSet[col] {
					continue
				}
				// 大标题跨 12 列合并
				f.MergeCell(sheet, cellName(col, headerRow), cellName(col+11, headerRow))
				// 小列标签写入 SubHeaderRow
				writeDigitLabelsAt(f, sheet, col, subRow, labelStyle)
			}
		}
	}
	return nil
}

// expandMoneyColumn 将单个金额列展开为 12 小列。
// 在原列右侧插入 11 列，读取原列值，拆位写入 12 小格。
func expandMoneyColumn(f *excelize.File, sheet string, col, lastRow int) error {
	// 读取原列所有行的值
	values := make([]string, lastRow+1)
	for r := 1; r <= lastRow; r++ {
		cell := cellName(col, r)
		v, _ := f.GetCellValue(sheet, cell)
		values[r] = strings.TrimSpace(v)
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

	// 写入拆位数据
	for r := 1; r <= lastRow; r++ {
		v := values[r]
		if v == "" {
			continue
		}
		cents, err := yuanStrToCents(v)
		if err != nil {
			continue // 非数字内容保留原样（已在原列）
		}
		digits := splitCNY(cents)
		for k, d := range digits {
			f.SetCellValue(sheet, cellName(col+k, r), d)
		}
		// 清空原列的数字格式（改为小号居中）
		style, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Size: printDigitFontSize},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		for k := 0; k < 12; k++ {
			f.SetCellStyle(sheet, cellName(col+k, r), cellName(col+k, r), style)
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
		hStart := start + 4 // h1
		h4 := hStart + 3

		if !isPaper1 {
			// Back 侧借/贷/余额大标题 h1-h3 矩形合并扩展 + h4 标签
			for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
				col := mlShiftedCol(lay.BackStartCol+off, expandedCols)
				topLeft := cellName(col, hStart)
				f.MergeCell(sheet, topLeft, cellName(col+11, hStart+2))
				writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
			}
			// 明细1-4：h2-h3 合并扩展 + h4 标签
			for i := 0; i < 4; i++ {
				col := mlShiftedCol(mlDetailCol(lay, i), expandedCols)
				topLeft := cellName(col, hStart+1)
				f.MergeCell(sheet, topLeft, cellName(col+11, hStart+2))
				writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
			}
		}

		// Front 侧明细5-14：h2-h3 合并扩展 + h4 标签
		for i := 4; i < mlMaxDetails; i++ {
			col := mlShiftedCol(mlDetailCol(lay, i), expandedCols)
			topLeft := cellName(col, hStart+1)
			f.MergeCell(sheet, topLeft, cellName(col+11, hStart+2))
			writeDigitLabelsAt(f, sheet, col, h4, labelStyle)
		}
	}
	return nil
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
