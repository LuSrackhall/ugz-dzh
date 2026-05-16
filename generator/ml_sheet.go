package generator

import (
	"fmt"
	"sort"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

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

// writeMLTitle 写入多科目明细账标题行和列标题。
func (wb *Workbook) writeMLTitle(sheet, general string, details []string) error {
	lastCol := 7 + len(details)
	endCell, _ := excelize.CoordinatesToCellName(lastCol, 1)

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
	// 扩展列标题 H-U
	for i, detail := range details {
		col := 8 + i
		cell, _ := excelize.CoordinatesToCellName(col, 2)
		wb.File.SetCellValue(sheet, cell, detail)
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerEnd, _ := excelize.CoordinatesToCellName(lastCol, 2)
	wb.File.SetCellStyle(sheet, "A2", headerEnd, headerStyle)

	// 列宽
	wb.File.SetColWidth(sheet, "A", "A", 12)
	wb.File.SetColWidth(sheet, "B", "B", 8)
	wb.File.SetColWidth(sheet, "C", "C", 35)
	wb.File.SetColWidth(sheet, "D", "D", 14)
	wb.File.SetColWidth(sheet, "E", "E", 14)
	wb.File.SetColWidth(sheet, "F", "F", 6)
	wb.File.SetColWidth(sheet, "G", "G", 16)
	for i := range details {
		colLetter, _ := excelize.ColumnNumberToName(8 + i)
		wb.File.SetColWidth(sheet, colLetter, colLetter, 14)
	}

	return nil
}

// mlDetailTotals 明细科目合计。
type mlDetailTotals struct {
	debit  int64
	credit int64
}

// AppendMLEntries 将分录追加到多科目明细账 Sheet。
// 仅对存在明细科目的总账科目创建多科目明细账。
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry) error {
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

	// 明细科目按字母排序以保持列稳定
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
		if err := wb.appendToMLSheet(general, g.entries, g.details, g.detailIdx); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	return nil
}

// appendToMLSheet 追加分录到指定总账科目的多科目明细账 Sheet。
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, details []string, detailIdx map[string]int) error {
	sheet, err := wb.ensureMLSheet(general, details)
	if err != nil {
		return err
	}

	// 计算父级汇总和明细列合计
	dt := make([]mlDetailTotals, len(details))
	var grandDebit, grandCredit int64
	for _, e := range entries {
		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				dt[idx].debit += e.DebitCents
				dt[idx].credit += e.CreditCents
			}
		}
		grandDebit += e.DebitCents
		grandCredit += e.CreditCents
	}

	// 写入父级汇总行
	row, err := wb.nextDataRow(sheet)
	if err != nil {
		return err
	}

	wb.writeMLParentSummary(sheet, row, general, grandDebit, grandCredit, dt)
	wb.markRowForPrint(sheet, row)
	row++

	// 按日期、凭证号排序
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})

	// 获取上页过次页余额
	balance := wb.lastPageBreakBalance(sheet)
	if !wb.pageHasBreakRow(sheet) {
		wb.markExistingPageForPrint(sheet)
	}

	numDetails := len(details)
	for _, e := range entries {
		if wb.rowIsPageBreak(sheet, row) {
			wb.writeMLPageBreakRow(sheet, row, balance, numDetails)
			row++
		}

		balance = balance + e.DebitCents - e.CreditCents
		dir, dispBal := directionFor(balance, 0)

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

		// 填入对应明细列金额（净额）
		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				col := 8 + idx
				net := e.DebitCents - e.CreditCents
				wb.File.SetCellValue(sheet, cellName(col, row), centsToYuanStr(net))
			}
		}

		wb.markRowForPrint(sheet, row)

		dataRows := row - wb.pageStartRow(sheet) + 1
		if dataRows >= pageSize {
			wb.writeMLPageBreakRow(sheet, row+1, balance, numDetails)
			row++
		}

		row++
	}

	return nil
}

// writeMLParentSummary 写入多科目明细账的父级汇总行。
func (wb *Workbook) writeMLParentSummary(sheet string, row int, general string, grandDebit, grandCredit int64, dt []mlDetailTotals) {
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), general+" 汇总")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(grandDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(grandCredit))
	dir, dispBal := directionFor(grandDebit, grandCredit)
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

	for i, t := range dt {
		col := 8 + i
		net := t.debit - t.credit
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuanStr(net))
	}

	numDetails := len(dt)
	parentStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
	})
	endCol := 7 + numDetails
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(endCol, row), parentStyle)
}

// writeMLPageBreakRow 写入多科目明细账的过次页行（含扩展列）。
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, numDetails int) {
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), pageBreakLabel)
	for col := 2; col <= 7+numDetails; col++ {
		wb.File.SetCellValue(sheet, cellName(col, row), "")
	}
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))
}
