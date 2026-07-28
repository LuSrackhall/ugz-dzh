package generator

import (
	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// WriteMLMonthClosings 对有变化的多科目明细账 Sheet 追加月结行（本月合计/本季合计/本年累计/期末余额）。
// 每写一行前检查页容量，满了就过次页翻页，保证每页恰好20数据行+1过次页行。
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

	mlSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.MLSuppressAccounts {
		mlSuppress[a] = true
	}

	for _, e := range entries {
		if e.DetailAccount == "" {
			continue
		}
		if mlSuppress[e.GeneralAccount] {
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

		detailIdx, details, err := wb.readMLDetailHeaders(sheet)
		if err != nil {
			return err
		}

		numDetails := mlMaxDetails

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

		row, err := wb.mlNextDataRow(sheet)
		if err != nil {
			return err
		}

		lay := mlLayout()

		// 如果当前位置到达过次页位置，先翻页再写月结
		// 用当前页起始 + 20 计算，不依赖 mlPageStartRow（后者可能找到后续真实过次页）
		bRows, _ := wb.File.GetRows(sheet)
		curPageStart := mlFirstDataPageStart() + lay.DataStartRow
		for i := len(bRows) - 1; i >= 0; i-- {
			if mlHasPageBreakAt(bRows[i], lay) && !mlIsStructuralBreak(bRows[i], lay) {
				curPageStart = i + 2 + lay.DataStartRow
				break
			}
		}
		initBreakPos := curPageStart + pageSize
		if row >= initBreakPos {
			bal := wb.mlLastPageBalance(sheet)
			pageNum := 1
			for _, br := range bRows {
				if mlHasPageBreakAt(br, lay) && !mlIsStructuralBreak(br, lay) {
					pageNum++
				}
			}
			pageNum++
			wb.writeMLPageBreakRow(sheet, initBreakPos, bal, 0, 0, make([]mlDetailTotals, numDetails))
			wb.writeMLPageHeader(sheet, initBreakPos+1, pageNum, pageNum, general, true, true)
			cfRow := initBreakPos + 1 + lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, cfRow, bal, 0, 0, make([]mlDetailTotals, numDetails), carryForwardLabel)
			row = cfRow + 1
		}

		// 检查是否需要过次页翻页（月结行逐行检查）
		// 用当前页起始 + 20 计算过次页位置
		findPageBreakRow := func() int {
			bRows, _ := wb.File.GetRows(sheet)
			ps := mlFirstDataPageStart() + lay.DataStartRow
			for i := len(bRows) - 1; i >= 0; i-- {
				if mlHasPageBreakAt(bRows[i], lay) && !mlIsStructuralBreak(bRows[i], lay) {
					ps = i + 2 + lay.DataStartRow
					break
				}
			}
			return ps + pageSize
		}
		mlCheckPageBreak := func(r int) int {
			pbRow := findPageBreakRow()
			if r < pbRow {
				return r
			}
			// 到达过次页位置 → 翻页
			bal := wb.mlLastPageBalance(sheet)
			pageNum := 1
			bRows, _ := wb.File.GetRows(sheet)
			for _, br := range bRows {
				if mlHasPageBreakAt(br, lay) && !mlIsStructuralBreak(br, lay) {
					pageNum++
				}
			}
			pageNum++
			wb.writeMLPageBreakRow(sheet, pbRow, bal, 0, 0, make([]mlDetailTotals, numDetails))
			wb.writeMLPageHeader(sheet, pbRow+1, pageNum, pageNum, general, true, true)
			cfRow := pbRow + 1 + lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, cfRow, bal, 0, 0, make([]mlDetailTotals, numDetails), carryForwardLabel)
			return cfRow + 1
		}

		// 本月合计
		row = mlCheckPageBreak(row)
		wb.writeMLClosingRow(sheet, row, "本月合计", mtdDebit, mtdCredit, mtdDetails, details, lay)
		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:   &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 1}},
		})
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, row), mlCellName(lay.BackStartCol+mlOffBalance, row), monthlyStyle)
		wb.File.SetCellStyle(sheet, mlCellName(mlDetailCol(lay, 4), row), mlCellName(mlDetailCol(lay, mlMaxDetails-1), row), monthlyStyle)
		row++

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
			for i := range qtDetails {
				if details[i] != "" {
					prevQt := wb.getDetailPrevQuarterTotal(general, details[i])
					net := qtDetails[i].debit - qtDetails[i].credit + prevQt
					if net >= 0 {
						qtDetails[i].debit = net
						qtDetails[i].credit = 0
					} else {
						qtDetails[i].debit = 0
						qtDetails[i].credit = -net
					}
				}
			}

			row = mlCheckPageBreak(row)
			wb.writeMLClosingRow(sheet, row, "本季合计", qtDebit, qtCredit, qtDetails, details, lay)
			qtStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
			})
			wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, row), mlCellName(lay.BackStartCol+mlOffBalance, row), qtStyle)
			wb.File.SetCellStyle(sheet, mlCellName(mlDetailCol(lay, 4), row), mlCellName(mlDetailCol(lay, mlMaxDetails-1), row), qtStyle)
			row++
		}

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
		for i := range ytdDetails {
			if details[i] != "" {
				prevYtd := wb.getDetailPrevYearTotal(general, details[i])
				net := ytdDetails[i].debit - ytdDetails[i].credit + prevYtd
				if net >= 0 {
					ytdDetails[i].debit = net
					ytdDetails[i].credit = 0
				} else {
					ytdDetails[i].debit = 0
					ytdDetails[i].credit = -net
				}
			}
		}

		row = mlCheckPageBreak(row)
		wb.writeMLClosingRow(sheet, row, "本年累计", cumDebit, cumCredit, ytdDetails, details, lay)
		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:   &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
		})
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, row), mlCellName(lay.BackStartCol+mlOffBalance, row), cumStyle)
		wb.File.SetCellStyle(sheet, mlCellName(mlDetailCol(lay, 4), row), mlCellName(mlDetailCol(lay, mlMaxDetails-1), row), cumStyle)
		row++

		// 期末余额
		row = mlCheckPageBreak(row)
		endBalance := initials[general] + mtdDebit - mtdCredit
		endDir, endDisp := directionFor(endBalance, 0)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffMonth, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDay, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouChar, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouNum, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), periodEndLabel)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, row), "")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, row), endDir)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, row), centsToYuan(endDisp))
		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:   &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{{Type: "bottom", Color: "#000000", Style: 2}},
		})
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, row), mlCellName(mlDetailCol(lay, mlMaxDetails-1), row), endStyle)
		wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffBalance)

		// 月末补齐当前页结构过次页（计算方式和 padMLPage 一致，避免重复写入）
		ps := mlFirstDataPageStart() + lay.DataStartRow
		bRowsFinal, _ := wb.File.GetRows(sheet)
		for i := len(bRowsFinal) - 1; i >= 0; i-- {
			if mlHasPageBreakAt(bRowsFinal[i], lay) && !mlIsStructuralBreak(bRowsFinal[i], lay) {
				ps = i + 2 + lay.DataStartRow
				break
			}
		}
		brRow := ps + pageSize
		brCell := mlCellName(lay.BackStartCol+mlOffSummary, brRow)
		brVal, _ := wb.File.GetCellValue(sheet, brCell)
		if brVal == "" {
			wb.File.SetCellValue(sheet, brCell, pageBreakLabel)
			redBr, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Color: "CC0000", Size: 10, Bold: true},
			})
			wb.File.SetCellStyle(sheet, brCell, brCell, redBr)
		}
	}

		return nil
}

// FinalizeMLPages 补齐所有多科目明细账 Sheet 的最后一页。
// 每页固定20数据行+1过次页行，数据不满时用空行补齐。
func (wb *Workbook) FinalizeMLPages() {
	for _, name := range wb.File.GetSheetList() {
		if len(name) < len(sheetPrefixML) || name[:len(sheetPrefixML)] != sheetPrefixML {
			continue
		}
		general := name[len(sheetPrefixML):]
		wb.padMLPage(name, general)
	}
}
// 如果当前页已有真实过次页（翻页），则在新页上补齐。
// 如果当前页不满20行，用空行补齐后写结构过次页。
// padMLPage 补齐当前页的结构过次页。
// 空行不写入任何内容（SetCellValue 和 SetCellStyle 都会被 GetRows 检测到）。
func (wb *Workbook) padMLPage(sheet string, general string) {
	lay := mlLayout()
	rows, _ := wb.File.GetRows(sheet)

	pageStart := mlFirstDataPageStart() + lay.DataStartRow
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			pageStart = i + 2 + lay.DataStartRow
			break
		}
	}

	lastDataIdx := mlLastDataBeforeBreak(rows, lay)
	usedRows := 0
	if lastDataIdx >= 0 {
		usedRows = lastDataIdx + 1 - pageStart + 1
	}
	if usedRows < 0 {
		return // 无数据，跳过
	}

	// 页未满时才写入结构过次页（页满时月结翻页逻辑已写入）
	structRow := pageStart + pageSize
	if usedRows >= 0 && usedRows < pageSize {
		structCell := mlCellName(lay.BackStartCol+mlOffSummary, structRow)
		structVal, _ := wb.File.GetCellValue(sheet, structCell)
		if structVal == "" {
			wb.File.SetCellValue(sheet, structCell, pageBreakLabel)
			redStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Color: "CC0000", Size: 10, Bold: true},
			})
			wb.File.SetCellStyle(sheet, structCell, structCell, redStyle)
		}
	}

	// PaperN Back 尾部占位：反面空白占位表（Back 侧标题，无页码）
	wb.writeMLPageHeader(sheet, structRow+1, 0, 0, general, true, false)
	// 占位页底部结构过次页
	pnRow := structRow + 1 + lay.DataStartRow + pageSize
	pnCell := mlCellName(lay.BackStartCol+mlOffSummary, pnRow)
	pnVal, _ := wb.File.GetCellValue(sheet, pnCell)
	if pnVal == "" {
		wb.File.SetCellValue(sheet, pnCell, pageBreakLabel)
		redS, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Color: "CC0000", Size: 10, Bold: true},
		})
		wb.File.SetCellStyle(sheet, pnCell, pnCell, redS)
	}
}

// writeMLClosingRow 将月结行写入双面：Back 侧（基础列+明细1~4），Front 侧（明细5~14）。
func (wb *Workbook) writeMLClosingRow(sheet string, row int, label string, debit, credit int64, details []mlDetailTotals, detailsList []string, lay layout.MLLayout) {
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffMonth, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDay, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouChar, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouNum, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), label)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, row), centsToYuan(debit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, row), centsToYuan(credit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, row), "")
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffDebit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffCredit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffBalance)

	for i := 0; i < 4 && i < len(details); i++ {
		if i < len(detailsList) && detailsList[i] != "" {
			net := details[i].debit - details[i].credit
			col := mlDetailCol(lay, i)
			wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
			wb.setMoneyStyle(sheet, row, col)
		}
	}

	for i := 4; i < len(details); i++ {
		if i < len(detailsList) && detailsList[i] != "" {
			net := details[i].debit - details[i].credit
			col := mlDetailCol(lay, i)
			wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
			wb.setMoneyStyle(sheet, row, col)
		}
	}
}

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
