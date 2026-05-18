package generator

import (
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMonthClosings 对有变化的 Sheet 追加"本月合计"、"本年累计"和"期末余额"行。
// activity: 各科目全路径 → Activity (当月借/贷合计，分)。
// ytdDebit/ytdCredit: 各科目全路径 → 截至上月的累计借/贷（分）。
// initials: 各科目全路径 → 本月期初余额（分）。
// changedSheets: 当月有数据写入的 Sheet 名称集合。
func (wb *Workbook) WriteMonthClosings(activity map[string]Activity, ytdDebit, ytdCredit map[string]int64, initials map[string]int64, changedSheets map[string]bool) error {
	for account, act := range activity {
		sheet := sheetNameGL(account)
		if !changedSheets[sheet] {
			continue
		}

		row, err := wb.nextDataRowAfterBreak(sheet)
		if err != nil {
			return err
		}

		// "本月合计" 行
		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本月合计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(act.Debit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(act.Credit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")

		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), monthlyStyle)
		row++

		// "本年累计" 行
		cumDebit := (ytdDebit[account]) + act.Debit
		cumCredit := (ytdCredit[account]) + act.Credit

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(cumDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(cumCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")

		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), cumStyle)
		row++

		// "期末余额" 行 — 期初 + 本月借 - 本月贷
		endBalance := initials[account] + act.Debit - act.Credit
		endDir, endDisp := directionFor(endBalance, 0)

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), periodEndLabel)
		wb.File.SetCellValue(sheet, cellName(4, row), "")
		wb.File.SetCellValue(sheet, cellName(5, row), "")
		wb.File.SetCellValue(sheet, cellName(6, row), endDir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(endDisp))

		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 2},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), endStyle)
	}

	return nil
}

// nextDataRowAfterBreak 返回 Sheet 中最后一行之后的下一行。
// 用于在追加完所有数据后再追加月结行。
func (wb *Workbook) nextDataRowAfterBreak(sheet string) (int, error) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}
	return len(rows) + 1, nil
}

// Activity 某一科目在当月的借/贷合计。
type Activity struct {
	Debit  int64
	Credit int64
}

// ComputeActivity 从分录计算各科目的当月发生额。
func ComputeActivity(entries []voucher.Entry) map[string]Activity {
	act := make(map[string]Activity)
	for _, e := range entries {
		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		a := act[path]
		a.Debit += e.DebitCents
		a.Credit += e.CreditCents
		act[path] = a
	}
	return act
}

// CollectChangedSheets 返回当期有分录变动的 Sheet 名称集合。
func CollectChangedSheets(entries []voucher.Entry) map[string]bool {
	sheets := make(map[string]bool)
	for _, e := range entries {
		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		sheets[sheetNameGL(path)] = true
	}
	return sheets
}

// ExtractYtdTotals 从配置中提取截至上月的各科目本年累计借贷。
// 返回 ytdDebit, ytdCredit。
func (wb *Workbook) ExtractYtdTotals(accounts []string) (map[string]int64, map[string]int64) {
	ytdDebit := make(map[string]int64)
	ytdCredit := make(map[string]int64)

	for _, account := range accounts {
		node, ok := wb.Config.Tree[account]
		if !ok {
			continue
		}
		for monthKey, mb := range node.Balances {
			if monthKey < wb.Month {
				ytdDebit[account] += mb.Debit
				ytdCredit[account] += mb.Credit
			}
		}
	}

	return ytdDebit, ytdCredit
}
