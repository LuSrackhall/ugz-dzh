// Package generator — 打印版位格输出：多科目明细账变换配置。
package generator

import "github.com/xuri/excelize/v2"

// transformMLSheet 把单个多科目明细账 Sheet 变换为打印版位格布局。
//
// ML 金额列：Back 侧 借/贷/余额 + 明细1-4（7 列）+ Front 侧 明细5-14（10 列），共 17 列 → 展开 +187 列。
// 每页块（blockRows = DataStartRow+pageSize+1+BottomMarginRows = 30 行）均有独立四行表头，
// 标签行 = 每块的 h4 = blockStart + 7，即 (r - DataStartRow) % blockRows == 0（DataStartRow=8 → r=8,38,68,...）。
// 垂直分页符在 PageGapStartCol+2（Back 与 Front 装订区之间）。
func transformMLSheet(f *excelize.File, sheet string) ([]areaRect, error) {
	lay := mlLayout()
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows

	amountCols := []int{
		lay.BackStartCol + mlOffDebit,
		lay.BackStartCol + mlOffCredit,
		lay.BackStartCol + mlOffBalance,
	}
	for i := 0; i < 4; i++ { // 明细1-4（Back 侧）
		amountCols = append(amountCols, mlDetailCol(lay, i))
	}
	for i := 4; i < mlMaxDetails; i++ { // 明细5-14（Front 侧）
		amountCols = append(amountCols, mlDetailCol(lay, i))
	}

	dataFirst := lay.DataStartRow + 1 // 首块数据首行（h4 标签行下一行）
	// 列数：借/贷/余 11 列（去十亿位）；明细列 10 列（去十亿/亿位，最多千万）。
	// 依据：9 个月数据最大金额 2418 万 < 明细 10 列上限 9999 万；借/贷/余 11 列上限 99.9 亿。
	split := make(map[int]int, len(amountCols))
	for _, c := range amountCols {
		split[c] = 11
	}
	for i := 0; i < mlMaxDetails; i++ {
		split[mlDetailCol(lay, i)] = 10
	}
	// 用户定值（2026-08-24 十四次调整）：基础列宽 14px；组内任意位置独立像素——
	//   借/贷/余（Back 侧，n=11：亿0 千1 百2 十3 万4 千5 百6 十7 元8 角9 分10）：
	//     k=0(亿) 16px、k=10(分) 15px（百2/千5 曾试 15px，已调回 14）
	//   明细1-4（Back 侧，n=10：千0 百1 十2 万3 千4 百5 十6 元7 角8 分9）：
	//     k=0(千万位千) 16、k=1(百万位百) 15、k=4(千位千) 15、k=9(分) 16
	//   明细5-14（Front 侧，n=10）：k=0(千万位千) 16、k=1(百万位百) 16、k=4(千位千) 16、k=9(分) 16
	//   借或贷列（非金额，查看版 col9）：28.1px → 26.1px（减 2px）
	// 标签 6pt、数字 7pt。
	edgePixel := map[[2]int]float64{}
	for i, c := range amountCols {
		switch {
		case i < 3: // 借/贷/余（n=11，索引较 n=10 右移一位）
			edgePixel[[2]int{c, 0}] = 16  // 亿
			edgePixel[[2]int{c, 10}] = 15 // 分
		case i < 7: // 明1-4
			edgePixel[[2]int{c, 0}] = 16 // 千万位千
			edgePixel[[2]int{c, 1}] = 15 // 百万位百
			edgePixel[[2]int{c, 4}] = 15 // 千位千
			edgePixel[[2]int{c, 9}] = 16 // 分
		default: // 明5-14
			edgePixel[[2]int{c, 0}] = 16 // 千万位千
			edgePixel[[2]int{c, 1}] = 16 // 百万位百
			edgePixel[[2]int{c, 4}] = 16 // 千位千
			edgePixel[[2]int{c, 9}] = 16 // 分
		}
	}
	// 非金额列特例：借或贷列（查看版 col9 = BackStartCol+mlOffDir）减 2px
	nonAmountPixel := map[int]float64{
		lay.BackStartCol + mlOffDir: 26.1,
	}
	cfg := printSheetConfig{
		totalViewCols: lay.TotalCols,
		amountCols:    amountCols,
		splitCols:     split,
		isLabelRow: func(r int) bool {
			if blockRows <= 0 || r < lay.DataStartRow {
				return false
			}
			return (r-lay.DataStartRow)%blockRows == 0
		},
		isDataRow: func(r int) bool {
			// 数据区 = 数据行 + 过次页行（pageSize+1 行）；下边距行不含在内
			if r < dataFirst || blockRows <= 0 {
				return false
			}
			return (r-dataFirst)%blockRows < pageSize+1
		},
		breakViewCol:    lay.PageGapStartCol + 2,
		applyPageLayout: applyMLPrintPageLayout,
		amountColPixel:  14,
		edgePixel:       edgePixel,
		nonAmountPixel:  nonAmountPixel,
		labelFontSize:   6,
		// 数据区金额数字：6pt，字体可配（print-config.json 字体.数字；默认 Noteworthy；表头标签仍 6pt 宋体）
		dataFontFamily: currentFonts().Digit,
		dataFontSize:   6,
		postProcess:    applyMLTitleArea,
	}
	// 摘要/借/贷/余额 表头字体样式 + 金额区域列数字样式（print-config.json fonts 扩展）
	fc := currentFonts()
	cfg.labelFontSizeDefault = 6 // ML 原默认标签字号（配置过 labelSize 时非目标列回退用）
	if fc.LabelSize != 0 {
		cfg.labelFontSize = fc.LabelSize
		cfg.labelSizeOverride = true // 配置过：labelSize 仅作用于摘要/借/贷/余额表头
	}
	if fc.DigitSize != 0 {
		cfg.dataFontSize = fc.DigitSize
	}
	cfg.labelBold = fc.LabelBold
	cfg.labelFamily = fc.LabelFamily
	// 正反面页独立列宽系数（Back 侧=借/贷/余+明细1-4；Front 侧=明细5-14；装订区列不属任一半侧）
	cfg.frontColScale, cfg.backColScale = sheetColScales()
	cfg.isBackCol = func(c int) bool { return c >= lay.BackStartCol && c < lay.BackStartCol+lay.BackColCount }
	cfg.isFrontCol = func(c int) bool { return c >= lay.FrontStartCol && c < lay.FrontStartCol+lay.FrontColCount }
	// 正反面页微调作用列（每个边只作用一列；金额拆位列不参与）
	//   反面(Back)：摘要列6 / 装订边列16 / 非装订边(书口)列1
	//   正面(Front)：无摘要列 / 装订边列17 / 非装订边(书口)列29
	if wd := sheetWidthDeltas(); wd != (widthDeltas{}) {
		cfg.colPxDelta = map[int]float64{
			lay.BackStartCol + mlOffSummary: wd.BackSummary,
			lay.PageGapStartCol + 1:         wd.BackBinding,  // 列16（Back 装订靠装订线）
			lay.BindingLeftCols:             wd.BackOuter,    // 列1（Back 书口）
			lay.PageGapStartCol + 2:         wd.FrontBinding, // 列17（Front 装订靠装订线）
			lay.TotalCols:                   wd.FrontOuter,   // 列29（Front 书口）
		}
	}
	// 正面页金额分位尺寸微调（frontDigitDelta）：单拎代表位，作用于所有明细列的同一位
	if fd := sheetFrontDigitDelta(); len(fd) > 0 {
		cfg.amountPxDelta = map[[2]int]float64{}
		for i := 4; i < mlMaxDetails; i++ { // 明细5-14（Front 半侧）
			c := mlDetailCol(lay, i)
			for k := 0; k < 10; k++ {
				key := "base"
				if k == 0 {
					key = "k0"
				} else if k == 1 {
					key = "k1"
				} else if k == 4 {
					key = "k4"
				} else if k == 9 {
					key = "k9"
				}
				if d, ok := fd[key]; ok && d != 0 {
					cfg.amountPxDelta[[2]int{c, k}] = d
				}
			}
		}
	}
	// 表头区 = 四行表头（DataStartRow-4 .. DataStartRow-1）+ h4 标签行（DataStartRow）
	cfg.isHeaderRow = func(r int) bool {
		start := lay.DataStartRow - 4
		if r < start || blockRows <= 0 {
			return false
		}
		return (r-start)%blockRows <= 4
	}
	cfg.digitBold = fc.DigitBold
	cfg.labelCols = map[int]bool{
		lay.BackStartCol + mlOffSummary: true,
		lay.BackStartCol + mlOffDebit:   true,
		lay.BackStartCol + mlOffCredit:  true,
		lay.BackStartCol + mlOffBalance: true,
	}
	// 多区域打印区域：滑动窗口阅读序 [占位正面, 反1, 正2, 反2 …]，页数恒为偶数
	cfg.planAreas = func(breakPrintCol, maxCol, lastRow int) []areaRect {
		return mlAreaPlan(lastRow, blockRows, breakPrintCol, maxCol)
	}
	return transformSheet(f, sheet, cfg)
}

// applyMLTitleArea 打印版 ML 标题区后处理（对每个页面块的标题行）：
//  1. Front "明      细      帐"：字号 18、仿宋、左对齐+底部对齐、明字前加一个空格；
//     底部绿色双边从明细7框延长到明细8的十万分位表头（合并单元格也延伸到此）；
//  2. 页码"分第 n 页"：分第 合并明细12整块并右对齐；数字 n 合并明细13整块并居中。
//
// 说明：标题区只存在于查看版的标题行（Paper1 行2 + 各数据页块首行+1），打印版行号不变；
// 明细列展开后各块起列由 colMap 给出；新增样式显式指定字体族，applyPrintFont 不会覆盖。
func applyMLTitleArea(f *excelize.File, sheet string, cm colMap, maxRow int) {
	lay := mlLayout()
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows
	// Front 侧关键明细列（查看版列号 → 打印版起列）
	d5 := cm.startCol(mlDetailCol(lay, 4))  // 明细5（标题起始）
	d8 := cm.startCol(mlDetailCol(lay, 7))  // 明细8
	d12 := cm.startCol(mlDetailCol(lay, 11)) // 明细12（分第）
	d13 := cm.startCol(mlDetailCol(lay, 12)) // 明细13（页码数字）
	// 明细8 的十万分位（n=10 标签序列：千0 百1 十2 万3 …；十万 = k2）
	d8ShiWan := d8 + 2

	// Front 标题样式：18pt 仿宋、左/底对齐、绿色底双边
	titleStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: currentFonts().Title, Size: 18, Color: "006100", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
		Border:    []excelize.Border{{Type: "bottom", Color: "006100", Style: 6}},
	})
	if err != nil {
		return
	}
	// 明细5 块首列（装订边）需保留红色双线左框——标题行原样式含该边框，
	// 若用 titleStyle 整体覆盖会把它抹掉（用户反馈过此回归）。
	titleStyleLeft, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: currentFonts().Title, Size: 18, Color: "006100", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "006100", Style: 6},
			{Type: "left", Color: mlRed, Style: mlBorderDouble},
		},
	})
	if err != nil {
		return
	}

	// 标题行：Paper1 = 行 2；数据页块首行 = 1 + blockRows*k（k≥1），标题行 = 块首+1
	titleRows := []int{2}
	for start := 1 + blockRows; start <= maxRow; start += blockRows {
		titleRows = append(titleRows, start+1)
	}
	for _, r := range titleRows {
		// 1) Front 标题：解除文本标签分支生成的"明细5 单块合并"，重合并延伸至明细8 十万位
		a1 := cellAxis(d5, r)
		_ = f.UnmergeCell(sheet, a1, cellAxis(d5+9, r))
		_ = f.SetCellValue(sheet, a1, " 明      细      帐")
		// 首列用 titleStyleLeft（含装订边红色双线左框），其余列用 titleStyle（仅底双边）
		_ = f.SetCellStyle(sheet, a1, a1, titleStyleLeft)
		_ = f.SetCellStyle(sheet, cellAxis(d5+1, r), cellAxis(d8ShiWan, r), titleStyle)
		_ = f.MergeCell(sheet, a1, cellAxis(d8ShiWan, r))
		// 2) 页码：分第 合并明细12整块右对齐（样式已右对齐）；数字 合并明细13整块居中
		_ = f.MergeCell(sheet, cellAxis(d12, r), cellAxis(d12+9, r))
		_ = f.MergeCell(sheet, cellAxis(d13, r), cellAxis(d13+9, r))
	}
}
