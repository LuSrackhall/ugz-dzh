package generator

import (
	"fmt"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// GL headers
var glHeaders = []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}

const carryForwardLabel = "承前页"

// ensureGLSheet 确保总分类账 Sheet 存在并已初始化标题。
func (wb *Workbook) ensureGLSheet(account string) (string, error) {
	name := sheetNameGL(account)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writeGLTitle(name); err != nil {
		return "", err
	}
	return name, nil
}

// writeGLTitle 写入总分类账标题行和列标题。
func (wb *Workbook) writeGLTitle(sheet string) error {
	account := sheet[len(sheetPrefixGL):]
	title := "总分类账 — " + account
	wb.File.SetCellValue(sheet, "A1", title)
	wb.File.MergeCell(sheet, "A1", "G1")

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A1", "G1", titleStyle)
	wb.File.SetRowHeight(sheet, 1, 22)

	for i, h := range glHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(sheet, cell, h)
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerCell, _ := excelize.CoordinatesToCellName(1, 2)
	endCell, _ := excelize.CoordinatesToCellName(7, 2)
	wb.File.SetCellStyle(sheet, headerCell, endCell, headerStyle)

	wb.File.SetColWidth(sheet, "A", "A", 12)
	wb.File.SetColWidth(sheet, "B", "B", 8)
	wb.File.SetColWidth(sheet, "C", "C", 35)
	wb.File.SetColWidth(sheet, "D", "D", 14)
	wb.File.SetColWidth(sheet, "E", "E", 14)
	wb.File.SetColWidth(sheet, "F", "F", 6)
	wb.File.SetColWidth(sheet, "G", "G", 16)

	return nil
}

// nextDataRow 返回 Sheet 中下一个可用数据行号。
// 若最后一行为孤立过次页（无承前页跟随），返回过次页+1 供承前页写入。
func (wb *Workbook) nextDataRow(sheet string) (int, error) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}

	if len(rows) < 3 {
		return 3, nil
	}

	// 找最近的过次页（在摘要列 C）
	lastBreak := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			lastBreak = i + 1
			break
		}
	}

	// 过次页为最后一行 → 下一行供承前页
	if lastBreak > 0 && lastBreak == len(rows) {
		return lastBreak + 1, nil
	}

	// 过次页+1 为最后一行（已是承前页） → 返回承前页之后
	if lastBreak > 0 && lastBreak+1 == len(rows) {
		return len(rows) + 1, nil
	}

	dataStart := lastBreak + 1
	if dataStart == 1 {
		dataStart = 3
	}
	usedDataRows := len(rows) - dataStart + 1

	if usedDataRows >= pageSize {
		return len(rows) + 1, nil
	}

	return len(rows) + 1, nil
}

// AppendEntries 追加当月分录到对应的总分类账 Sheet。
func (wb *Workbook) AppendEntries(entries []voucher.Entry, initials map[string]int64) error {
	type entryGroup struct {
		entries []voucher.Entry
		initial int64
	}
	groups := make(map[string]*entryGroup)

	for _, e := range entries {
		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		g, ok := groups[path]
		if !ok {
			g = &entryGroup{initial: initials[path]}
			groups[path] = g
		}
		g.entries = append(g.entries, e)
	}

	for account, g := range groups {
		if err := wb.appendToGLSheet(account, g.entries, g.initial); err != nil {
			return fmt.Errorf("追加科目 %s: %w", account, err)
		}
	}

	return nil
}

// appendToGLSheet 将分录追加到指定科目的总分类账 Sheet。
func (wb *Workbook) appendToGLSheet(account string, entries []voucher.Entry, initial int64) error {
	sheet, err := wb.ensureGLSheet(account)
	if err != nil {
		return err
	}

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	if isNew && initial != 0 {
		if err := wb.insertCarryForward(sheet, initial); err != nil {
			return err
		}
	}

	balance := initial
	var pageDebit, pageCredit int64
	if !isNew {
		balance = wb.lastPageBalance(sheet)
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		// 如果上一行是孤立过次页（无承前页跟随），先补承前页
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		// 如果当前行超出页容量，写过次页 + 承前页
		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			wb.writeCarryForwardRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(e.CreditCents))

		dir, dispBal := directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)

		wb.markRowForPrint(sheet, row)
	}

	return nil
}

// insertCarryForward 在新科目首行插入"上年结转"。
func (wb *Workbook) insertCarryForward(sheet string, amount int64) error {
	row := 3
	dir, dispBal := directionFor(amount, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(4, row), "")
	wb.File.SetCellValue(sheet, cellName(5, row), "")
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 7)

	return nil
}

// lastRowIsOrphanBreak 检查最后一行是否为没有承前页跟随的孤立过次页。
func (wb *Workbook) lastRowIsOrphanBreak(sheet string) bool {
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return false
	}
	last := rows[len(rows)-1]
	return len(last) > 2 && last[2] == pageBreakLabel
}

// lastPageBalance 获取最后一个过次页行的余额。
func (wb *Workbook) lastPageBalance(sheet string) int64 {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			if len(rows[i]) >= 7 {
				if v, err := yuanStrToCents(rows[i][6]); err == nil {
					return v
				}
			}
			return 0
		}
	}
	return 0
}

// lastBreakTotals 获取最后一个过次页行的页借贷合计。
func (wb *Workbook) lastBreakTotals(sheet string) (debit, credit int64) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0, 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			if len(rows[i]) >= 5 {
				if v, err := yuanStrToCents(rows[i][3]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][4]); err == nil {
					credit = v
				}
			}
			return
		}
	}
	return 0, 0
}

// pageStartRow 返回当前页的起始数据行号（跳过标题/过次页/承前页）。
func (wb *Workbook) pageStartRow(sheet string) int {
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return 3
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			return i + 3 // 跳过过次页 + 承前页（i 为 0-index）
		}
	}
	return 3
}

// rowIsPageBreak 检查指定行是否已超出当页容量（pageSize 行数据后需过次页）。
func (wb *Workbook) rowIsPageBreak(sheet string, row int) bool {
	start := wb.pageStartRow(sheet)
	dataRows := row - start
	return dataRows > pageSize
}

// pageHasBreakRow 检查当前页是否已有过次页行。
func (wb *Workbook) pageHasBreakRow(sheet string) bool {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return false
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 2 && rows[i][2] == pageBreakLabel {
			return true
		}
	}
	return false
}

// writePageBreakRow 写"过次页"行（摘要列 + 本月累计发生额 + 余额）。
func (wb *Workbook) writePageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64) {
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), pageBreakLabel)
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
}

// writeCarryForwardRow 写"承前页"行，复制过次页的全部数据。
func (wb *Workbook) writeCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64) {
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), carryForwardLabel)
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
}

// cellName 返回 Excel 单元格名称。
func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
