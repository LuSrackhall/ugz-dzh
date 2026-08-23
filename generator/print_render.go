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
func transformGLMoneyCols(f *excelize.File, sheet string) error {
	lay := glLayout()
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil
	}

	// 从右往左处理两侧的金额列，避免插入点偏移
	// Back 侧先处理（列号更大），Front 侧后处理
	for _, side := range []struct {
		base   int
		offsets []int
	}{
		{lay.BackStartCol, glPrintMoneyOffsets},
		{lay.FrontStartCol, glPrintMoneyOffsets},
	} {
		// 从右往左处理每个金额列
		for i := len(side.offsets) - 1; i >= 0; i-- {
			off := side.offsets[i]
			col := side.base + off
			if err := expandMoneyColumn(f, sheet, col, len(rows)); err != nil {
				return err
			}
		}
	}

	// 重新设置金额列宽（12 小列均分原宽）
	setExpandedColWidths(f, sheet, lay)

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

	// 更新表头：大标题跨 12 列合并，小列标签写入 SubHeaderRow
	updateGLHeaderForExpanded(f, sheet, col)

	return nil
}

// updateGLHeaderForExpanded 更新 GL 表头以适应展开的金额列。
func updateGLHeaderForExpanded(f *excelize.File, sheet string, col int) {
	lay := glLayout()
	headerRow := lay.HeaderRow + 1
	subRow := lay.SubHeaderRow + 1

	// 大标题跨 12 列合并（原文字在首格）
	topLeft := cellName(col, headerRow)
	bottomRight := cellName(col+11, headerRow)
	f.MergeCell(sheet, topLeft, bottomRight)

	// 小列标签写入 SubHeaderRow
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: printDigitFontSize, Color: "006100"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	for k, lbl := range digitColLabels {
		c := cellName(col+k, subRow)
		f.SetCellValue(sheet, c, lbl)
		f.SetCellStyle(sheet, c, c, labelStyle)
	}
}

// setExpandedColWidths 重新设置展开后的金额列宽。
func setExpandedColWidths(f *excelize.File, sheet string, lay layout.GLLayout) {
	// 这个函数在 expandMoneyColumn 中已经处理了列宽
	// 这里可以添加额外的列宽调整逻辑
}

// transformMLMoneyCols 将 ML Sheet 的金额列展开为 12 小列。
func transformMLMoneyCols(f *excelize.File, sheet string) error {
	lay := mlLayout()
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil
	}

	// Back 侧：借/贷/余额 + 明细1-4
	backOffsets := []int{mlOffDebit, mlOffCredit, mlOffBalance}
	for i := 0; i < 4; i++ {
		backOffsets = append(backOffsets, mlDetailCol(lay, i))
	}

	// Front 侧：明细5-14
	frontOffsets := make([]int, 0, 10)
	for i := 4; i < mlMaxDetails; i++ {
		frontOffsets = append(frontOffsets, mlDetailCol(lay, i))
	}

	// 从右往左处理
	for _, side := range []struct {
		base    int
		offsets []int
	}{
		{0, frontOffsets}, // Front 侧（mlDetailCol 已含绝对列号）
		{0, backOffsets},  // Back 侧
	} {
		for i := len(side.offsets) - 1; i >= 0; i-- {
			col := side.offsets[i]
			if err := expandMoneyColumn(f, sheet, col, len(rows)); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
