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
		// 折中B：金额小列目标渲染宽 10px（列宽 0.714 字符）——借/贷/余区域≈110px(+9%)、
		// 明细区域≈100px(-1%)，与查看版金额区域（101px）接近；配合标签 5pt 放得下。
		amountColPixel: 10,
		labelFontSize:  5,
	}
	return transformSheet(f, sheet, cfg)
}
