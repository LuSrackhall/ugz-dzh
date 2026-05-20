package generator

import (
	"fmt"
	"sort"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

const (
	mlMaxDetails     = 14 // 明细科目上限
	mlDetailStartCol = 8  // 明细列起始列 H
)

// mlPrintMarkCol 打印标记列号 V（14明细列右侧固定）。
func mlPrintMarkCol() int {
	return mlDetailStartCol + mlMaxDetails // V = 8 + 14 = 22
}

// mlDetailTotals 明细科目合计。
type mlDetailTotals struct {
	debit  int64
	credit int64
}

// ensureMLSheet 确保多科目明细账 Sheet 存在并已初始化标题和扩展列。
func (wb *Workbook) ensureMLSheet(general string, details []string) (string, error) {
	name := sheetNameML(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writeMLTitle(name, general, details); err != nil {
		return "", err
	}
	return name, nil
}

// writeMLTitle 写入多科目明细账标题行和列标题，固定14列 H-U。
func (wb *Workbook) writeMLTitle(sheet, general string, details []string) error {
	lastDetailCol := mlDetailStartCol + mlMaxDetails - 1 // 最末明细列号 U
	endCell, _ := excelize.CoordinatesToCellName(lastDetailCol, 1)

	title := "多科目明细账 — " + general
	wb.File.SetCellValue(sheet, "A1", title)
	wb.File.MergeCell(sheet, "A1", endCell)

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A1", endCell, titleStyle)
	wb.File.SetRowHeight(sheet, 1, 22)

	// 标准列标题 A-G
	for i, h := range glHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(sheet, cell, h)
	}
	// 扩展列标题 H-U（固定14列，空明细显示空标题）
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailStartCol + i
		cell, _ := excelize.CoordinatesToCellName(col, 2)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerEnd, _ := excelize.CoordinatesToCellName(lastDetailCol, 2)
	wb.File.SetCellStyle(sheet, "A2", headerEnd, headerStyle)

	// 列宽
	wb.File.SetColWidth(sheet, "A", "A", 12)
	wb.File.SetColWidth(sheet, "B", "B", 8)
	wb.File.SetColWidth(sheet, "C", "C", 35)
	wb.File.SetColWidth(sheet, "D", "D", 14)
	wb.File.SetColWidth(sheet, "E", "E", 14)
	wb.File.SetColWidth(sheet, "F", "F", 6)
	wb.File.SetColWidth(sheet, "G", "G", 16)
	// 列宽 — H-U 固定14列
	for i := 0; i < mlMaxDetails; i++ {
		colLetter, _ := excelize.ColumnNumberToName(mlDetailStartCol + i)
		wb.File.SetColWidth(sheet, colLetter, colLetter, 14)
	}

	return nil
}

// AppendMLEntries 将分录追加到多科目明细账 Sheet。
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	type mlGroup struct {
		entries   []voucher.Entry
		details   []string
		detailIdx map[string]int
	}
	groups := make(map[string]*mlGroup)

	for _, e := range entries {
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlGroup{detailIdx: make(map[string]int)}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		if e.DetailAccount != "" {
			if _, exists := g.detailIdx[e.DetailAccount]; !exists {
				g.detailIdx[e.DetailAccount] = len(g.details)
				g.details = append(g.details, e.DetailAccount)
			}
		}
	}

	for _, g := range groups {
		sort.Strings(g.details)
		for i, d := range g.details {
			g.detailIdx[d] = i
		}
	}

	for general, g := range groups {
		if len(g.details) == 0 {
			continue
		}
		if len(g.details) > mlMaxDetails {
			return fmt.Errorf("总账科目 %q 明细科目数 %d 超过上限 %d，请合并或拆分: %v", general, len(g.details), mlMaxDetails, g.details)
		}
		if err := wb.appendToMLSheet(general, g.entries, g.details, g.detailIdx, initials[general]); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	return nil
}

// appendToMLSheet 追加分录到指定总账科目的多科目明细账 Sheet。
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, details []string, detailIdx map[string]int, initial int64) error {
	sheet, err := wb.ensureMLSheet(general, details)
	if err != nil {
		return err
	}

	numDetails := mlMaxDetails

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	if isNew && initial != 0 {
		wb.writeMLCarryForwardRow(sheet, 3, initial, 0, 0, make([]mlDetailTotals, numDetails), "上年结转")
	}

	row, err := wb.nextDataRow(sheet)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})

	balance := initial
	var pageDebit, pageCredit int64
	pageDetails := make([]mlDetailTotals, numDetails)

	if !isNew {
		balance = wb.lastPageBalance(sheet)
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingMLPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		// 补承前页（上月遗留的孤立过次页）
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			pbDetails := wb.lastBreakDetailTotals(sheet)
			wb.writeMLCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pbDetails, carryForwardLabel)
			row++
			pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		// 页满 → 过次页 + 承前页
		if wb.rowIsPageBreak(sheet, row) {
			wb.writeMLPageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageDetails)
			row++
			wb.writeMLCarryForwardRow(sheet, row, balance, pageDebit, pageCredit, pageDetails, carryForwardLabel)
			row++
			pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				net := e.DebitCents - e.CreditCents
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+idx, row), centsToYuanStr(net))
				pageDetails[idx].debit += e.DebitCents
				pageDetails[idx].credit += e.CreditCents
			}
		}

		wb.markMLRowForPrint(sheet, row)
		row++
	}

	return nil
}

// writeMLPageBreakRow 写多科目明细账的"过次页"行，A-G 总计 + H-U 各明细本页净额。
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals) {
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), pageBreakLabel)
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
	}
}

// writeMLCarryForwardRow 写多科目明细账的"承前页"行，与过次页数据相同。
func (wb *Workbook) writeMLCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals, label string) {
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), label)
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
	}
}

// lastBreakDetailTotals 读取最后一个过次页行的各明细列净额。
func (wb *Workbook) lastBreakDetailTotals(sheet string) []mlDetailTotals {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return make([]mlDetailTotals, mlMaxDetails)
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			result := make([]mlDetailTotals, mlMaxDetails)
			for j := 0; j < mlMaxDetails; j++ {
				colIdx := mlDetailStartCol + j - 1 // 0-indexed in rows
				if colIdx < len(rows[i]) {
					if v, err := yuanStrToCents(rows[i][colIdx]); err == nil {
						if v >= 0 {
							result[j].debit = v
						} else {
							result[j].credit = -v
						}
					}
				}
			}
			return result
		}
	}
	return make([]mlDetailTotals, mlMaxDetails)
}
