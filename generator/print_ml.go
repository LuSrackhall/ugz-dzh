// Package generator — 打印版位格输出：多科目明细账变换配置。
package generator

import "github.com/xuri/excelize/v2"

// transformMLSheet 把单个多科目明细账 Sheet 变换为打印版位格布局。
//
// ML 金额列：Back 侧 借/贷/余额 + 明细1-4（7 列）+ Front 侧 明细5-14（10 列），共 17 列 → 展开 +187 列。
// 每页块（blockRows = DataStartRow+pageSize+1+BottomMarginRows = 30 行）均有独立四行表头，
// 标签行 = 每块的 h4 = blockStart + 7，即 (r - DataStartRow) % blockRows == 0（DataStartRow=8 → r=8,38,68,...）。
// 垂直分页符在 PageGapStartCol+2（Back 与 Front 装订区之间）。
func transformMLSheet(f *excelize.File, sheet string) error {
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
	}
	return transformSheet(f, sheet, cfg)
}
