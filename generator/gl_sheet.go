package generator

import (
	"fmt"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// GL headers
var glHeaders = []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}

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
	// 标题行 (row 1): 总分类账 — {科目全路径}
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

	// 列标题行 (row 2)
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

	// 列宽
	wb.File.SetColWidth(sheet, "A", "A", 12)
	wb.File.SetColWidth(sheet, "B", "B", 8)
	wb.File.SetColWidth(sheet, "C", "C", 35)
	wb.File.SetColWidth(sheet, "D", "D", 14)
	wb.File.SetColWidth(sheet, "E", "E", 14)
	wb.File.SetColWidth(sheet, "F", "F", 6)
	wb.File.SetColWidth(sheet, "G", "G", 16)

	return nil
}

// nextDataRow 返回 Sheet 中下一个可用数据行号（标题行 1-2 之后）。
// 数据行从第 3 行开始，不超过 pageSize 行数据 + 1 过次页行。
func (wb *Workbook) nextDataRow(sheet string) (int, error) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil // 空 Sheet, 从第 3 行开始
	}

	totalRows := len(rows)
	if totalRows < 3 {
		return 3, nil
	}

	// 找到当前页的最后一个数据行
	// 从后往前找最近的 "过次页" 行
	lastPageBreak := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 0 && rows[i][0] == pageBreakLabel {
			lastPageBreak = i + 1 // 1-based
			break
		}
	}

	// 如果最后一行是过次页，需要开新页
	if lastPageBreak > 0 && lastPageBreak == len(rows) {
		return lastPageBreak + 1, nil
	}

	// 计算当前页已用数据行数
	dataStart := lastPageBreak + 1
	if dataStart == 1 {
		dataStart = 3
	}
	usedDataRows := len(rows) - dataStart + 1

	if usedDataRows >= pageSize {
		// 当前页满，插入过次页后开新页
		return len(rows) + 1, nil
	}

	return len(rows) + 1, nil
}

// AppendEntries 追加当月分录到对应的总分类账 Sheet。
func (wb *Workbook) AppendEntries(entries []voucher.Entry, initials map[string]int64) error {
	// 按叶子科目分组
	type entryGroup struct {
		entries  []voucher.Entry
		initial  int64
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

	// 检查是否是全新科目（首次出现），如果是且期初不为 0，需插入"上年结转"
	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2 // 只有标题行和列标题行

	if isNew && initial != 0 {
		if err := wb.insertCarryForward(sheet, initial); err != nil {
			return err
		}
	}

	// 获取当前余额（上页过次页余额或期初）
	balance := initial
	if !isNew {
		balance = wb.lastPageBreakBalance(sheet)
		// 跨月未满页：当前页没有过次页但有旧数据行
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		// 如果当前行是过次页行的位置（页已满），先写过次页
		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance)
			row++
			// 新页继续：余额从上页过次页结转
		}

		balance = balance + e.DebitCents - e.CreditCents
		dir, dispBal := directionFor(balance, 0)
		_ = dir

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(e.CreditCents))

		dir, dispBal = directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

		// 标记需打印
		wb.markRowForPrint(sheet, row)

		// 检查写入后是否满页
		writtenRows := row - wb.pageStartRow(sheet) + 1
		if writtenRows >= pageSize {
			wb.writePageBreakRow(sheet, row+1, balance)
		}
	}

	return nil
}

// insertCarryForward 在新科目首行插入"上年结转"。
func (wb *Workbook) insertCarryForward(sheet string, amount int64) error {
	row := 3
	dir, dispBal := directionFor(amount, 0)
	_ = dir
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(4, row), "")
	wb.File.SetCellValue(sheet, cellName(5, row), "")

	dir, dispBal = directionFor(amount, 0)
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))

	return nil
}

// lastPageBreakBalance 获取最后一个过次页行的余额。
func (wb *Workbook) lastPageBreakBalance(sheet string) int64 {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 0 && rows[i][0] == pageBreakLabel {
			if len(rows[i]) >= 7 {
				if v, err := yuanStrToCents(rows[i][6]); err == nil {
					return v
				}
			}
		}
	}
	return 0
}

// pageStartRow 返回当前页的起始数据行号。
func (wb *Workbook) pageStartRow(sheet string) int {
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return 3
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 0 && rows[i][0] == pageBreakLabel {
			return i + 2 // 1-based + 下一行
		}
	}
	return 3
}

// rowIsPageBreak 检查指定行是否应该是过次页位置（满 20 行数据后）。
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
		if len(rows[i]) > 0 && rows[i][0] == pageBreakLabel {
			return true
		}
	}
	return false
}

// writePageBreakRow 写"过次页"行。
func (wb *Workbook) writePageBreakRow(sheet string, row int, balance int64) {
	dir, dispBal := directionFor(balance, 0)
	_ = dir
	wb.File.SetCellValue(sheet, cellName(1, row), pageBreakLabel)
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "")
	wb.File.SetCellValue(sheet, cellName(4, row), "")
	wb.File.SetCellValue(sheet, cellName(5, row), "")
	dir, dispBal = directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(dispBal))
}

// cellName 返回 Excel 单元格名称。
func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
