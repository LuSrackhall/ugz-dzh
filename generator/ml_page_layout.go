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
	}

	// 页间隙（Back 区之后 1 列）
	setMMWidth(f, sheet, excelCol, gapMM)
	excelCol++

	// 右半：Front 数据区（明细5~14），合计 = A4 宽度
	frontTotal := 0.0
	for _, c := range lay.FrontColumns {
		frontTotal += c.WidthMM
	}
	frontScale := a4WidthMM / frontTotal
	for _, c := range lay.FrontColumns {
		setMMWidth(f, sheet, excelCol, c.WidthMM*frontScale)
		excelCol++
	}

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
