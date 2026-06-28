package generator

import (
	"fmt"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// 打印版总分类账列布局
const (
	printGLColDate      = 1  // A: 日期
	printGLColVoucher   = 2  // B: 凭证号
	printGLColSummary   = 3  // C: 摘要
	printGLColDebit     = 4  // D: 借方金额起始列（D-O，共12列）
	printGLColCredit    = 16 // P: 贷方金额起始列（P-AA，共12列）
	printGLColDirection = 28 // AB: 方向
	printGLColBalance   = 29 // AC: 余额起始列（AC-AN，共12列）
	printGLTotalCols    = 40 // 总列数
)

// 打印版总分类账表头
var printGLHeaders = []string{
	"日期", "凭证号", "摘要",
	"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分", // 借方
	"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分", // 贷方
	"方向",
	"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分", // 余额
}

// sheetNamePrintGL 返回打印版总分类账 Sheet 名称。
func sheetNamePrintGL(account string) string {
	return fmt.Sprintf("打印-总分类账-%s", account)
}

// ensurePrintGLSheet 确保打印版总分类账 Sheet 存在并已初始化。
func (wb *Workbook) ensurePrintGLSheet(account string) (string, error) {
	name := sheetNamePrintGL(account)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建打印版 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writePrintGLTitle(name, account); err != nil {
		return "", err
	}

	// 设置页面布局
	wb.setPrintPageLayout(name)

	return name, nil
}

// writePrintGLTitle 写入打印版总分类账标题行和列标题。
func (wb *Workbook) writePrintGLTitle(sheet, account string) error {
	// 标题行
	title := "总分类账 — " + account + " （打印版）"
	wb.File.SetCellValue(sheet, "A1", title)
	wb.File.MergeCell(sheet, "A1", cellName(printGLTotalCols, 1))

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A1", cellName(printGLTotalCols, 1), titleStyle)
	wb.File.SetRowHeight(sheet, 1, 20)

	// 第二行：金额栏标题（借方/贷方/余额）
	wb.File.SetCellValue(sheet, cellName(printGLColDebit, 2), "借方金额")
	wb.File.MergeCell(sheet, cellName(printGLColDebit, 2), cellName(printGLColCredit-1, 2))
	wb.File.SetCellValue(sheet, cellName(printGLColCredit, 2), "贷方金额")
	wb.File.MergeCell(sheet, cellName(printGLColCredit, 2), cellName(printGLColDirection-1, 2))
	wb.File.SetCellValue(sheet, cellName(printGLColBalance, 2), "余额")
	wb.File.MergeCell(sheet, cellName(printGLColBalance, 2), cellName(printGLTotalCols, 2))

	// 第三行：详细列标题
	for i, h := range printGLHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		wb.File.SetCellValue(sheet, cell, h)
	}

	// 应用表头样式
	headerStyleID, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 9},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "#808080", Style: 1},
			{Type: "right", Color: "#808080", Style: 1},
			{Type: "top", Color: "#808080", Style: 1},
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// 应用到第二行和第三行
	wb.File.SetCellStyle(sheet, "A2", cellName(printGLTotalCols, 3), headerStyleID)

	// 设置列宽
	wb.File.SetColWidth(sheet, "A", "A", 10)  // 日期
	wb.File.SetColWidth(sheet, "B", "B", 8)   // 凭证号
	wb.File.SetColWidth(sheet, "C", "C", 20)  // 摘要
	for i := 0; i < 12; i++ {
		wb.File.SetColWidth(sheet, cellName(printGLColDebit+i, 1), cellName(printGLColDebit+i, 1), 3) // 借方栏
		wb.File.SetColWidth(sheet, cellName(printGLColCredit+i, 1), cellName(printGLColCredit+i, 1), 3) // 贷方栏
		wb.File.SetColWidth(sheet, cellName(printGLColBalance+i, 1), cellName(printGLColBalance+i, 1), 3) // 余额栏
	}
	wb.File.SetColWidth(sheet, cellName(printGLColDirection, 1), cellName(printGLColDirection, 1), 4) // 方向

	return nil
}

// setPrintPageLayout 设置打印版页面布局。
func (wb *Workbook) setPrintPageLayout(sheet string) {
	// 设置横向打印
	orientation := "landscape"
	fitToWidth := 1
	fitToHeight := 0
	wb.File.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Orientation: &orientation,
		FitToWidth:  &fitToWidth,
		FitToHeight: &fitToHeight,
	})

	// 设置页边距（1cm）
	left := 0.4
	right := 0.4
	top := 0.5
	bottom := 0.5
	wb.File.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
		Left:   &left,
		Right:  &right,
		Top:    &top,
		Bottom: &bottom,
	})

	// 启用 FitToPage
	enable := true
	wb.File.SetSheetProps(sheet, &excelize.SheetPropsOptions{
		FitToPage: &enable,
	})
}

// AppendPrintEntries 追加当月分录到对应的打印版总分类账 Sheet。
func (wb *Workbook) AppendPrintEntries(entries []voucher.Entry, initials map[string]int64) error {
	type entryGroup struct {
		entries []voucher.Entry
		initial int64
	}
	groups := make(map[string]*entryGroup)

	// 构建忽略集合
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
		if err := wb.appendToPrintGLSheet(account, g.entries, g.initial); err != nil {
			return fmt.Errorf("追加打印版科目 %s: %w", account, err)
		}
	}

	return nil
}

// appendToPrintGLSheet 将分录追加到指定科目的打印版总分类账 Sheet。
func (wb *Workbook) appendToPrintGLSheet(account string, entries []voucher.Entry, initial int64) error {
	sheet, err := wb.ensurePrintGLSheet(account)
	if err != nil {
		return err
	}

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 3

	if isNew && initial != 0 {
		if err := wb.insertPrintCarryForward(sheet, initial); err != nil {
			return err
		}
	}

	balance := initial
	if !isNew {
		balance = wb.lastPrintPageBalance(sheet)
	}

	// 创建数据行样式
	dataStyleID, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 8},
		Border: []excelize.Border{
			{Type: "left", Color: "#808080", Style: 1},
			{Type: "right", Color: "#808080", Style: 1},
			{Type: "top", Color: "#808080", Style: 1},
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for _, e := range entries {
		row := len(rows) + 1
		if row <= 4 {
			row = 4
		}

		// 写入基础列
		wb.File.SetCellValue(sheet, cellName(printGLColDate, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(printGLColVoucher, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(printGLColSummary, row), e.Summary)

		// 写入借方金额（分栏）
		writeAmountCells(wb.File, sheet, row, printGLColDebit, e.DebitCents, dataStyleID)

		// 写入贷方金额（分栏）
		writeAmountCells(wb.File, sheet, row, printGLColCredit, e.CreditCents, dataStyleID)

		// 计算余额
		balance = balance + e.DebitCents - e.CreditCents
		dir, dispBal := directionFor(balance, 0)

		// 写入方向
		wb.File.SetCellValue(sheet, cellName(printGLColDirection, row), dir)

		// 写入余额（分栏）
		writeAmountCells(wb.File, sheet, row, printGLColBalance, dispBal, dataStyleID)

		// 应用样式到基础列
		wb.File.SetCellStyle(sheet, cellName(printGLColDate, row), cellName(printGLColSummary, row), dataStyleID)
		wb.File.SetCellStyle(sheet, cellName(printGLColDirection, row), cellName(printGLColDirection, row), dataStyleID)

		rows, _ = wb.File.GetRows(sheet)
	}

	return nil
}

// insertPrintCarryForward 在打印版新科目首行插入"上年结转"。
func (wb *Workbook) insertPrintCarryForward(sheet string, amount int64) error {
	row := 4
	dir, dispBal := directionFor(amount, 0)

	// 创建样式
	dataStyleID, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 8},
		Border: []excelize.Border{
			{Type: "left", Color: "#808080", Style: 1},
			{Type: "right", Color: "#808080", Style: 1},
			{Type: "top", Color: "#808080", Style: 1},
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	wb.File.SetCellValue(sheet, cellName(printGLColDate, row), "")
	wb.File.SetCellValue(sheet, cellName(printGLColVoucher, row), "")
	wb.File.SetCellValue(sheet, cellName(printGLColSummary, row), "上年结转")

	// 借方和贷方留空
	for i := 0; i < 12; i++ {
		wb.File.SetCellValue(sheet, cellName(printGLColDebit+i, row), "")
		wb.File.SetCellValue(sheet, cellName(printGLColCredit+i, row), "")
	}

	wb.File.SetCellValue(sheet, cellName(printGLColDirection, row), dir)

	// 写入余额（分栏）
	writeAmountCells(wb.File, sheet, row, printGLColBalance, dispBal, dataStyleID)

	// 应用样式
	wb.File.SetCellStyle(sheet, cellName(printGLColDate, row), cellName(printGLColSummary, row), dataStyleID)
	wb.File.SetCellStyle(sheet, cellName(printGLColDirection, row), cellName(printGLColDirection, row), dataStyleID)

	return nil
}

// lastPrintPageBalance 获取打印版最后一个过次页行的余额。
func (wb *Workbook) lastPrintPageBalance(sheet string) int64 {
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 4 {
		return 0
	}

	// 从最后一行开始找余额列
	for i := len(rows) - 1; i >= 3; i-- {
		if len(rows[i]) > printGLColBalance {
			// 尝试从余额栏的第一个非空单元格读取
			for j := 0; j < 12; j++ {
				colIdx := printGLColBalance + j - 1
				if colIdx < len(rows[i]) && rows[i][colIdx] != "" {
					// 这里简化处理，实际需要解析分栏数字
					// 暂时返回 0，后续完善
					return 0
				}
			}
		}
	}

	return 0
}
