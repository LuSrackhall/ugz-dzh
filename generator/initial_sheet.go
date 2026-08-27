package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

// initSheetHeaders 期初表列标题（设计专家审查 Change 9：借贷分列试算平衡口径）
var initSheetHeaders = []string{"科目", "方向", "借方金额", "贷方金额"}

// WriteInitialSheet 生成 {month}期初 Sheet。
// initials 为各叶子科目全路径 → 期初余额（分）的映射。
// 会先删除所有期初 Sheet（包括往月残留），再创建本月期初 Sheet。
func (wb *Workbook) WriteInitialSheet(initials map[string]int64) error {
	name := wb.Month + "期初"

	// 删除所有往月期初 Sheet（从复制上月的 xlsx 继承而来）
	for _, sheet := range wb.File.GetSheetList() {
		if strings.HasSuffix(sheet, "期初") {
			wb.File.DeleteSheet(sheet)
		}
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return fmt.Errorf("创建期初表 Sheet: %w", err)
	}
	wb.File.SetActiveSheet(idx)

	// 标题行
	title := wb.Month + " 期初余额（试算平衡）"
	wb.File.SetCellValue(name, "A1", title)
	wb.File.MergeCell(name, "A1", "D1")

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(name, "A1", "D1", titleStyle)
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
	wb.File.SetCellStyle(name, "A2", "D2", headerStyle)

	// 列宽
	wb.File.SetColWidth(name, "A", "A", 40)
	wb.File.SetColWidth(name, "B", "B", 8)
	wb.File.SetColWidth(name, "C", "C", 16)
	wb.File.SetColWidth(name, "D", "D", 16)

	// 数据行：按科目全路径排序输出
	accounts := make([]string, 0, len(initials))
	for account := range initials {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)

	var totalDebit, totalCredit int64
	row := 3
	for _, account := range accounts {
		amount := initials[account]
		dir, dispBal := directionFor(amount, 0)

		wb.File.SetCellValue(name, cellName(1, row), account)
		wb.File.SetCellValue(name, cellName(2, row), dir)
		// 借贷分列（试算平衡口径）：借→借方金额列，贷→贷方金额列
		if amount > 0 {
			wb.File.SetCellValue(name, cellName(3, row), centsToYuan(dispBal))
			wb.setMoneyStyle(name, row, 3)
			totalDebit += amount
		} else if amount < 0 {
			wb.File.SetCellValue(name, cellName(4, row), centsToYuan(dispBal))
			wb.setMoneyStyle(name, row, 4)
			totalCredit += -amount
		}
		row++
	}

	// 合计行：借方合计 / 贷方合计 / 差额（试算平衡）
	totalCell := cellName(1, row)
	wb.File.SetCellValue(name, totalCell, "合计")
	wb.File.SetCellValue(name, cellName(2, row), "")
	wb.File.SetCellValue(name, cellName(3, row), centsToYuan(totalDebit))
	wb.File.SetCellValue(name, cellName(4, row), centsToYuan(totalCredit))
	if totalDebit != totalCredit {
		diff := totalDebit - totalCredit
		wb.File.SetCellValue(name, cellName(1, row+1), fmt.Sprintf("差额（借正贷负）: %.2f 元", centsToYuan(diff)))
	}

	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(name, totalCell, cellName(4, row), totalStyle)

	wb.setMoneyStyle(name, row, 3)
	wb.setMoneyStyle(name, row, 4)

	return nil
}
