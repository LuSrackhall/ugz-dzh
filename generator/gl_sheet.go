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

// writeGLTitle 写入总分类账的标题区（2 行）、列标题和列宽。
//
// 行结构：
//   Row 1: 总    分    类    账（居中，绿色+双下划线）| 分第 n 页（"分第"/"页"绿色，数字红色+绿色虚线下划线）
//   Row 2: 会计科目（绿色，右对齐）+ 科目名称（印章红，绿色虚线下划线）
//   Row 3: [空行]
//   Row 4: 年（合并两列）│凭证号│摘要│借方金额│贷方金额│方向│余额
//   Row 5: 月│日

func (wb *Workbook) writeGLTitle(sheet string) error {
	account := sheet[len(sheetPrefixGL):]
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())

	darkGreen := "006100"
	sealRed := "CC0000"

	// ── Row 1: 总    分    类    账（居中）| 分第 n 页（"分第"/"页"绿色，数字红色+绿色虚线下划线） ──
	tl := cellName(lay.TitleColLeft, lay.TitleRow+1)
	tr := cellName(lay.TitleColRight, lay.TitleRow+1)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "   总    分    类    账   ")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	// Right side: "分第 " (cols 6-7, green, right-aligned) + number (col 8, red, green dotted) + " 页" (col 9, green)
	pnLabelLeft := cellName(lay.AccountColLeft, lay.TitleRow+1)
	pnLabelRight := cellName(lay.AccountColLeft+1, lay.TitleRow+1)
	wb.File.MergeCell(sheet, pnLabelLeft, pnLabelRight)
	wb.File.SetCellValue(sheet, pnLabelLeft, "分第 ")
	pnLabelStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, pnLabelLeft, pnLabelRight, pnLabelStyle)

	pnNumLeft := cellName(lay.AccountColLeft+2, lay.TitleRow+1)
	pnNumRight := cellName(lay.AccountColRight-1, lay.TitleRow+1)
	wb.File.MergeCell(sheet, pnNumLeft, pnNumRight)
	wb.File.SetCellValue(sheet, pnNumLeft, "1")
	pnNumStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border: []excelize.Border{
			{Type: "bottom", Color: darkGreen, Style: 4},
		},
	})
	wb.File.SetCellStyle(sheet, pnNumLeft, pnNumRight, pnNumStyle)

	pnSuffix := cellName(lay.AccountColRight, lay.TitleRow+1)
	wb.File.SetCellValue(sheet, pnSuffix, " 页")
	pnSuffixStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, pnSuffix, pnSuffix, pnSuffixStyle)
	wb.File.SetRowHeight(sheet, lay.TitleRow+1, 28)


	// ── Row 2: 会计科目（绿色，右对齐）+ 科目名称（红色，绿色虚线下划线） ──
	// 会计科目 (col 6 only, right-aligned), 科目名称 (cols 7-9, green dotted)
	labelLeft := cellName(lay.AccountColLeft, lay.PageNumRow+1)
	wb.File.SetCellValue(sheet, labelLeft, "会计科目")
	labelStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, labelLeft, labelLeft, labelStyle)

	nameLeft := cellName(lay.AccountColLeft+1, lay.PageNumRow+1)
	nameRight := cellName(lay.AccountColRight, lay.PageNumRow+1)
	wb.File.MergeCell(sheet, nameLeft, nameRight)
	wb.File.SetCellValue(sheet, nameLeft, account)
	nameStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border: []excelize.Border{
			{Type: "bottom", Color: darkGreen, Style: 4},
		},
	})
	wb.File.SetCellStyle(sheet, nameLeft, nameRight, nameStyle)
	wb.File.SetRowHeight(sheet, lay.PageNumRow+1, 18)
	// ── Row 4: 顶层列标题 — N 年（合并月/日两列）+ 凭证号/摘要/金额 ──
	year := wb.Month[:4]
	yearLeft := cellName(lay.FrontStartCol, lay.HeaderRow+1)
	yearRight := cellName(lay.FrontStartCol+1, lay.HeaderRow+1)
	wb.File.MergeCell(sheet, yearLeft, yearRight)
	wb.File.SetCellValue(sheet, yearLeft, year+"年")
	// "凭证" 合并字+号两列
	vouchLeft := cellName(lay.FrontStartCol+2, lay.HeaderRow+1)
	vouchRight := cellName(lay.FrontStartCol+3, lay.HeaderRow+1)
	wb.File.MergeCell(sheet, vouchLeft, vouchRight)
	wb.File.SetCellValue(sheet, vouchLeft, "凭证")
	otherCols := []string{"摘要", "借方金额", "贷方金额", "方向", "余额"}
	for i, h := range otherCols {
		cell := cellName(lay.FrontStartCol+4+i, lay.HeaderRow+1)
		wb.File.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: "006100"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := cellName(lay.FrontStartCol, lay.HeaderRow+1)
	he := cellName(lay.FrontStartCol+len(otherCols)+3, lay.HeaderRow+1)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

	// ── Row 5: 子表头 — 月 | 日 | 字 | 号 ──
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol, lay.SubHeaderRow+1), "月")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+1, lay.SubHeaderRow+1), "日")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+2, lay.SubHeaderRow+1), "字")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+3, lay.SubHeaderRow+1), "号")
	subHStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: "006100"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol, lay.SubHeaderRow+1), cellName(lay.FrontStartCol+3, lay.SubHeaderRow+1), subHStyle)

	// ── 列宽（按比例分配） ──
	for i, c := range lay.Columns {
		if i >= len(lay.ExcelColumns) {
			break
		}
		cl, _ := excelize.ColumnNumberToName(lay.ExcelColumns[i].Col)
		w := layout.GLMMToExcelColWidth(c.WidthMM)
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
	// 反面区列宽（与正面按比例一致）
	for i, c := range lay.Columns {
		if i >= len(lay.ExcelColumns) {
			break
		}
		backCol := lay.ExcelColumns[i].Col + (lay.BackStartCol - lay.FrontStartCol)
		cl, _ := excelize.ColumnNumberToName(backCol)
		w := layout.GLMMToExcelColWidth(c.WidthMM)
		if w < 3 {
			w = 3
		}
		wb.File.SetColWidth(sheet, cl, cl, w)
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


// hasPageBreakAt 检查 row 中是否有完整过次页标记（标签 + 余额数据）。
// 仅标签无金额（模板站位）不算真断页。
func hasPageBreakAt(row []string, lay layout.GLLayout) bool {
	return (len(row) > lay.BindingLeftCols+8 && row[lay.BindingLeftCols+4] == pageBreakLabel && row[lay.BindingLeftCols+8] != "") ||
		(len(row) > lay.BackStartCol+7 && row[lay.BackStartCol+3] == pageBreakLabel && row[lay.BackStartCol+7] != "")
}

// nextDataRow 返回 Sheet 中下一个可用数据行号（Excel 行号，1-indexed）。
// 从 GetRows 末尾反向扫描，跳过空行和模板标签，找到最后一个实际数据行。
func (wb *Workbook) nextDataRow(sheet string) (int, error) {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return lay.DataStartRow + 1, nil
	}

	// 从后向前扫描 GetRows
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) == 0 {
			continue // 空行跳过
		}

		// 真断页（完整过次页：标签 + 余额）
		if hasPageBreakAt(rows[i], lay) {
			// 过次页在 GetRows[i]，Excel 行号 = i+1
			// 返回过次页后一行供承前页/新页处理
			return i + 2, nil
		}

		// 模板标签（仅红色"过次页"文字，无余额）→ 跳过
		isTemplate := false
		if len(rows[i]) > lay.BindingLeftCols+4 && rows[i][lay.BindingLeftCols+4] == pageBreakLabel {
			if len(rows[i]) <= lay.BindingLeftCols+8 || rows[i][lay.BindingLeftCols+8] == "" {
				isTemplate = true
			}
		}
		if isTemplate {
			continue
		}

		// 真实数据行 → 返回下一行
		// GetRows[i] = Excel 行号 i+1，下一行 = i+2
		return i + 2, nil
	}

	// 完全无数据 → 从表头后开始
	return lay.DataStartRow + 1, nil
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
// getPageNum 从 Sheet 中过次页标签总数计算当前页码（含模板和真断页）。
func (wb *Workbook) getPageNum(sheet string) int {
	lay := glLayout()
	rows, _ := wb.File.GetRows(sheet)
	pn := 1
	for _, r := range rows {
		if len(r) > lay.BindingLeftCols+4 && r[lay.BindingLeftCols+4] == pageBreakLabel {
			pn++
		}
		if len(r) > lay.BackStartCol+3 && r[lay.BackStartCol+3] == pageBreakLabel {
			pn++
		}
	}
	return pn
}

func (wb *Workbook) appendToGLSheet(account string, entries []voucher.Entry, initial int64) error {
	sheet, err := wb.ensureGLSheet(account)
	if err != nil {
		return err
	}

	lay := glLayout()
	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	// 计算页码：从文件"过次页"标签数（含模板和真断页）
	pageNum := wb.getPageNum(sheet)

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
		}
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			pageNum = wb.getPageNum(sheet)
			wb.writePageHeader(sheet, row, pageNum, account)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pageNum)
			row++
		}

		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageNum)
			row++
			pageNum = wb.getPageNum(sheet)
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

		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), e.Date[5:7])
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), e.Date[8:10])
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "")
			wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), fmt.Sprintf("%d", e.VoucherNum))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(e.CreditCents))

		dir, dispBal := directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), dir)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))

	}

	return nil
}

// insertCarryForward 在新科目首行插入"上年结转"。
func (wb *Workbook) insertCarryForward(sheet string, amount int64, pageNum int) error {
	lay := glLayout()
	row := lay.DataStartRow + 1
	dir, dispBal := directionFor(amount, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
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
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "上年结转")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
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
		if len(rows[i]) > lay.BindingLeftCols+4 && rows[i][lay.BindingLeftCols+4] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+9 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+8]); err == nil {
					return v
				}
			}
		}
		// 过次页在 Back 区
		if len(rows[i]) > lay.BackStartCol+3 && rows[i][lay.BackStartCol+3] == pageBreakLabel {
			balIdx := lay.BackStartCol + 7 // GetRows index = col - 1
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
		if len(rows[i]) > lay.BindingLeftCols+4 && rows[i][lay.BindingLeftCols+4] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+5 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+5]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+6]); err == nil {
					credit = v
				}
			}
		}
		// 过次页在 Back 区
		if len(rows[i]) > lay.BackStartCol+3 && rows[i][lay.BackStartCol+3] == pageBreakLabel {
			debIdx := lay.BackStartCol + 4 // GetRows index for debit
			crdIdx := lay.BackStartCol + 5 // GetRows index for credit
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
	if err != nil || len(rows) < lay.DataStartRow {
		return lay.DataStartRow + 1
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if hasPageBreakAt(rows[i], lay) {
			// i = GetRows index ≈ Excel row - 1（GetRows 包含空行）
			// 过次页后一行为空，再后 DataStartRow 行为新页页头
			return i + 3 + lay.DataStartRow
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
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), pageBreakLabel)
	pbStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "CC0000", Size: 10},
	})
	wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 4), row), cellName(dataCol(lay, pageNum, 4), row), pbStyle)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
}

// writeCarryForwardRow 写"承前页"行。
func (wb *Workbook) writeCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageNum int) {
	lay := glLayout()
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), carryForwardLabel)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 5), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 6), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 7), row), dir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 8), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 5))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 6))
	wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, 8))
}

// writePageHeader 写入后续页标题行（过次页之后、承前页之前调用），包含页码、总分类账、科目名称、列标题。
// 行结构（4 行，与 writeGLTitle 相同）：
//   Row N+0: 总    分    类    账（居中，绿色+双下划线）| 分第 n 页（"分第"/"页"绿色，数字红色+绿色虚线下划线）
//   Row N+1: 会计科目（绿色，右对齐）+ 科目名称（印章红，绿色虚线下划线）
//   Row N+2: [空行]
//   Row N+3: 年（合并两列）│凭证号│摘要│借方金额│贷方金额│方向│余额
//   Row N+4: 月│日
func (wb *Workbook) writePageHeader(sheet string, row int, pageNum int, account string) error {
	lay := glLayout()

	colOffset := 0
	if pageNum%2 == 0 {
		colOffset = lay.BackStartCol - lay.FrontStartCol
	}

	darkGreen := "006100"
	sealRed := "CC0000"

		// Row N+0: 总    分    类    账（居中）| 分第 n 页（"分第"/"页"绿色，数字红色+绿色虚线下划线）
	tl := cellName(lay.TitleColLeft+colOffset, row)
	tr := cellName(lay.TitleColRight+colOffset, row)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "   总    分    类    账   ")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	// "分第 " (cols 6-7, green, right-aligned) + number (col 8, red, green dotted) + " 页" (col 9, green)
	pnLabelLeft := cellName(lay.AccountColLeft+colOffset, row)
	pnLabelRight := cellName(lay.AccountColLeft+1+colOffset, row)
	wb.File.MergeCell(sheet, pnLabelLeft, pnLabelRight)
	wb.File.SetCellValue(sheet, pnLabelLeft, "分第 ")
	pnLabelStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, pnLabelLeft, pnLabelRight, pnLabelStyle)

	pnNumLeft := cellName(lay.AccountColLeft+2+colOffset, row)
	pnNumRight := cellName(lay.AccountColRight-1+colOffset, row)
	wb.File.MergeCell(sheet, pnNumLeft, pnNumRight)
	wb.File.SetCellValue(sheet, pnNumLeft, fmt.Sprintf("%d", pageNum))
	pnNumStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border: []excelize.Border{
			{Type: "bottom", Color: darkGreen, Style: 4},
		},
	})
	wb.File.SetCellStyle(sheet, pnNumLeft, pnNumRight, pnNumStyle)

	pnSuffix := cellName(lay.AccountColRight+colOffset, row)
	wb.File.SetCellValue(sheet, pnSuffix, " 页")
	pnSuffixStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, pnSuffix, pnSuffix, pnSuffixStyle)
	wb.File.SetRowHeight(sheet, row, 28)
	row++
	// Row N+1: 会计科目（绿色，右对齐）+ 科目名称（红色，绿色虚线下划线）
	// 会计科目 (col 6 only, right-aligned), 科目名称 (cols 7-9, green dotted)
	labelLeft := cellName(lay.AccountColLeft+colOffset, row)
	wb.File.SetCellValue(sheet, labelLeft, "会计科目")
	labelStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, labelLeft, labelLeft, labelStyle)

	nameLeft := cellName(lay.AccountColLeft+1+colOffset, row)
	nameRight := cellName(lay.AccountColRight+colOffset, row)
	wb.File.MergeCell(sheet, nameLeft, nameRight)
	wb.File.SetCellValue(sheet, nameLeft, account)
	nameStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border: []excelize.Border{
			{Type: "bottom", Color: darkGreen, Style: 4},
		},
	})
	wb.File.SetCellStyle(sheet, nameLeft, nameRight, nameStyle)
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+2: [空行]
	row++

	// Row N+3: 顶层列标题 — N 年（合并月/日两列）+ 凭证号/摘要/金额
	year := wb.Month[:4]
	yearLeft := cellName(lay.FrontStartCol+colOffset, row)
	yearRight := cellName(lay.FrontStartCol+1+colOffset, row)
	wb.File.MergeCell(sheet, yearLeft, yearRight)
	wb.File.SetCellValue(sheet, yearLeft, year+"年")
	// "凭证" 合并字+号两列
	vouchLeft := cellName(lay.FrontStartCol+2+colOffset, row)
	vouchRight := cellName(lay.FrontStartCol+3+colOffset, row)
	wb.File.MergeCell(sheet, vouchLeft, vouchRight)
	wb.File.SetCellValue(sheet, vouchLeft, "凭证")
	otherCols := []string{"摘要", "借方金额", "贷方金额", "方向", "余额"}
	for i, h := range otherCols {
		cell := cellName(lay.FrontStartCol+4+i+colOffset, row)
		wb.File.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: "006100"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := cellName(lay.FrontStartCol+colOffset, row)
	he := cellName(lay.FrontStartCol+len(otherCols)+3+colOffset, row)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)
	row++

	// Row N+4: 子表头 — 月 | 日 | 字 | 号
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+colOffset, row), "月")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+1+colOffset, row), "日")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+2+colOffset, row), "字")
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+3+colOffset, row), "号")
	subHStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: "006100"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol+colOffset, row), cellName(lay.FrontStartCol+3+colOffset, row), subHStyle)
	row++

	return nil
}





// finalizeGLSheet 在每页第 21 行写入红色过次页标签（不写金额，满行时由 writePageBreakRow 覆盖）。
func (wb *Workbook) finalizeGLSheet(sheet string) error {
	lay := glLayout()
	sealRed := "CC0000"

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return err
	}
	breakCount := 0
	for _, r := range rows {
		if hasPageBreakAt(r, lay) {
			breakCount++
		}
	}
	pageNum := breakCount + 1

	pageStart := wb.pageStartRow(sheet)
	breakRow := pageStart + pageSize

	colOff := 0
	if pageNum%2 == 0 {
		colOff = lay.BackStartCol - lay.FrontStartCol
	}

	// 检查是否已有完整过次页（有金额的）
	existingVal, _ := wb.File.GetCellValue(sheet, cellName(lay.FrontStartCol+5+colOff, breakRow))
	if existingVal != "" && existingVal != "0.00" {
		return nil // 已有带数据的过次页
	}
	// 检查是否已写过红色标签
	existingLabel, _ := wb.File.GetCellValue(sheet, cellName(lay.FrontStartCol+4+colOff, breakRow))
	if existingLabel == pageBreakLabel {
		return nil // 标签已存在
	}

	// 写红色过次页标签（仅文字，无金额）
	wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+4+colOff, breakRow), pageBreakLabel)
	pbStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: sealRed, Size: 10},
	})
	wb.File.SetCellStyle(sheet, cellName(lay.FrontStartCol+4+colOff, breakRow), cellName(lay.FrontStartCol+4+colOff, breakRow), pbStyle)
	return nil
}

func (wb *Workbook) finalizeAllGLSheets() error {
	for _, sheet := range wb.File.GetSheetList() {
		if strings.HasPrefix(sheet, sheetPrefixGL) {
			if err := wb.finalizeGLSheet(sheet); err != nil {
				return err
			}
		}
	}
	return nil
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
