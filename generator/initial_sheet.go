package generator

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// initSheetHeaders 期初表列标题
var initSheetHeaders = []string{"科目", "方向", "期初余额"}

// WriteInitialSheet 生成 {month}期初 Sheet。
// initials 为各叶子科目全路径 → 期初余额（分）的映射。
func (wb *Workbook) WriteInitialSheet(initials map[string]int64) error {
	name := wb.Month + "期初"

	// 若已存在则先删除再重建
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		wb.File.DeleteSheet(name)
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return fmt.Errorf("创建期初表 Sheet: %w", err)
	}
	wb.File.SetActiveSheet(idx)

	// 标题行
	title := wb.Month + " 期初余额"
	wb.File.SetCellValue(name, "A1", title)
	wb.File.MergeCell(name, "A1", "C1")

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(name, "A1", "C1", titleStyle)
	wb.File.SetRowHeight(name, 1, 22)

	// 列标题
	for i, h := range initSheetHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(name, cell, h)
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(name, "A2", "C2", headerStyle)

	// 列宽
	wb.File.SetColWidth(name, "A", "A", 40)
	wb.File.SetColWidth(name, "B", "B", 8)
	wb.File.SetColWidth(name, "C", "C", 16)

	// 数据行：按 config 中科目顺序输出
	row := 3
	for _, account := range wb.Config.Settings.Order {
		amount, ok := initials[account]
		if !ok {
			continue
		}
		dir, dispBal := directionFor(amount, 0)

		wb.File.SetCellValue(name, cellName(1, row), account)
		wb.File.SetCellValue(name, cellName(2, row), dir)
		wb.File.SetCellValue(name, cellName(3, row), centsToYuanStr(dispBal))
		row++
	}

	// 合计行
	totalCell := cellName(1, row)
	wb.File.SetCellValue(name, totalCell, "合计")

	var totalInit int64
	for _, account := range wb.Config.Settings.Order {
		totalInit += initials[account]
	}
	totalDir, totalDispBal := directionFor(totalInit, 0)
	wb.File.SetCellValue(name, cellName(2, row), totalDir)
	wb.File.SetCellValue(name, cellName(3, row), centsToYuanStr(totalDispBal))

	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(name, totalCell, cellName(3, row), totalStyle)

	return nil
}
