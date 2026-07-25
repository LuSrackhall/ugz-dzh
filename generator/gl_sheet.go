package generator

import (
	"fmt"

	"ledger/generator/layout"
	"strings"
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

// writeGLTitle 写入总分类账的标题区（3 行）、列标题和列宽。
//
// 行结构：
//   Row 1: 分第 n 页（右侧，绿色+数字红色）
//   Row 2: 总    分    类    账（居中，绿色+双下划线）| 科目名称（右侧）
//   Row 3: 科目名称（右侧，印章红）
//   Row 4: [空行]
//   Row 5: 日期│凭证号│摘要│借方金额│贷方金额│方向│余额│金额分栏
func (wb *Workbook) writeGLTitle(sheet string) error {
	account := sheet[len(sheetPrefixGL):]
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())

	darkGreen := "006100"
	sealRed := "CC0000"

	// ── Row 1: 分第 1 页（右侧，绿色，数字印章红） ──
	pnLeft := cellName(lay.AccountColLeft, lay.PageNumRow+1)
	pnRight := cellName(lay.AccountColRight, lay.PageNumRow+1)
	wb.File.MergeCell(sheet, pnLeft, pnRight)
	wb.File.SetCellRichText(sheet, pnLeft, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: "1", Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	wb.File.SetRowHeight(sheet, lay.PageNumRow+1, 18)

	// ── Row 2: 总    分    类    账（居中）+ 科目名称（右侧） ──
	tl := cellName(lay.TitleColLeft, lay.TitleRow+1)
	tr := cellName(lay.TitleColRight, lay.TitleRow+1)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "   总    分    类    账   ")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	al := cellName(lay.AccountColLeft, lay.TitleRow+1)
	ar := cellName(lay.AccountColRight, lay.TitleRow+1)
	wb.File.MergeCell(sheet, al, ar)
	wb.File.SetCellValue(sheet, al, account)
	accStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, al, ar, accStyle)
	wb.File.SetRowHeight(sheet, lay.TitleRow+1, 28)

	// ── Row 3: 科目名称（右侧，印章红） — 独立行 ──
	acLeft := cellName(lay.AccountColLeft, lay.AccountRow+1)
	acRight := cellName(lay.AccountColRight, lay.AccountRow+1)
	wb.File.MergeCell(sheet, acLeft, acRight)
	wb.File.SetCellValue(sheet, acLeft, account)
	acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, acLeft, acRight, acRowStyle)
	wb.File.SetRowHeight(sheet, lay.AccountRow+1, 18)

	// ── Row 5: 列标题 ──
	colNames := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额", "金额分栏"}
	for i, h := range colNames {
		cell := cellName(lay.FrontStartCol+i, lay.HeaderRow+1)
		if i < len(colNames) {
			wb.File.SetCellValue(sheet, cell, h)
		}
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := cellName(lay.FrontStartCol, lay.HeaderRow+1)
	he := cellName(lay.FrontStartCol+len(colNames)-1, lay.HeaderRow+1)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

	// ── 列宽 ──
	avgWidth := layout.GLMMToExcelColWidth(lay.FrontWidthMM / float64(len(lay.ExcelColumns)))
	for _, ec := range lay.ExcelColumns {
		cl, _ := excelize.ColumnNumberToName(ec.Col)
		w := avgWidth
		if w < 3 {
			w = 3
		}
		wb.File.SetColWidth(sheet, cl, cl, w)
	}
	// 装订列
	for _, offset := range []int{1, 2} {
		if offset <= lay.TotalCols {
			cl, _ := excelize.ColumnNumberToName(offset)
			wb.File.SetColWidth(sheet, cl, cl, 2)
		}
	}
	// 页间隙列
	if lay.PageGapStartCol <= lay.TotalCols {
		cl, _ := excelize.ColumnNumberToName(lay.PageGapStartCol)
		wb.File.SetColWidth(sheet, cl, cl, 2)
	}
	// 反面区列宽（与正面一致）
	for _, ec := range lay.ExcelColumns {
		backCol := ec.Col + (lay.BackStartCol - lay.FrontStartCol)
		cl, _ := excelize.ColumnNumberToName(backCol)
		wb.File.SetColWidth(sheet, cl, cl, avgWidth)
	}
	// 右侧装订列
	for _, offset := range []int{lay.TotalCols, lay.TotalCols - 1} {
		if offset > 0 && offset > lay.BackStartCol && offset <= lay.TotalCols {
			cl, _ := excelize.ColumnNumberToName(offset)
			wb.File.SetColWidth(sheet, cl, cl, 2)
		}
	}

	return nil
}

// glLayout 返回当前 GL 布局，供写入和读取端统一使用。
func glLayout() layout.GLLayout {
	return layout.GLComputeLayout(layout.DefaultGLSpec())
}

// dataCol 根据 pageNum 奇偶决定写入列，奇数→FrontStartCol，偶数→BackStartCol。
func dataCol(lay layout.GLLayout, pageNum, offset int) int {
	if pageNum%2 == 1 {
		return lay.FrontStartCol + offset
	}
	return lay.BackStartCol + offset
}


// hasPageBreakAt 检查 row 中是否有"过次页"标记（在 Front 或 Back 区域）。
func hasPageBreakAt(row []string, lay layout.GLLayout) bool {
	return (len(row) > lay.BindingLeftCols+2 && row[lay.BindingLeftCols+2] == pageBreakLabel) ||
		(len(row) > lay.BackStartCol+1 && row[lay.BackStartCol+1] == pageBreakLabel)
}

// nextDataRow 返回 Sheet 中下一个可用数据行号。
// 若最后一行为孤立过次页（无承前页跟随），返回过次页+1 供承前页写入。
func (wb *Workbook) nextDataRow(sheet string) (int, error) {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}

	if len(rows) < 3 {
		return 3, nil
	}

	lastBreak := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if hasPageBreakAt(rows[i], lay) {
			lastBreak = i + 1
			break
		}
	}

	if lastBreak > 0 && lastBreak == len(rows) {
		return lastBreak + 1, nil
	}

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

	glSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.GLSuppressAccounts {
		glSuppress[a] = true
	}

	for _, e := range entries {
		if glSuppress[e.GeneralAccount] {
			continue
		}
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

	lay := glLayout()
	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	// 计算页码：已有过次页数 + 1
	pageNum := 1
	for _, r := range rows {
		if hasPageBreakAt(r, lay) {
			pageNum++
		}
	}

	if isNew && initial != 0 {
		if err := wb.insertCarryForward(sheet, initial, pageNum); err != nil {
			return err
		}
	} else if !isNew && initial != 0 && strings.HasSuffix(wb.Month, "-01") {
		// 仅 1 月的 Sheet 有跨年期初余额时，在数据区首行插入上年结转
		if err := wb.insertCarryForwardAtRow(sheet, initial, pageNum); err != nil {
			return err
		}
	}

	balance := initial
	var pageDebit, pageCredit int64
	if !isNew && initial == 0 {
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

		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			pageNum++
			wb.writePageHeader(sheet, row, pageNum, account)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pageNum)
			row++
		}

		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageNum)
			row++
			pageNum++
			wb.writePageHeader(sheet, row, pageNum, account)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, pageDebit, pageCredit, pageNum)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), e.Date)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), centsToYuan(e.CreditCents))

		dir, dispBal := directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), dir)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 3))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 4))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))

		wb.markRowForPrint(sheet, row)
	}

	return nil
}

// insertCarryForward 在新科目首行插入"上年结转"。
func (wb *Workbook) insertCarryForward(sheet string, amount int64, pageNum int) error {
	lay := glLayout()
	row := 3
	dir, dispBal := directionFor(amount, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
	return nil
}

// insertCarryForwardAtRow 在已有数据的 Sheet 中，于数据区首行追加"上年结转"行。
func (wb *Workbook) insertCarryForwardAtRow(sheet string, amount int64, pageNum int) error {
	lay := glLayout()
	row, err := wb.nextDataRow(sheet)
	if err != nil {
		return err
	}
	dir, dispBal := directionFor(amount, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
	return nil
}

// lastRowIsOrphanBreak 检查最后一行是否为没有承前页跟随的孤立过次页。
func (wb *Workbook) lastRowIsOrphanBreak(sheet string) bool {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return false
	}
	last := rows[len(rows)-1]
	return hasPageBreakAt(last, lay)
}

// lastPageBalance 获取最后一个过次页行的余额。
func (wb *Workbook) lastPageBalance(sheet string) int64 {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if !hasPageBreakAt(rows[i], lay) {
			continue
		}
		// 过次页在 Front 区
		if len(rows[i]) > lay.BindingLeftCols+2 && rows[i][lay.BindingLeftCols+2] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+7 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+6]); err == nil {
					return v
				}
			}
		}
		// 过次页在 Back 区
		if len(rows[i]) > lay.BackStartCol+1 && rows[i][lay.BackStartCol+1] == pageBreakLabel {
			balIdx := lay.BackStartCol + 5 // GetRows index = col - 1
			if len(rows[i]) > balIdx {
				if v, err := yuanStrToCents(rows[i][balIdx]); err == nil {
					return v
				}
			}
		}
		return 0
	}
	return 0
}

// lastBreakTotals 获取最后一个过次页行的页借贷合计。
func (wb *Workbook) lastBreakTotals(sheet string) (debit, credit int64) {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0, 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if !hasPageBreakAt(rows[i], lay) {
			continue
		}
		// 过次页在 Front 区
		if len(rows[i]) > lay.BindingLeftCols+2 && rows[i][lay.BindingLeftCols+2] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+5 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+3]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+4]); err == nil {
					credit = v
				}
			}
		}
		// 过次页在 Back 区
		if len(rows[i]) > lay.BackStartCol+1 && rows[i][lay.BackStartCol+1] == pageBreakLabel {
			debIdx := lay.BackStartCol + 2 // GetRows index for debit
			crdIdx := lay.BackStartCol + 3 // GetRows index for credit
			if len(rows[i]) > crdIdx {
				if v, err := yuanStrToCents(rows[i][debIdx]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][crdIdx]); err == nil {
					credit = v
				}
			}
		}
		return
	}
	return 0, 0
}

// pageStartRow 返回当前页第一个有效数据行的行号。
// 过次页后新页：承前页为第一行。首页：DataStartRow+1（列标题之后的首行）。
func (wb *Workbook) pageStartRow(sheet string) int {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return 3
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if hasPageBreakAt(rows[i], lay) {
			// 过次页在 i→承前页在 i+1+DataStartRow
			return i + 2 + lay.DataStartRow
		}
	}
	return lay.DataStartRow + 1
}

// rowIsPageBreak 检查指定行是否已超出当页容量（pageSize 行数据后需过次页）。
func (wb *Workbook) rowIsPageBreak(sheet string, row int) bool {
	start := wb.pageStartRow(sheet)
	dataRows := row - start
	return dataRows >= pageSize
}

// pageHasBreakRow 检查当前页是否已有过次页行。
func (wb *Workbook) pageHasBreakRow(sheet string) bool {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return false
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if hasPageBreakAt(rows[i], lay) {
			return true
		}
	}
	return false
}

// writePageBreakRow 写"过次页"行。
func (wb *Workbook) writePageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageNum int) {
	lay := glLayout()
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), pageBreakLabel)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 3))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 4))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
}

// writeCarryForwardRow 写"承前页"行。
func (wb *Workbook) writeCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageNum int) {
	lay := glLayout()
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), carryForwardLabel)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 3))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 4))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
}

// writePageHeader 写入后续页标题行（过次页之后、承前页之前调用），包含页码、总分类账、科目名称、列标题。
// 行结构（5 行，与 writeGLTitle 相同）：
//   Row N+0: 分第 n 页（右侧，绿色+数字红色）
//   Row N+1: 总    分    类    账（居中，绿色+双下划线）| 科目名称（右侧）
//   Row N+2: 科目名称（右侧，印章红）
//   Row N+3: [空行]
//   Row N+4: 日期│凭证号│摘要│借方金额│贷方金额│方向│余额│金额分栏
func (wb *Workbook) writePageHeader(sheet string, row int, pageNum int, account string) error {
	lay := glLayout()

	colOffset := 0
	if pageNum%2 == 0 {
		colOffset = lay.BackStartCol - lay.FrontStartCol
	}

	darkGreen := "006100"
	sealRed := "CC0000"

	// Row N+0: 分第 n 页（右侧，绿色，数字印章红）
	pnLeft := cellName(lay.AccountColLeft+colOffset, row)
	pnRight := cellName(lay.AccountColRight+colOffset, row)
	wb.File.MergeCell(sheet, pnLeft, pnRight)
	wb.File.SetCellRichText(sheet, pnLeft, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+1: 总    分    类    账（居中）+ 科目名称（右侧）
	tl := cellName(lay.TitleColLeft+colOffset, row)
	tr := cellName(lay.TitleColRight+colOffset, row)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "   总    分    类    账   ")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	al := cellName(lay.AccountColLeft+colOffset, row)
	ar := cellName(lay.AccountColRight+colOffset, row)
	wb.File.MergeCell(sheet, al, ar)
	wb.File.SetCellValue(sheet, al, account)
	accStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, al, ar, accStyle)
	wb.File.SetRowHeight(sheet, row, 28)
	row++

	// Row N+2: 科目名称（右侧，印章红）
	acLeft := cellName(lay.AccountColLeft+colOffset, row)
	acRight := cellName(lay.AccountColRight+colOffset, row)
	wb.File.MergeCell(sheet, acLeft, acRight)
	wb.File.SetCellValue(sheet, acLeft, account)
	acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, acLeft, acRight, acRowStyle)
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+3: [空行]
	row++

	// Row N+4: 列标题
	colNames := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额", "金额分栏"}
	for i, h := range colNames {
		cell := cellName(lay.FrontStartCol+colOffset+i, row)
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
	hs := cellName(lay.FrontStartCol+colOffset, row)
	he := cellName(lay.FrontStartCol+colOffset+len(colNames)-1, row)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

	return nil
}
func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
