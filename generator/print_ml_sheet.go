package generator

import (
	"fmt"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// 打印版多科目明细账列布局
const (
	printMLColDate      = 1  // A: 日期
	printMLColVoucher   = 2  // B: 凭证号
	printMLColSummary   = 3  // C: 摘要
	printMLColDebit     = 4  // D: 借方金额（普通格式）
	printMLColCredit    = 5  // E: 贷方金额（普通格式）
	printMLColDirection = 6  // F: 方向
	printMLColBalance   = 7  // G: 余额（普通格式）
	printMLColDetail    = 8  // H: 明细科目起始列（分栏）
	printMLTotalCols    = 40 // 总列数（预留）
)

// sheetNamePrintML 返回打印版多科目明细账 Sheet 名称。
func sheetNamePrintML(general string) string {
	// 与查看版保持一致，通过文件名区分
	return sheetPrefixML + general
}

// ensurePrintMLSheet 确保打印版多科目明细账 Sheet 存在并已初始化。
func (wb *Workbook) ensurePrintMLSheet(general string, details []string) (string, error) {
	name := sheetNamePrintML(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建打印版明细账 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writePrintMLTitle(name, general, details); err != nil {
		return "", err
	}

	// 设置页面布局
	wb.setPrintPageLayout(name)

	return name, nil
}

// writePrintMLTitle 写入打印版多科目明细账标题行和列标题。
func (wb *Workbook) writePrintMLTitle(sheet, general string, details []string) error {
	// 标题行
	title := "多科目明细账 — " + general + " （打印版）"
	wb.File.SetCellValue(sheet, "A1", title)
	wb.File.MergeCell(sheet, "A1", cellName(printMLTotalCols, 1))

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A1", cellName(printMLTotalCols, 1), titleStyle)
	wb.File.SetRowHeight(sheet, 1, 20)

	// 第二行：左页标题（A-G）
	leftHeaders := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}
	for i, h := range leftHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(sheet, cell, h)
	}

	// 第二行：右页标题（明细科目）
	for i, detail := range details {
		if detail == "" {
			continue
		}
		col := printMLColDetail + i*12
		if col+11 > printMLTotalCols {
			break
		}
		// 合并单元格显示科目名
		wb.File.SetCellValue(sheet, cellName(col, 2), detail)
		wb.File.MergeCell(sheet, cellName(col, 2), cellName(col+11, 2))
	}

	// 第三行：金额分栏标题
	for i := 0; i < len(details); i++ {
		col := printMLColDetail + i*12
		if col+11 > printMLTotalCols {
			break
		}
		amountHeaders := []string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}
		for j, h := range amountHeaders {
			cell, _ := excelize.CoordinatesToCellName(col+j, 3)
			wb.File.SetCellValue(sheet, cell, h)
		}
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

	wb.File.SetCellStyle(sheet, "A2", cellName(printMLTotalCols, 3), headerStyleID)

	// 设置列宽
	wb.File.SetColWidth(sheet, "A", "A", 10)  // 日期
	wb.File.SetColWidth(sheet, "B", "B", 8)   // 凭证号
	wb.File.SetColWidth(sheet, "C", "C", 20)  // 摘要
	wb.File.SetColWidth(sheet, "D", "D", 12)  // 借方
	wb.File.SetColWidth(sheet, "E", "E", 12)  // 贷方
	wb.File.SetColWidth(sheet, "F", "F", 4)   // 方向
	wb.File.SetColWidth(sheet, "G", "G", 12)  // 余额

	// 明细科目列宽
	for i := 0; i < len(details)*12; i++ {
		col := printMLColDetail + i
		if col > printMLTotalCols {
			break
		}
		wb.File.SetColWidth(sheet, cellName(col, 1), cellName(col, 1), 3)
	}

	return nil
}

// AppendPrintMLEntries 追加当月分录到对应的打印版多科目明细账 Sheet。
func (wb *Workbook) AppendPrintMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	// 按总账科目分组
	type mlGroup struct {
		entries []voucher.Entry
		details map[string]bool
	}
	groups := make(map[string]*mlGroup)

	for _, e := range entries {
		if e.DetailAccount == "" {
			continue
		}

		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlGroup{details: make(map[string]bool)}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		g.details[e.DetailAccount] = true
	}

	for general, g := range groups {
		// 获取明细科目列表
		var details []string
		for d := range g.details {
			details = append(details, d)
		}

		if err := wb.appendToPrintMLSheet(general, g.entries, details, initials); err != nil {
			return fmt.Errorf("追加打印版明细账 %s: %w", general, err)
		}
	}

	return nil
}

// appendToPrintMLSheet 将分录追加到指定总账科目的打印版多科目明细账 Sheet。
func (wb *Workbook) appendToPrintMLSheet(general string, entries []voucher.Entry, details []string, initials map[string]int64) error {
	sheet, err := wb.ensurePrintMLSheet(general, details)
	if err != nil {
		return err
	}

	rows, _ := wb.File.GetRows(sheet)

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

	// 计算总账级别的余额
	balance := initials[general]

	for _, e := range entries {
		row := len(rows) + 1
		if row <= 3 {
			row = 4
		}

		// 写入左页基础列
		wb.File.SetCellValue(sheet, cellName(printMLColDate, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(printMLColVoucher, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(printMLColSummary, row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(printMLColDebit, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(printMLColCredit, row), centsToYuan(e.CreditCents))

		// 计算余额
		balance = balance + e.DebitCents - e.CreditCents
		dir, dispBal := directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(printMLColDirection, row), dir)
		wb.File.SetCellValue(sheet, cellName(printMLColBalance, row), centsToYuan(dispBal))

		// 写入右页明细科目（分栏）
		for i, detail := range details {
			col := printMLColDetail + i*12
			if col+11 > printMLTotalCols {
				break
			}

			// 如果是当前分录的明细科目，写入金额
			if e.DetailAccount == detail {
				writeAmountCells(wb.File, sheet, row, col, e.DebitCents-e.CreditCents, dataStyleID)
			} else {
				// 留空
				for j := 0; j < 12; j++ {
					wb.File.SetCellValue(sheet, cellName(col+j, row), "")
				}
			}
		}

		// 应用样式到左页
		wb.File.SetCellStyle(sheet, cellName(printMLColDate, row), cellName(printMLColBalance, row), dataStyleID)

		rows, _ = wb.File.GetRows(sheet)
	}

	return nil
}
