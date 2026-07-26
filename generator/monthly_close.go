package generator

import (
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMonthClosings 对有变化的 Sheet 追加"本月合计"、"本季合计"（仅季末）、"本年累计"和"期末余额"行。
func (wb *Workbook) WriteMonthClosings(activity map[string]Activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit map[string]int64, initials map[string]int64, changedSheets map[string]bool) error {
	lay := glLayout()
	for account, act := range activity {
		sheet := sheetNameGL(account)
		if !changedSheets[sheet] {
			continue
		}

		// 计算当前 Sheet 的最后一页页码
		rows, _ := wb.File.GetRows(sheet)
		pageNum := 1
		for _, r := range rows {
			if hasPageBreakAt(r, lay) {
				pageNum++
			}
		}
		row, err := wb.nextDataRowAfterBreak(sheet)
		if err != nil {
			return err
		}

		// "本月合计" 行
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本月合计")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(act.Debit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(act.Credit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), "")

		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol, row), cellName(dataCol(lay, pageNum, 8), row), monthlyStyle)

		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))

		row++

		// "本季合计" 行 — 仅季末月份（3、6、9、12）
		if isQuarterEnd(wb.Month) {
			qtDebit := (qtdDebit[account]) + act.Debit
			qtCredit := (qtdCredit[account]) + act.Credit

			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本季合计")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(qtDebit))
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(qtCredit))
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), "")

			qtStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
			})
			wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol, row), cellName(dataCol(lay, pageNum, 8), row), qtStyle)

			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))

			row++
		}

		// "本年累计" 行
		cumDebit := (ytdDebit[account]) + act.Debit
		cumCredit := (ytdCredit[account]) + act.Credit

		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(cumDebit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(cumCredit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), "")

		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol, row), cellName(dataCol(lay, pageNum, 8), row), cumStyle)

		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))

		row++

		// "期末余额" 行 — 期初 + 本月借 - 本月贷
		endBalance := initials[account] + act.Debit - act.Credit
		endDir, endDisp := directionFor(endBalance, 0)

		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), periodEndLabel)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), endDir)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(endDisp))

		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 2},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol, row), cellName(dataCol(lay, pageNum, 8), row), endStyle)

			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
		}

	return nil
}

// nextDataRowAfterBreak 返回 Sheet 中最后一行之后的下一行。
// 若最后一行为孤立过次页（无承前页跟随），则返回过次页所在行（关账行直接覆盖过次页）。
func (wb *Workbook) nextDataRowAfterBreak(sheet string) (int, error) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}
	// 孤立过次页：最后一行为过次页 → 覆盖之（过次页不上承前页，关账行替代）
	if wb.lastRowIsOrphanBreak(sheet) {
		return len(rows), nil
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

// ExtractQuarterlyTotals 从配置中提取本季度截至上月的各科目本季累计借贷。
func (wb *Workbook) ExtractQuarterlyTotals(accounts []string) (map[string]int64, map[string]int64) {
	qtdDebit := make(map[string]int64)
	qtdCredit := make(map[string]int64)
	qStart := quarterStart(wb.Month)

	for _, account := range accounts {
		node, ok := wb.Config.Tree[account]
		if !ok {
			continue
		}
		for monthKey, mb := range node.Balances {
			if monthKey >= qStart && monthKey < wb.Month {
				qtdDebit[account] += mb.Debit
				qtdCredit[account] += mb.Credit
			}
		}
	}

	return qtdDebit, qtdCredit
}

// isQuarterEnd 判断月份是否为季末（3、6、9、12）。
func isQuarterEnd(month string) bool {
	return month[5:] == "03" || month[5:] == "06" || month[5:] == "09" || month[5:] == "12"
}

// quarterStart 返回当前月份所在季度的起始月份。
func quarterStart(month string) string {
	yy := month[:4]
	switch month[5:] {
	case "01", "02", "03":
		return yy + "-01"
	case "04", "05", "06":
		return yy + "-04"
	case "07", "08", "09":
		return yy + "-07"
	default:
		return yy + "-10"
	}
}
