package generator

import (
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMLMonthClosings 对有变化的多科目明细账 Sheet 追加月结行（本月合计/本季合计/本年累计/期末余额）。
// 明细列映射统一从 Sheet 第2行标题读取，确保月结列与标题一致。
func (wb *Workbook) WriteMLMonthClosings(
	entries []voucher.Entry,
	initials map[string]int64,
	ytdDebit, ytdCredit map[string]int64,
	qtdDebit, qtdCredit map[string]int64,
	changedSheets map[string]bool,
) error {
	type mlClosing struct {
		entries []voucher.Entry
	}
	groups := make(map[string]*mlClosing)

	for _, e := range entries {
		if e.DetailAccount == "" {
			continue
		}
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlClosing{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
	}

	for general, g := range groups {
		sheet := sheetNameML(general)
		if !changedSheets[sheet] {
			continue
		}

		// 从 Sheet 标题读取列映射
		detailIdx, details, err := wb.readMLDetailHeaders(sheet)
		if err != nil {
			return err
		}

		numDetails := mlMaxDetails

		// 计算本月各明细发生额
		mtdDetails := make([]mlDetailTotals, numDetails)
		var mtdDebit, mtdCredit int64
		for _, e := range g.entries {
			mtdDebit += e.DebitCents
			mtdCredit += e.CreditCents
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				mtdDetails[idx].debit += e.DebitCents
				mtdDetails[idx].credit += e.CreditCents
			}
		}

		row, err := wb.nextDataRowAfterBreak(sheet)
		if err != nil {
			return err
		}

		// "本月合计" 行
		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本月合计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(mtdDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(mtdCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] != "" {
				net := mtdDetails[i].debit - mtdDetails[i].credit
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuan(net))
			}
		}

		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#808080", Style: 1},
			},
		})
		lastDetailCol := mlDetailStartCol + numDetails - 1
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), monthlyStyle)
		row++

		// "本季合计" 行 — 仅季末月份（3、6、9、12）
		if isQuarterEnd(wb.Month) {
			qtDetails := make([]mlDetailTotals, numDetails)
			var qtDebit, qtCredit int64
			for _, e := range g.entries {
				qtDebit += e.DebitCents
				qtCredit += e.CreditCents
				if idx, ok := detailIdx[e.DetailAccount]; ok {
					qtDetails[idx].debit += e.DebitCents
					qtDetails[idx].credit += e.CreditCents
				}
			}
			for _, d := range details {
				if d != "" {
					key := general + "-" + d
					qtDebit += qtdDebit[key]
					qtCredit += qtdCredit[key]
				}
			}

			wb.File.SetCellValue(sheet, cellName(1, row), "")
			wb.File.SetCellValue(sheet, cellName(2, row), "")
			wb.File.SetCellValue(sheet, cellName(3, row), "本季合计")
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(qtDebit))
			wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(qtCredit))
			wb.File.SetCellValue(sheet, cellName(6, row), "")
			wb.File.SetCellValue(sheet, cellName(7, row), "")
			for i := 0; i < mlMaxDetails; i++ {
				if details[i] != "" {
					prevQt := wb.getDetailPrevQuarterTotal(general, details[i])
					net := qtDetails[i].debit - qtDetails[i].credit + prevQt
					wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuan(net))
				}
			}

			qtStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
			})
			wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), qtStyle)
			row++
		}

		// "本年累计" 行
		ytdDetails := make([]mlDetailTotals, numDetails)
		var cumDebit, cumCredit int64
		for _, e := range g.entries {
			cumDebit += e.DebitCents
			cumCredit += e.CreditCents
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				ytdDetails[idx].debit += e.DebitCents
				ytdDetails[idx].credit += e.CreditCents
			}
		}
		for _, d := range details {
			if d != "" {
				key := general + "-" + d
				cumDebit += ytdDebit[key]
				cumCredit += ytdCredit[key]
			}
		}

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(cumDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(cumCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] != "" {
				prevYtd := wb.getDetailPrevYearTotal(general, details[i])
				net := ytdDetails[i].debit - ytdDetails[i].credit + prevYtd
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuan(net))
			}
		}

		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), cumStyle)
		row++

		// "期末余额" 行 — 期初 + 本月借 - 本月贷
		endBalance := initials[general] + mtdDebit - mtdCredit
		endDir, endDisp := directionFor(endBalance, 0)

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), periodEndLabel)
		wb.File.SetCellValue(sheet, cellName(4, row), "")
		wb.File.SetCellValue(sheet, cellName(5, row), "")
		wb.File.SetCellValue(sheet, cellName(6, row), endDir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(endDisp))

		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 2},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), endStyle)
	}

	return nil
}

// getDetailPrevYearTotal 从配置余额中提取某明细科目截至上月的本年累计净额。
func (wb *Workbook) getDetailPrevYearTotal(general, detail string) int64 {
	accountPath := general + "-" + detail
	node, ok := wb.Config.Tree[accountPath]
	if !ok {
		return 0
	}
	var total int64
	for monthKey, mb := range node.Balances {
		if monthKey < wb.Month {
			total += mb.Debit - mb.Credit
		}
	}
	return total
}

// getDetailPrevQuarterTotal 从配置余额中提取某明细科目本季截至上月的累计净额。
func (wb *Workbook) getDetailPrevQuarterTotal(general, detail string) int64 {
	accountPath := general + "-" + detail
	node, ok := wb.Config.Tree[accountPath]
	if !ok {
		return 0
	}
	qStart := quarterStart(wb.Month)
	var total int64
	for monthKey, mb := range node.Balances {
		if monthKey >= qStart && monthKey < wb.Month {
			total += mb.Debit - mb.Credit
		}
	}
	return total
}
