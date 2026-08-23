// Package generator — 打印版位格输出：总分类账（含合并总账）变换配置。
package generator

import "github.com/xuri/excelize/v2"

// transformGLSheet 把单个总分类账 Sheet（含合并总账）变换为打印版位格布局。
//
// GL 每面（Front/Back）各 3 个金额列（借/贷/余额），共 6 个金额列 → 展开 +66 列。
// GL 为多页块结构：每块 blockRows = (SubHeaderRow+1) + pageSize + 1 + BottomMarginRows 行，
// 交替 Front/Back，每块自有 HeaderRow（大标题，由文本标签分支合并 12 列）与 SubHeaderRow（标签行）。
// 标签行 = 每块的 SubHeaderRow+1，即 (r - SubHeaderRow - 1) % blockRows == 0。
// 垂直分页符在 PageGapStartCol+1（Front 与 Back 列区之间）。
func transformGLSheet(f *excelize.File, sheet string) error {
	lay := glLayout()
	amountCols := []int{
		lay.FrontStartCol + glColDebit,
		lay.FrontStartCol + glColCredit,
		lay.FrontStartCol + glColBalance,
		lay.BackStartCol + glColDebit,
		lay.BackStartCol + glColCredit,
		lay.BackStartCol + glColBalance,
	}
	labelRow1 := lay.SubHeaderRow + 1 // 首块标签行（绝对行号）
	blockRows := (lay.SubHeaderRow + 1) + pageSize + 1 + lay.BottomMarginRows
	cfg := printSheetConfig{
		totalViewCols: lay.TotalCols,
		amountCols:    amountCols,
		isLabelRow: func(r int) bool {
			if r < labelRow1 || blockRows <= 0 {
				return false
			}
			return (r-labelRow1)%blockRows == 0
		},
		breakViewCol:    lay.PageGapStartCol + 1,
		applyPageLayout: applyGLPrintPageLayout,
	}
	return transformSheet(f, sheet, cfg)
}
