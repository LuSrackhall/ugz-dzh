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

	var backSum float64
	for _, p := range spec.BackColProportions {
		backSum += p.Ratio
	}
	if backSum < 99.9 || backSum > 100.1 {
		t.Errorf("back col proportions sum: want ~100, got %g", backSum)
	}

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
	if lay.BackStartCol >= lay.FrontStartCol {
		t.Errorf("back should start before front (BackStartCol=%d, FrontStartCol=%d)", lay.BackStartCol, lay.FrontStartCol)
	}
	if lay.TotalCols <= lay.FrontStartCol {
		t.Errorf("total should include front area")
	}

	if len(lay.ExcelColumns) != len(spec.FrontColProportions) {
		t.Errorf("excel columns: want %d, got %d", len(spec.FrontColProportions), len(lay.ExcelColumns))
	}
}

func TestMLComputeLayout_ColWidthSum(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	var backSum, frontSum float64
	for _, c := range lay.BackColumns {
		backSum += c.WidthMM
	}
	for _, c := range lay.FrontColumns {
		frontSum += c.WidthMM
	}

	contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	if backSum < contentWidth-1 || backSum > contentWidth+1 {
		t.Errorf("Back 列宽和 %.2f 应与 contentWidth %.2f 相近", backSum, contentWidth)
	}
	if frontSum < contentWidth-1 || frontSum > contentWidth+1 {
		t.Errorf("Front 列宽和 %.2f 应与 contentWidth %.2f 相近", frontSum, contentWidth)
	}
}

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

func TestMLComputeLayout_BackFrontColumns(t *testing.T) {
	spec := DefaultMLSpec()
	lay := MLComputeLayout(spec)

	if len(lay.BackColumns) == 0 {
		t.Fatal("BackColumns 不应为空")
	}
	if len(lay.FrontColumns) == 0 {
		t.Fatal("FrontColumns 不应为空")
	}

	if lay.BackColumns[0].Name != "日期" {
		t.Errorf("BackColumns[0] 应为 日期，实际 %s", lay.BackColumns[0].Name)
	}

	// Back columns should be left (BackStartCol=3), Front should be right (FrontStartCol=15)
	if lay.BackStartCol >= lay.FrontStartCol {
		t.Errorf("BackStartCol(%d) should be < FrontStartCol(%d)", lay.BackStartCol, lay.FrontStartCol)
	}

	// Back area: start=3, end=3+11-1=13. Front area: start=15
	backEnd := lay.BackStartCol + len(lay.BackColumns) - 1
	if backEnd >= lay.FrontStartCol {
		t.Errorf("Back 列与 Front 列重叠：Back end=%d, Front start=%d", backEnd, lay.FrontStartCol)
	}

	// Page gap should be between Back and Front
	if lay.PageGapStartCol != lay.BackStartCol+len(lay.BackColumns) {
		t.Errorf("PageGap 位置错误：%d != %d", lay.PageGapStartCol, lay.BackStartCol+len(lay.BackColumns))
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
