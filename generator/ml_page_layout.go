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
		applyMLBorders(f, sheet)
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
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	lastRow := len(rows)
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize // 数据 + 过次页行
		for r := dataStart; r <= dataEnd && r <= lastRow; r++ {
			f.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffDir, r),
				mlCellName(lay.BackStartCol+mlOffDir, r), dirStyle)
		}
	}
}

// setMLSummaryFonts 为数据区摘要列应用 9号加粗+自动换行+左对齐。
// 仅作用于每页内容区（数据行+过次页行），跳过页头/标题区，与 GL 摘要列数据区配置一致。
func setMLSummaryFonts(f *excelize.File, sheet string, summaryStyle int) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	sumIdx := lay.BindingLeftCols + mlOffSummary
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	lastRow := len(rows)
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize + lay.BottomMarginRows // 数据+过次页+下边距
		for r := dataStart; r <= dataEnd && r <= lastRow; r++ {
			row := rows[r-1]
			if len(row) <= sumIdx {
				continue
			}
			v := strings.TrimSpace(row[sumIdx])
			if v == "" || v == pageBreakLabel || v == carryForwardLabel {
				continue
			}
			if strings.HasPrefix(v, "摘") {
				continue
			}
			f.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffSummary, r),
				mlCellName(lay.BackStartCol+mlOffSummary, r), summaryStyle)
		}
	}
}

// setMLDataRowHeights 将每页内容区（20 数据行 + 1 过次页行 + 1 下边距行）的行高统一设为 25pt。
// 每页块 = 上边距 + DataStartRow 页头 + pageSize 数据 + 过次页 + 下边距 = 31 行，
// 块起始行依次为 1, 1+31, ...（Paper1 Front 也从 row 1 起）。
func setMLDataRowHeights(f *excelize.File, sheet string) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	const dataRowHeight = 25.0
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	lastRow := len(rows)
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize + lay.BottomMarginRows // 数据+过次页+下边距
		for r := dataStart; r <= dataEnd && r <= lastRow; r++ {
			f.SetRowHeight(sheet, r, dataRowHeight)
		}
	}
}

// setMLColumnWidths 设置列宽，使左半（Back）和右半（Front）各占一张 A4。
// 边距角色：上/下边距行与 GL 一致；中间共享间隙是装订边（宽），两侧外缘是窄边（非装订，与 GL 相反）。
// 金额栏（借/贷/余/明细1-14）宽度统一；摘要列吸收差额使两半同宽。
func setMLColumnWidths(f *excelize.File, sheet string) {
	lay := mlLayout()
	const a4HalfUnits = 160.0   // 一页 A4 横向 ≈ 160 Excel 宽度单位
	const edgeW = 2.0           // 外缘窄边（书口，非装订）
	const bindW = 7.0           // 中间装订边（共享）
	const glDateVouchColW = 3.0 // 月/日/字/号（GL：各 3）
	const dirRatio = 1.1
	dirNew := glDateVouchColW * dirRatio // 借或贷 = 3.3

	// Front 半页 = 10 明细列 + 右侧外缘 2 列，合计一页 A4
	frontW := (a4HalfUnits - 2*edgeW) / 10.0
	// Back 半页 = 左侧外缘2 + 日期2×3 + 凭证2×3 + 摘要 + 方向 + 7金额 + 中间装订
	sumNew := a4HalfUnits - 2*edgeW - bindW - (2*glDateVouchColW + 2*glDateVouchColW + dirNew) - 7*frontW

	// 左外缘 A-B（窄边，非装订）
	f.SetColWidth(sheet, cellColLetter(1), cellColLetter(2), edgeW)
	// Back 基础列
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol), cellColLetter(lay.BackStartCol+1), glDateVouchColW)                 // 月 日
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+2), cellColLetter(lay.BackStartCol+3), glDateVouchColW)                 // 字 号
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffSummary), cellColLetter(lay.BackStartCol+mlOffSummary), sumNew)     // 摘要
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDebit), cellColLetter(lay.BackStartCol+mlOffDebit), frontW)         // 借
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffCredit), cellColLetter(lay.BackStartCol+mlOffCredit), frontW)       // 贷
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDir), cellColLetter(lay.BackStartCol+mlOffDir), dirNew)             // 方向
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffBalance), cellColLetter(lay.BackStartCol+mlOffBalance), frontW)     // 余额
	// 明细1-4（Back 侧）
	for i := 0; i < 4; i++ {
		f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, i)), cellColLetter(mlDetailCol(lay, i)), frontW)
	}
	// 中间装订边（Back 区之后，共享）
	f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, 3)+1), cellColLetter(mlDetailCol(lay, 3)+1), bindW)
	// 明细5-14（Front 侧）
	for i := 4; i < mlMaxDetails; i++ {
		f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, i)), cellColLetter(mlDetailCol(lay, i)), frontW)
	}
	// 右外缘（Front 区之后 2 列，窄边）
	rightStart := mlDetailCol(lay, mlMaxDetails-1) + 1
	f.SetColWidth(sheet, cellColLetter(rightStart), cellColLetter(rightStart+1), edgeW)
}

// setMMWidth 将 mm 宽度转换为 Excel 列宽单位并设置。
func setMMWidth(f *excelize.File, sheet string, col int, mm float64) {
	f.SetColWidth(sheet, cellColLetter(col), cellColLetter(col), layout.MLMMToExcelColWidth(mm))
}
