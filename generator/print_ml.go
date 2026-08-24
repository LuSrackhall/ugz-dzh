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
	// 用户定值（2026-08-24 十次调整）：基础列宽 14px；组内任意位置独立像素——
	//   借/贷/余（Back 侧，n=11）：分列 k=10 15px
	//   明细1-4（Back 侧，n=10）：分列 k=9 16px
	//   明细5-14（Front 侧，n=10）：k=0(千万位千) 16、k=1(百万位百) 16、k=4(千位千) 16、k=9(分) 16
	// 标签 6pt、数字 7pt。
	edgePixel := map[[2]int]float64{}
	for i, c := range amountCols {
		switch {
		case i < 3: // 借/贷/余
			edgePixel[[2]int{c, 10}] = 15
		case i < 7: // 明1-4
			edgePixel[[2]int{c, 9}] = 16
		default: // 明5-14
			edgePixel[[2]int{c, 0}] = 16 // 千万位千
			edgePixel[[2]int{c, 1}] = 16 // 百万位百
			edgePixel[[2]int{c, 4}] = 16 // 千位千
			edgePixel[[2]int{c, 9}] = 16 // 分
		}
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
		labelFontSize:   6,
	}
	return transformSheet(f, sheet, cfg)
}
