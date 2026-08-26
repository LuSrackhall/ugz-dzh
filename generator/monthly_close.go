package generator

import (
	"strings"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMonthClosings 对有变化的 Sheet 追加"本月合计"、"本季合计"（仅季末）、"本年累计"和"期末余额"行。
func (wb *Workbook) WriteMonthClosings(activity map[string]Activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit map[string]int64, initials map[string]int64, changedSheets map[string]bool) error {
	lay := glLayout()

	// M4 无活动月份补月结行：期初（上月末余额）≠0 但当月无分录、且存在 GL Sheet 的科目，
	// 补"本月合计 0/0 + 本年累计 + 期末余额=期初"行，使账本逐月连续清晰。
	// 使用副本扩展，不修改入参（不影响 MergeGL/ML 后续处理）。
	extendedActivity := make(map[string]Activity, len(activity))
	extendedChanged := make(map[string]bool, len(changedSheets))
	for k, v := range activity {
		extendedActivity[k] = v
		extendedChanged[sheetNameGL(k)] = true
	}
	for account, init := range initials {
		if init == 0 {
			continue
		}
		if _, hasAct := extendedActivity[account]; hasAct {
			continue
		}
		sheet := sheetNameGL(account)
		if !wb.hasSheet(sheet) {
			continue
		}
		extendedActivity[account] = Activity{}
		extendedChanged[sheet] = true
	}

	for account, act := range extendedActivity {
		sheet := sheetNameGL(account)
		if !extendedChanged[sheet] {
			continue
		}

		pageNum := wb.getPageNum(sheet)
		row, err := wb.nextDataRowAfterBreak(sheet)
		if err != nil {
			return err
		}

		balance := initials[account] + act.Debit - act.Credit
		var closingDebit, closingCredit int64

		// 检查页容量，满了就翻页
		checkBreak := func() {
			pageStart := wb.pageStartRow(sheet)
			if row-pageStart >= pageSize {
				wb.writePageBreakRow(sheet, row, balance, closingDebit, closingCredit, pageNum)
				row++
				pageNum = wb.getPageNum(sheet)
				row += lay.BottomMarginRows + lay.TopMarginRows
				marginStart := row - lay.BottomMarginRows - lay.TopMarginRows
				for d := marginStart; d < row; d++ {
					h := 19.0 // 下边距（与 GL 其他翻页统一）
					if d >= row-lay.TopMarginRows {
						h = 16.0 // 下页上边距（与 GL 其他翻页统一）
					}
					wb.File.SetRowHeight(sheet, d, h)
				}
				wb.writePageHeader(sheet, row, pageNum, account)
				row += lay.DataStartRow
				wb.writeCarryForwardRow(sheet, row, balance, closingDebit, closingCredit, pageNum)
				row++
				closingDebit = 0
				closingCredit = 0
			}
		}

		// "本月合计" 行
		checkBreak()
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本月合计")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(act.Debit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(act.Credit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")
		closingDebit += act.Debit
		closingCredit += act.Credit

		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#006100", Style: 1},
				{Type: "right", Color: "#006100", Style: 1},
				{Type: "bottom", Color: "#006100", Style: 1},
				{Type: "left", Color: "#006100", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
			cellName(dataCol(lay, pageNum, glColCount-1), row), monthlyStyle)

		// 应用每5行底边加粗样式（基于固定页结构）
		isThickRow := glRowInPage(lay, row)%5 == 0

		if isThickRow {
			monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Border: []excelize.Border{
					{Type: "top", Color: "#006100", Style: 1},
					{Type: "right", Color: "#006100", Style: 1},
					{Type: "bottom", Color: "#006100", Style: 2},
					{Type: "left", Color: "#006100", Style: 1},
				},
			})
			wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
				cellName(dataCol(lay, pageNum, glColCount-1), row), monthlyStyle)
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColDebit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColCredit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColBalance))
		} else {
			monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Border: []excelize.Border{
					{Type: "top", Color: "#006100", Style: 1},
					{Type: "right", Color: "#006100", Style: 1},
					{Type: "bottom", Color: "#006100", Style: 1},
					{Type: "left", Color: "#006100", Style: 1},
				},
			})
			wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
				cellName(dataCol(lay, pageNum, glColCount-1), row), monthlyStyle)
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColDebit))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColCredit))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))
		}

		row++

		// "本季合计" 行 — 仅季末月份（3、6、9、12）
		if isQuarterEnd(wb.Month) {
			qtDebit := (qtdDebit[account]) + act.Debit
			qtCredit := (qtdCredit[account]) + act.Credit

			checkBreak()
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本季合计")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(qtDebit))
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(qtCredit))
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")

			qtStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Border: []excelize.Border{
					{Type: "top", Color: "#006100", Style: 1},
					{Type: "right", Color: "#006100", Style: 1},
					{Type: "bottom", Color: "#006100", Style: 1},
					{Type: "left", Color: "#006100", Style: 1},
				},
			})
			wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
				cellName(dataCol(lay, pageNum, glColCount-1), row), qtStyle)

			// 应用每5行底边加粗样式（基于固定页结构）
			if glRowInPage(lay, row)%5 == 0 {
				ts2, _ := wb.File.NewStyle(&excelize.Style{
					Font: &excelize.Font{Bold: true, Size: 10},
					Border: []excelize.Border{
						{Type: "top", Color: "#006100", Style: 1},
						{Type: "right", Color: "#006100", Style: 1},
						{Type: "bottom", Color: "#006100", Style: 2},
						{Type: "left", Color: "#006100", Style: 1},
					},
				})
				wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
					cellName(dataCol(lay, pageNum, glColCount-1), row), ts2)
				wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColDebit))
				wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColCredit))
				wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColBalance))
			} else {
				wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColDebit))
				wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColCredit))
				wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))
			}

			row++
		}

		// "本年累计" 行
		cumDebit := (ytdDebit[account]) + act.Debit
		cumCredit := (ytdCredit[account]) + act.Credit

		checkBreak()
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(cumDebit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(cumCredit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")

		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#006100", Style: 1},
				{Type: "right", Color: "#006100", Style: 1},
				{Type: "bottom", Color: "#006100", Style: 1},
				{Type: "left", Color: "#006100", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
			cellName(dataCol(lay, pageNum, glColCount-1), row), cumStyle)

		// 应用每5行底边加粗样式（基于固定页结构）
		if glRowInPage(lay, row)%5 == 0 {
			ts3, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Border: []excelize.Border{
					{Type: "top", Color: "#006100", Style: 1},
					{Type: "right", Color: "#006100", Style: 1},
					{Type: "bottom", Color: "#006100", Style: 2},
					{Type: "left", Color: "#006100", Style: 1},
				},
			})
			wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
				cellName(dataCol(lay, pageNum, glColCount-1), row), ts3)
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColDebit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColCredit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColBalance))
		} else {
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColDebit))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColCredit))
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))
		}

		row++

		// "期末余额" 行 — 期初 + 本月借 - 本月贷
		checkBreak()
		endBalance := initials[account] + act.Debit - act.Credit
		endDir, endDisp := directionFor(endBalance, 0)

		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), periodEndLabel)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), endDir)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), centsToYuan(endDisp))

		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#006100", Style: 1},
				{Type: "right", Color: "#006100", Style: 1},
				{Type: "bottom", Color: "#006100", Style: 1},
				{Type: "left", Color: "#006100", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
			cellName(dataCol(lay, pageNum, glColCount-1), row), endStyle)

		// 应用每5行底边加粗样式（基于固定页结构）
		if glRowInPage(lay, row)%5 == 0 {
			ts4, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Border: []excelize.Border{
					{Type: "top", Color: "#006100", Style: 1},
					{Type: "right", Color: "#006100", Style: 1},
					{Type: "bottom", Color: "#006100", Style: 2},
					{Type: "left", Color: "#006100", Style: 1},
				},
			})
			wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
				cellName(dataCol(lay, pageNum, glColCount-1), row), ts4)
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColDebit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColCredit))
			wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColBalance))
		} else {
			wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))
		}
	}

	return nil
}

// nextDataRowAfterBreak 定位月末结账行的写入起点。
// 从下往上找最后一条有实际内容的行（跳过空行和结构过次页），返回其下一行。
func (wb *Workbook) nextDataRowAfterBreak(sheet string) (int, error) {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) <= 2 {
		return lay.DataStartRow + 1 + lay.TopMarginRows, nil
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) == 0 {
			continue
		}
		if isTemplateBreak(rows[i], lay) {
			continue
		}
		return i + 2, nil
	}
	return lay.DataStartRow + 1, nil
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
			// 审计 H1：year-close 会跨年保留 Balances，必须按同年过滤，
			// 否则新年度的"本年累计"会把上年全年发生额加进来。
			if strings.HasPrefix(monthKey, wb.Month[:5]) && monthKey < wb.Month {
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
			// 同年过滤（审计 H1，与 ytd 保持一致；qStart 已在年内，双保险）
			if strings.HasPrefix(monthKey, wb.Month[:5]) && monthKey >= qStart && monthKey < wb.Month {
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
