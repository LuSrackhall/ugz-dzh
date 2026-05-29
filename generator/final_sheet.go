package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

// WriteFinalSheet 生成独立的 {month}期末 Sheet。
// 期末余额 = 期初 + 当月借方 - 当月贷方。
func (wb *Workbook) WriteFinalSheet(initials map[string]int64, activity map[string]Activity) error {
	name := wb.Month + "期末"

	for _, sheet := range wb.File.GetSheetList() {
		if strings.HasSuffix(sheet, "期末") {
			wb.File.DeleteSheet(sheet)
		}
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return fmt.Errorf("创建期末表 Sheet: %w", err)
	}
	wb.File.SetActiveSheet(idx)

	title := wb.Month + " 期末余额"
	wb.File.SetCellValue(name, "A1", title)
	wb.File.MergeCell(name, "A1", "C1")

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(name, "A1", "C1", titleStyle)
	wb.File.SetRowHeight(name, 1, 22)

	headers := []string{"科目", "方向", "期末余额"}
	for i, h := range headers {
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

	wb.File.SetColWidth(name, "A", "A", 40)
	wb.File.SetColWidth(name, "B", "B", 8)
	wb.File.SetColWidth(name, "C", "C", 16)

	accounts := make([]string, 0, len(initials))
	for account := range initials {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)

	row := 3
	var totalFinal int64
	for _, account := range accounts {
		act, ok := activity[account]
		if !ok {
			act = Activity{Debit: 0, Credit: 0}
		}
		final := initials[account] + act.Debit - act.Credit
		totalFinal += final

		dir, dispBal := directionFor(final, 0)

		wb.File.SetCellValue(name, cellName(1, row), account)
		wb.File.SetCellValue(name, cellName(2, row), dir)
		wb.File.SetCellValue(name, cellName(3, row), centsToYuan(dispBal))
		wb.setMoneyStyle(name, row, 3)
		row++
	}

	totalCell := cellName(1, row)
	wb.File.SetCellValue(name, totalCell, "合计")
	totalDir, totalDispBal := directionFor(totalFinal, 0)
	wb.File.SetCellValue(name, cellName(2, row), totalDir)
	wb.File.SetCellValue(name, cellName(3, row), centsToYuan(totalDispBal))

	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(name, totalCell, cellName(3, row), totalStyle)

	wb.setMoneyStyle(name, row, 3)

	return nil
}
