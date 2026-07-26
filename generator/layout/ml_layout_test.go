package layout

import "testing"

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

	if lay.FrontStartCol <= lay.BindingLeftCols {
		t.Errorf("front should start after binding cols")
	}
	if lay.BackStartCol <= lay.FrontStartCol {
		t.Errorf("back should start after front")
	}
	if lay.TotalCols <= lay.BackStartCol {
		t.Errorf("total should include back area")
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

	// Back title cols should be within back columns
	if lay.BackTitleColLeft < lay.FrontStartCol {
		t.Errorf("back title left should be within back area")
	}
	if lay.BackTitleColRight > lay.BackAccountColRight {
		t.Errorf("back title right should not exceed back area")
	}
	if lay.BackAccountColLeft <= lay.BackTitleColRight {
		t.Errorf("back account col should start after title col")
	}
	if lay.BackAccountColRight > lay.FrontStartCol+lay.BackColCount-1 {
		t.Errorf("back account right should stay within back column range")
	}

	// Front title cols should be within front columns
	if lay.FrontTitleColLeft < lay.BackStartCol {
		t.Errorf("front title left should be within front area")
	}
	if lay.FrontAccountColRight > lay.BackStartCol+lay.FrontColCount-1 {
		t.Errorf("front account right should stay within front column range")
	}
}
