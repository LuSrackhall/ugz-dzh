package generator

// markRowForPrint 在隐藏列 H 标记指定行为"需打印"（总分类账用）。
func (wb *Workbook) markRowForPrint(sheet string, row int) {
	wb.File.SetCellValue(sheet, cellName(8, row), "需打印")
}

// markMLRowForPrint 在多科目明细账的最末明细列右侧标记打印。
func (wb *Workbook) markMLRowForPrint(sheet string, row, numDetails int) {
	wb.File.SetCellValue(sheet, cellName(mlPrintMarkCol(numDetails), row), "需打印")
}

// markRowsForPrint 标记从 startRow 到 endRow 的行为"需打印"。
func (wb *Workbook) markRowsForPrint(sheet string, startRow, endRow int) {
	for r := startRow; r <= endRow; r++ {
		wb.markRowForPrint(sheet, r)
	}
}

// markExistingPageForPrint 将当前页中已有的数据行标记为"需打印"（跨月未满页场景）。
// sheet 中已经存在一些数据行但没有过次页行，说明上月的页未满。
func (wb *Workbook) markExistingPageForPrint(sheet string) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return
	}

	// 找到最后一页的起始行
	pageStart := wb.pageStartRow(sheet)
	// 标记从 pageStart 到最后一行的所有数据行
	lastRow := len(rows)
	for r := pageStart; r <= lastRow; r++ {
		// 跳过过次页行（在摘要列 C）
		if r <= len(rows) && len(rows[r-1]) > 2 && rows[r-1][2] == pageBreakLabel {
			continue
		}
		wb.markRowForPrint(sheet, r)
	}
}

// markExistingMLPageForPrint 多科目明细账版：标记已有数据行（V 列）。
func (wb *Workbook) markExistingMLPageForPrint(sheet string, numDetails int) {
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return
	}

	pageStart := wb.pageStartRow(sheet)
	lastRow := len(rows)
	for r := pageStart; r <= lastRow; r++ {
		if r <= len(rows) && len(rows[r-1]) > 2 && rows[r-1][2] == pageBreakLabel {
			continue
		}
		wb.markMLRowForPrint(sheet, r, numDetails)
	}
}
