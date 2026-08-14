package generator

import (
	"fmt"
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// setMLSheetPageLayout 为多科目明细账 Sheet 设置 B5 横向打印布局（对齐 GL）。
// 左半（反面/Back）占一张 B5 纸宽度，右半（正面/Front）占一张 B5 纸宽度，
// 固定缩放 74%、显式分页符（垂直分页在正/反书口列之间，水平分页每页对），
// 消除"每次打印缩放/分页不一致"的问题。
func setMLSheetPageLayout(f *excelize.File) {
	paperSize := 13 // B5 (JIS)
	fp := false
	lay := mlLayout()
	// 垂直分页：正面书口列（PageGapStartCol+1）前分页，Back/反面各一张纸
	pbCell, _ := excelize.ColumnNumberToName(lay.PageGapStartCol + 1)
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
		// 固定缩放 74%：关闭 FitToPage 与 FitToWidth/Height，缩放不再每次打开重算
		scale := uint(74)
		fw := 0
		fh := 0
		f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
			Orientation: stringPtr("landscape"),
			Size:        &paperSize,
			AdjustTo:    &scale,
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
		f.InsertPageBreak(sheet, pbCell+"1")
		// 水平分页：每页对（30 行）起始行前分页，打印时一页对一张纸（对齐 GL）
		if rows, err := f.GetRows(sheet); err == nil {
			blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
			for start := 1 + blockRows; start <= len(rows); start += blockRows {
				f.InsertPageBreak(sheet, fmt.Sprintf("A%d", start))
			}
		}
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
// 下边距行高 16，与 GL 一致。
// 每页块 = 上边距 + DataStartRow 页头 + pageSize 数据 + 过次页 + 下边距 = 30 行，
// 块起始行依次为 1, 1+30, ...（Paper1 Front 也从 row 1 起）。
func setMLDataRowHeights(f *excelize.File, sheet string) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	const dataRowHeight = 25.0
	const bottomMarginHeight = 20.0 // 与 GL 一致
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	lastRow := len(rows)
	// 上限延伸到最后一块的下边距行（GetRows 不含空的下边距行，但该行由 applyMLBorders 创建）
	lastBlockStart := 1 + ((lastRow - 1) / blockRows) * blockRows
	maxRow := lastBlockStart + lay.DataStartRow + pageSize + lay.BottomMarginRows
	for start := 1; start <= lastRow; start += blockRows {
		dataStart := start + lay.DataStartRow
		dataEnd := dataStart + pageSize + lay.BottomMarginRows // 数据+过次页+下边距
		for r := dataStart; r <= dataEnd && r <= maxRow; r++ {
			h := dataRowHeight
			if r == dataEnd {
				h = bottomMarginHeight // 下边距行 16
			}
			f.SetRowHeight(sheet, r, h)
		}
	}
}

// setMLColumnWidths 设置列宽，使半页总列宽与 GL 一致（每半页 153.24）。
// 半页划分：左半 A-P、右半 P-AB，中间装订列 P 为共享（两半均含 P）。
// 边距与 GL 结构相反但总宽一致：GL 装订=2列×7（总14）、非装订=1列×2；
// ML 装订=中间 1 共享列 P=14，非装订=两侧 2 列各 1（总2）。
// 金额栏（借/贷/余/明细1-14）宽度统一；摘要列吸收差额使两半同宽。
func setMLColumnWidths(f *excelize.File, sheet string) {
	lay := mlLayout()
	// 半页总列宽对齐 GL（152.54）。GL 数据区 135.84，ML 的 Back/Front 数据区各对齐之。
	// 打印机补偿复用 GL 常量（glBackBindColW/glFrontBindColW/glBackGutterW/glFrontGutterW）：
	//   - 左侧装订 = 反面 Back 纸的装订边 → glBackBindColW（7.75）
	//   - 右侧装订 = 正面 Front 纸的装订边 → glFrontBindColW（8.35）
	//   - col16 反面书口 → glBackGutterW（1.2）；col17 正面书口 → glFrontGutterW（0）
	// 注意：ML 正面（Front）在右、反面（Back）在左，与 GL 的左右顺序相反。
	const backDataW = 135.84
	const frontDataW = 135.84
	const glDateVouchColW = 3.0
	const dirRatio = 1.1
	dirNew := glDateVouchColW * dirRatio // 方向 = 3.3
	frontW := 13.724                     // Back 金额列（借/贷/余额/明细1-4）
	frontW2 := frontDataW / 10.0         // Front 明细列（明细5-14，均摊）
	// Back 摘要列吸收差额：135.84 - 月日字号(4×3) - 金额7列(7×13.724) - 方向(3.3)
	sumNew := backDataW - 4*glDateVouchColW - 7*frontW - dirNew

	// 左侧装订（反面 Back，打印机补偿见 glBackBindColW）
	f.SetColWidth(sheet, cellColLetter(1), cellColLetter(2), glBackBindColW)
	// Back 基础列
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol), cellColLetter(lay.BackStartCol+1), glDateVouchColW)             // 月 日
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+2), cellColLetter(lay.BackStartCol+3), glDateVouchColW)             // 字 号
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffSummary), cellColLetter(lay.BackStartCol+mlOffSummary), sumNew) // 摘要
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDebit), cellColLetter(lay.BackStartCol+mlOffDebit), frontW)     // 借
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffCredit), cellColLetter(lay.BackStartCol+mlOffCredit), frontW)   // 贷
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffDir), cellColLetter(lay.BackStartCol+mlOffDir), dirNew)         // 方向
	f.SetColWidth(sheet, cellColLetter(lay.BackStartCol+mlOffBalance), cellColLetter(lay.BackStartCol+mlOffBalance), frontW) // 余额
	// 明细1-4（Back 侧）
	for i := 0; i < 4; i++ {
		f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, i)), cellColLetter(mlDetailCol(lay, i)), frontW)
	}
	// 中间书口：PageGapStartCol = 反面书口（Back 纸右缘），+1 = 正面书口（Front 纸左缘）
	f.SetColWidth(sheet, cellColLetter(lay.PageGapStartCol), cellColLetter(lay.PageGapStartCol), glBackGutterW)
	f.SetColWidth(sheet, cellColLetter(lay.PageGapStartCol+1), cellColLetter(lay.PageGapStartCol+1), glFrontGutterW)
	// 明细5-14（Front 侧）
	for i := 4; i < mlMaxDetails; i++ {
		f.SetColWidth(sheet, cellColLetter(mlDetailCol(lay, i)), cellColLetter(mlDetailCol(lay, i)), frontW2)
	}
	// 右侧装订（正面 Front，打印机补偿见 glFrontBindColW）
	rightStart := mlDetailCol(lay, mlMaxDetails-1) + 1
	f.SetColWidth(sheet, cellColLetter(rightStart), cellColLetter(rightStart+1), glFrontBindColW)
}

// setMMWidth 将 mm 宽度转换为 Excel 列宽单位并设置。
func setMMWidth(f *excelize.File, sheet string, col int, mm float64) {
	f.SetColWidth(sheet, cellColLetter(col), cellColLetter(col), layout.MLMMToExcelColWidth(mm))
}
