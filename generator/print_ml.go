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

	cfg := printSheetConfig{
		totalViewCols: lay.TotalCols,
		amountCols:    amountCols,
		isLabelRow: func(r int) bool {
			if blockRows <= 0 || r < lay.DataStartRow {
				return false
			}
			return (r-lay.DataStartRow)%blockRows == 0
		},
		breakViewCol:    lay.PageGapStartCol + 2,
		applyPageLayout: applyMLPrintPageLayout,
	}
	return transformSheet(f, sheet, cfg)
}
