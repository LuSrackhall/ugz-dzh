package layout

import "testing"

func TestDefaultMLSpec_HasBackFront(t *testing.T) {
	spec := DefaultMLSpec()
	if len(spec.BackColProportions) == 0 {
		t.Error("BackColProportions 不应为空")
	}
	if len(spec.FrontColProportions) == 0 {
		t.Error("FrontColProportions 不应为空")
	}
	if spec.BackColProportions[0].Name != "日期" {
		t.Errorf("Back 第一列应为 日期，实际 %s", spec.BackColProportions[0].Name)
	}
}

func TestDefaultMLSpec(t *testing.T) {
	spec := DefaultMLSpec()
	if spec.PaperWidthMM != 297 || spec.PaperHeightMM != 210 {
		t.Errorf("A4 size: want 297x210, got %gx%g", spec.PaperWidthMM, spec.PaperHeightMM)
	}
	if spec.DataRowsPerPage != 20 {
		t.Errorf("data rows: want 20, got %d", spec.DataRowsPerPage)
	}

	// Back proportions: 8+7+15+10+10+4+8+6+6+6+6 = 86
	var backSum float64
	for _, p := range spec.BackColProportions {
		backSum += p.Ratio
	}
	if backSum < 85.9 || backSum > 86.1 {
		t.Errorf("back col proportions sum: want ~86, got %g", backSum)
	}

	// Front proportions: 10x10 = 100
	var frontSum float64
	for _, p := range spec.FrontColProportions {
		frontSum += p.Ratio
	}
	if frontSum < 99.9 || frontSum > 100.1 {
		t.Errorf("front col proportions sum: want ~100, got %g", frontSum)
	}
}

func TestMLComputeLayout_Basic(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	if lay.FrontLeftMM != spec.LeftMarginMM {
		t.Errorf("front start: want %g, got %g", spec.LeftMarginMM, lay.FrontLeftMM)
	}

	wantWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	if lay.FrontWidthMM != wantWidth {
		t.Errorf("front width: want %g, got %g", wantWidth, lay.FrontWidthMM)
	}

	total := lay.FrontWidthMM + lay.PageGapWidthMM + lay.BackWidthMM + spec.LeftMarginMM + spec.RightMarginMM
	if total > spec.PaperWidthMM+0.01 || total < spec.PaperWidthMM-0.01 {
		t.Errorf("width sum: want %g, got %g", spec.PaperWidthMM, total)
	}

	// Back (left area, col 3) must come before Front (right area, col 15)
	if lay.BackStartCol >= lay.FrontStartCol {
		t.Errorf("back should start before front: back=%d, front=%d", lay.BackStartCol, lay.FrontStartCol)
	}
	if lay.FrontStartCol <= lay.BindingLeftCols {
		t.Errorf("front should start after binding cols")
	}
	if lay.TotalCols <= lay.FrontStartCol {
		t.Errorf("total should include front area")
	}
	if lay.BackStartCol <= lay.BindingLeftCols {
		t.Errorf("back should start after binding cols")
	}

	if len(lay.ExcelColumns) != len(spec.BackColProportions)+len(spec.FrontColProportions) {
		t.Errorf("excel columns: want %d, got %d", len(spec.BackColProportions)+len(spec.FrontColProportions), len(lay.ExcelColumns))
	}

	if lay.BackColCount != len(spec.BackColProportions) {
		t.Errorf("back col count: want %d, got %d", len(spec.BackColProportions), lay.BackColCount)
	}
	if lay.FrontColCount != len(spec.FrontColProportions) {
		t.Errorf("front col count: want %d, got %d", len(spec.FrontColProportions), lay.FrontColCount)
	}
}

func TestMLComputeLayout_ColWidthSum(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	// Back columns width sum = contentWidth * backRatioSum / 100
	var backRatioSum float64
	for _, p := range spec.BackColProportions {
		backRatioSum += p.Ratio
	}
	backExpected := lay.FrontWidthMM * backRatioSum / 100.0
	var backSum float64
	for _, c := range lay.BackColumns {
		backSum += c.WidthMM
	}
	if backSum < backExpected-0.01 || backSum > backExpected+0.01 {
		t.Errorf("back col widths: want %g, got %g", backExpected, backSum)
	}

	// Front columns width sum = contentWidth * frontRatioSum / 100
	var frontRatioSum float64
	for _, p := range spec.FrontColProportions {
		frontRatioSum += p.Ratio
	}
	frontExpected := lay.FrontWidthMM * frontRatioSum / 100.0
	var frontSum float64
	for _, c := range lay.FrontColumns {
		frontSum += c.WidthMM
	}
	if frontSum < frontExpected-0.01 || frontSum > frontExpected+0.01 {
		t.Errorf("front col widths: want %g, got %g", frontExpected, frontSum)
	}
}

func TestMLComputeLayout_Rows(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	if lay.HeaderRow < lay.TitleRow {
		t.Errorf("header row should be at or after title row")
	}
	if lay.DataStartRow < lay.HeaderRow {
		t.Errorf("data start should be at or after header row")
	}
}

func TestMLComputeLayout_TitleAccountCols(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	// Back title cols should be within back columns (left area, starting at BackStartCol)
	if lay.BackTitleColLeft < lay.BackStartCol {
		t.Errorf("back title left should be within back area: left=%d, backStart=%d", lay.BackTitleColLeft, lay.BackStartCol)
	}
	if lay.BackTitleColRight > lay.BackAccountColRight {
		t.Errorf("back title right should not exceed back area: right=%d, accountRight=%d", lay.BackTitleColRight, lay.BackAccountColRight)
	}
	if lay.BackAccountColLeft <= lay.BackTitleColRight {
		t.Errorf("back account col should start after title col: acctLeft=%d, titleRight=%d", lay.BackAccountColLeft, lay.BackTitleColRight)
	}
	if lay.BackAccountColRight > lay.BackStartCol+lay.BackColCount-1 {
		t.Errorf("back account right should stay within back column range: right=%d, max=%d", lay.BackAccountColRight, lay.BackStartCol+lay.BackColCount-1)
	}

	// Front title cols should be within front columns (right area, starting at FrontStartCol)
	if lay.FrontTitleColLeft < lay.FrontStartCol {
		t.Errorf("front title left should be within front area: left=%d, frontStart=%d", lay.FrontTitleColLeft, lay.FrontStartCol)
	}
	if lay.FrontAccountColRight > lay.FrontStartCol+lay.FrontColCount-1 {
		t.Errorf("front account right should stay within front column range: right=%d, max=%d", lay.FrontAccountColRight, lay.FrontStartCol+lay.FrontColCount-1)
	}
}

func TestMLComputeLayout_BackFrontColumns(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	if len(lay.BackColumns) == 0 {
		t.Fatal("BackColumns 不应为空")
	}
	if len(lay.FrontColumns) == 0 {
		t.Fatal("FrontColumns 不应为空")
	}

	// Back 前一列为 日期
	if lay.BackColumns[0].Name != "日期" {
		t.Errorf("BackColumns[0] 应为 日期，实际 %s", lay.BackColumns[0].Name)
	}

	// 验证 Back 列与 Front 列不重叠
	backEnd := lay.BackStartCol + len(lay.BackColumns) - 1
	if backEnd >= lay.FrontStartCol {
		t.Errorf("Back 列与 Front 列重叠：Back end=%d, Front start=%d", backEnd, lay.FrontStartCol)
	}

	// 验证间隙列位置
	wantPageGap := lay.BackStartCol + len(lay.BackColumns)
	if lay.PageGapStartCol != wantPageGap {
		t.Errorf("PageGap 位置错误：%d != %d", lay.PageGapStartCol, wantPageGap)
	}

	// 验证 Front 起始列在间隙列之后
	if lay.FrontStartCol != lay.PageGapStartCol+1 {
		t.Errorf("Front 起始列应在 PageGap 后：Front=%d, PageGap=%d", lay.FrontStartCol, lay.PageGapStartCol)
	}

	// 验证总列数
	wantTotal := lay.FrontStartCol + len(lay.FrontColumns) + lay.BindingRightCols
	if lay.TotalCols != wantTotal {
		t.Errorf("TotalCols 错误：%d != %d", lay.TotalCols, wantTotal)
	}
}

func TestMLComputeLayout_ColWidthSumProportions(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	var backSum, frontSum float64
	var backRatioSum, frontRatioSum float64
	for _, c := range lay.BackColumns {
		backSum += c.WidthMM
	}
	for _, p := range spec.BackColProportions {
		backRatioSum += p.Ratio
	}
	for _, c := range lay.FrontColumns {
		frontSum += c.WidthMM
	}
	for _, p := range spec.FrontColProportions {
		frontRatioSum += p.Ratio
	}

	contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	backExpected := contentWidth * backRatioSum / 100.0
	if backSum < backExpected-1 || backSum > backExpected+1 {
		t.Errorf("Back 列宽和 %.2f 应与 contentWidth*backRatioSum/100=%.2f 相近", backSum, backExpected)
	}
	frontExpected := contentWidth * frontRatioSum / 100.0
	if frontSum < frontExpected-1 || frontSum > frontExpected+1 {
		t.Errorf("Front 列宽和 %.2f 应与 contentWidth*frontRatioSum/100=%.2f 相近", frontSum, frontExpected)
	}
}
