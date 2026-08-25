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
	dataFirst := labelRow1 + 1 // 首块数据首行（标签行下一行）
	// 借/贷/余额 12 小列（十亿…分）中，不减少的列 = 十亿位 k=0（表头"十"）、
	// 百万位 k=3（表头"百"）、千位 k=6（表头"千"）；其余 k 各减 0.3px（用户定值）。
	edgePixelDelta := map[[2]int]float64{}
	for _, c := range amountCols {
		for k := 0; k < 12; k++ {
			switch k {
			case 0, 3, 6: // 十亿位/百万位/千位 表头列不减少
			default:
				edgePixelDelta[[2]int{c, k}] = -0.3
			}
		}
	}
	cfg := printSheetConfig{
		totalViewCols:   lay.TotalCols,
		amountCols:      amountCols,
		edgePixelDelta:  edgePixelDelta,
		isLabelRow: func(r int) bool {
			if r < labelRow1 || blockRows <= 0 {
				return false
			}
			return (r-labelRow1)%blockRows == 0
		},
		isDataRow: func(r int) bool {
			// 数据区 = 数据行 + 过次页行（pageSize+1 行）；下边距行不含在内
			if r < dataFirst || blockRows <= 0 {
				return false
			}
			return (r-dataFirst)%blockRows < pageSize+1
		},
		breakViewCol:    lay.PageGapStartCol + 1,
		applyPageLayout: applyGLPrintPageLayout,
	}
	return transformSheet(f, sheet, cfg)
}
