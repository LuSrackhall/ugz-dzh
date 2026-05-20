package generator

import (
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMLMonthClosings 对有变化的多科目明细账 Sheet 追加月结行（本月合计/本季合计/本年累计/期末余额）。
// details 参数从 entries 中按总账科目提取。
func (wb *Workbook) WriteMLMonthClosings(
	entries []voucher.Entry,
	initials map[string]int64,
	ytdDebit, ytdCredit map[string]int64,
	qtdDebit, qtdCredit map[string]int64,
	changedSheets map[string]bool,
) error {
	// 按总账科目收集分录
	type mlClosing struct {
		entries     []voucher.Entry
		details     []string
		detailIdx   map[string]int
	}
	groups := make(map[string]*mlClosing)

	for _, e := range entries {
		if e.DetailAccount == "" {
			continue
		}
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlClosing{detailIdx: make(map[string]int)}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		if _, exists := g.detailIdx[e.DetailAccount]; !exists {
			g.detailIdx[e.DetailAccount] = len(g.details)
			g.details = append(g.details, e.DetailAccount)
		}
	}

	for general, g := range groups {
		sheet := sheetNameML(general)
		if !changedSheets[sheet] {
			continue
		}

		numDetails := mlMaxDetails

		// 计算本月各明细发生额
		mtdDetails := make([]mlDetailTotals, numDetails)
		var mtdDebit, mtdCredit int64
		for _, e := range g.entries {
			mtdDebit += e.DebitCents
			mtdCredit += e.CreditCents
			if idx, ok := g.detailIdx[e.DetailAccount]; ok {
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
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(mtdDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(mtdCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i, dt := range mtdDetails {
			net := dt.debit - dt.credit
			wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
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
				if idx, ok := g.detailIdx[e.DetailAccount]; ok {
					qtDetails[idx].debit += e.DebitCents
					qtDetails[idx].credit += e.CreditCents
				}
			}
			// 加上本季此前月份（从 qtd maps 中取父级，明细列从配置余额取）
			qtDebit += qtdDebit[general]
			qtCredit += qtdCredit[general]

			wb.File.SetCellValue(sheet, cellName(1, row), "")
			wb.File.SetCellValue(sheet, cellName(2, row), "")
			wb.File.SetCellValue(sheet, cellName(3, row), "本季合计")
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(qtDebit))
			wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(qtCredit))
			wb.File.SetCellValue(sheet, cellName(6, row), "")
			wb.File.SetCellValue(sheet, cellName(7, row), "")
			for i := range g.details {
				// 明细列本季累计 = 本月 + 此前季度（从配置余额取）
				detailName := g.details[i]
				prevQt := wb.getDetailPrevQuarterTotal(general, detailName)
				net := qtDetails[i].debit - qtDetails[i].credit + prevQt
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
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
			if idx, ok := g.detailIdx[e.DetailAccount]; ok {
				ytdDetails[idx].debit += e.DebitCents
				ytdDetails[idx].credit += e.CreditCents
			}
		}
		cumDebit += ytdDebit[general]
		cumCredit += ytdCredit[general]

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(cumDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(cumCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i := range g.details {
			detailName := g.details[i]
			prevYtd := wb.getDetailPrevYearTotal(general, detailName)
			net := ytdDetails[i].debit - ytdDetails[i].credit + prevYtd
			wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
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
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(endDisp))
		// H-U 留空 — 明细列期末余额无会计意义

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
