package generator

import (
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// setMLSheetPageLayout 为多科目明细账 Sheet 设置 A4 横向打印布局（独立于 GL）。
// 左半（反面/Back）占一张 A4 纸宽度，右半（正面/Front）占一张 A4 纸宽度，
// 高度不限制，按每页 20 数据行+1 过次页 纵向自然分页（有几页出几张）。
func setMLSheetPageLayout(f *excelize.File) {
	paperSize := 9 // A4
	fp := true
	// 摘要列数据行字体：9号加粗+自动换行+左对齐（参考 GL 摘要列数据区配置）
	summaryStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
	})
	// 借或贷列数据区：横向+纵向居中
	dirAlignStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	for _, sheet := range f.GetSheetList() {
		if !strings.HasPrefix(sheet, sheetPrefixML) {
			continue
		}
		// 宽度不缩放：关闭 FitToWidth，让 Excel 按列宽在中间自然分页
		fw := 0
		fh := 0
		f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
			Orientation: stringPtr("landscape"),
			Size:        &paperSize,
			FitToWidth:  &fw,
			FitToHeight: &fh,
		})
		f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
			Top:    float64Ptr(0),
			Bottom: float64Ptr(0),
			Left:   float64Ptr(0),
			Right:  float64Ptr(0),
		})
		f.SetSheetProps(sheet, &excelize.SheetPropsOptions{
			FitToPage: &fp,
		})
		setMLColumnWidths(f, sheet)
		setMLDataRowHeights(f, sheet)
		setMLSummaryFonts(f, sheet, summaryStyle)
		setMLDirAlignment(f, sheet, dirAlignStyle)
	}
}

// setMLDirAlignment 将 借或贷 列数据区（数据行 + 过次页行）设为横向+纵向居中。
// 跳过每页页头（含表头"借或贷"），页头样式由 writeMLPageHeader 独立管理。
func setMLDirAlignment(f *excelize.File, sheet string, dirStyle int) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	blockRows := lay.DataStartRow + pageSize + 1
	lastRow := len(rows)
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize // 含过次页行
		for r := dataStart; r <= dataEnd && r <= lastRow; r++ {
			f.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffDir, r),
				mlCellName(lay.BackStartCol+mlOffDir, r), dirStyle)
		}
	}
}

// setMLSummaryFonts 为数据区摘要列应用 9号加粗+自动换行+左对齐。
// 跳过表头（摘要）、过次页、承前页行，与 GL 摘要列数据区配置一致。
func setMLSummaryFonts(f *excelize.File, sheet string, summaryStyle int) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	sumIdx := lay.BindingLeftCols + mlOffSummary
	for i, r := range rows {
		if len(r) <= sumIdx {
			continue
		}
		v := strings.TrimSpace(r[sumIdx])
		if v == "" || v == pageBreakLabel || v == carryForwardLabel {
			continue
		}
		if strings.HasPrefix(v, "摘") {
			continue
		}
		f.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffSummary, i+1),
			mlCellName(lay.BackStartCol+mlOffSummary, i+1), summaryStyle)
	}
}

// setMLDataRowHeights 将每页内容区（20 数据行 + 1 过次页行）的行高统一设为 25pt。
// 每页块 = DataStartRow 页头 + pageSize 数据行 + 1 过次页 = 29 行，
// 块起始行依次为 1, 1+29, ...（Paper1 Front 也从 row 1 起，占 rows 1-29）。
func setMLDataRowHeights(f *excelize.File, sheet string) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	const dataRowHeight = 25.0
	blockRows := lay.DataStartRow + pageSize + 1
	lastRow := len(rows)
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize // 含过次页行
		for r := dataStart; r <= dataEnd && r <= lastRow; r++ {
			f.SetRowHeight(sheet, r, dataRowHeight)
		}
	}
}

// setMLColumnWidths 按 ML 布局比例设置列宽，使左半（Back）和右半（Front）各占一张 A4 横向宽度。
func setMLColumnWidths(f *excelize.File, sheet string) {
	lay := mlLayout()
	const a4WidthMM = 297.0 // A4 横向宽度
	const bindMM = 5.0      // 装订边宽度
	const gapMM = 2.0       // 左右页间隙宽度

	// 左半：装订边 A-B + Back 数据区（C 起）
	backTarget := a4WidthMM - 2*bindMM // 287mm 给 Back 数据区
	backTotal := 0.0
	for _, c := range lay.BackColumns {
		backTotal += c.WidthMM
	}
	backScale := backTarget / backTotal

	setMMWidth(f, sheet, 1, bindMM) // A 装订
	setMMWidth(f, sheet, 2, bindMM) // B 装订

	excelCol := lay.BackStartCol // C=3
	var dateW, vouchW, sumW, dirW float64
	for _, c := range lay.BackColumns {
		w := c.WidthMM * backScale
		// 日期(月/日)、凭证号(字/号)各覆盖 2 个 Excel 列
		span := 1
		if c.Name == "日期" || c.Name == "凭证号" {
			span = 2
		}
		for i := 0; i < span; i++ {
			setMMWidth(f, sheet, excelCol, w/float64(span))
			excelCol++
		}
		if c.Name == "日期" {
			dateW = layout.MLMMToExcelColWidth(w / float64(span))
		} else if c.Name == "凭证号" {
			vouchW = layout.MLMMToExcelColWidth(w / float64(span))
		} else if c.Name == "摘要" {
			sumW = layout.MLMMToExcelColWidth(w)
		} else if c.Name == "方向" {
			dirW = layout.MLMMToExcelColWidth(w)
		}
	}

	// 年/凭证列宽参考 GL：月/日/字/号各 3 单位（合计 6 单位）；
	// 借或贷列宽 = 月/日/字/号列宽的 1.1 倍。三者释放的宽度给摘要列（后续被金额栏统一重算覆盖）。
	const glDateVouchColW = 3.0
	const dirRatio = 1.1
	dirNew := glDateVouchColW * dirRatio // = 3.3
	freed := 2*(dateW-glDateVouchColW) + 2*(vouchW-glDateVouchColW) + (dirW - dirNew)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol), cellColLetter(lay.BackStartCol+1), glDateVouchColW)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+2), cellColLetter(lay.BackStartCol+3), glDateVouchColW)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDir), cellColLetter(lay.BackStartCol+mlOffDir), dirNew)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+4), cellColLetter(lay.BackStartCol+4), sumW+freed)

	// 页间隙（Back 区之后 1 列）
	gapW := layout.MLMMToExcelColWidth(gapMM)
	setMMWidth(f, sheet, excelCol, gapMM)
	excelCol++

	// 右半：Front 数据区（明细5~14），合计 = A4 宽度
	frontTotal := 0.0
	for _, c := range lay.FrontColumns {
		frontTotal += c.WidthMM
	}
	frontScale := a4WidthMM / frontTotal
	var frontW float64
	for _, c := range lay.FrontColumns {
		setMMWidth(f, sheet, excelCol, c.WidthMM*frontScale)
		frontW = layout.MLMMToExcelColWidth(c.WidthMM * frontScale)
		excelCol++
	}

	// 金额栏列宽统一（强约束）：借方、贷方、余额、明细1-14 全部等于 Front 明细列宽 frontW。
	// 摘要列吸收差额，使 Back 半页与 Front 半页同宽（均为一页 A4）。
	bindW := layout.MLMMToExcelColWidth(bindMM)
	sumNew := gapW + 3*frontW - (2*bindW + 2*glDateVouchColW + 2*glDateVouchColW + dirNew)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDebit), cellColLetter(lay.BackStartCol+mlOffDebit), frontW)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffCredit), cellColLetter(lay.BackStartCol+mlOffCredit), frontW)
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffBalance), cellColLetter(lay.BackStartCol+mlOffBalance), frontW)
	for i := 0; i < mlMaxDetails; i++ {
		f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, i)), cellColLetter(mlDetailCol(lay, i)), frontW)
	}
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffSummary), cellColLetter(lay.BackStartCol+mlOffSummary), sumNew)

	// 右装订边（Front 区之后，若有剩余列）
	for excelCol <= lay.FrontStartCol+lay.FrontColCount+1 {
		setMMWidth(f, sheet, excelCol, bindMM)
		excelCol++
	}
}

// setMMWidth 将 mm 宽度转换为 Excel 列宽单位并设置。
func setMMWidth(f *excelize.File, sheet string, col int, mm float64) {
	f.SetColWidth(sheet, cellColLetter(col), cellColLetter(col), layout.MLMMToExcelColWidth(mm))
}
