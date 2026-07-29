package layout

import "testing"

func TestDefaultGLSpec(t *testing.T) {
	spec := DefaultGLSpec()
	if spec.PaperWidthMM != 297 || spec.PaperHeightMM != 210 {
		t.Errorf("A4 size: want 297x210, got %gx%g", spec.PaperWidthMM, spec.PaperHeightMM)
	}
	if spec.DataRowsPerPage != 20 {
		t.Errorf("data rows: want 20, got %d", spec.DataRowsPerPage)
	}

	var sum float64
	for _, p := range spec.ColProportions {
		sum += p.Ratio
	}
	if sum < 99.9 || sum > 100.1 {
		t.Errorf("col proportions sum: want ~100, got %g", sum)
	}
}

func TestGLComputeLayout_Basic(t *testing.T) {
	spec := DefaultGLSpec()
	lay := GLComputeLayout(spec)

	if lay.FrontLeftMM != spec.LeftMarginMM {
		t.Errorf("front start: want %g, got %g", spec.LeftMarginMM, lay.FrontLeftMM)
	}

	wantWidth := spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM
	if lay.FrontWidthMM != wantWidth {
		t.Errorf("front width: want %g, got %g", wantWidth, lay.FrontWidthMM)
	}
	if lay.FrontWidthMM <= 0 {
		t.Errorf("front width should be positive")
	}
	if lay.BackWidthMM <= 0 {
		t.Errorf("back width should be positive")
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

	if len(lay.ExcelColumns) != len(spec.ColProportions) {
		t.Errorf("excel columns: want %d, got %d", len(spec.ColProportions), len(lay.ExcelColumns))
	}
}

func TestGLComputeLayout_ColWidthSum(t *testing.T) {
	spec := DefaultGLSpec()
	lay := GLComputeLayout(spec)

	var sum float64
	for _, c := range lay.Columns {
		sum += c.WidthMM
	}
	if sum < lay.FrontWidthMM-0.01 || sum > lay.FrontWidthMM+0.01 {
		t.Errorf("col widths: want %g, got %g", lay.FrontWidthMM, sum)
	}
}

func TestGLComputeLayout_Rows(t *testing.T) {
	spec := DefaultGLSpec()
	lay := GLComputeLayout(spec)

	if lay.HeaderRow < lay.TitleRow {
		t.Errorf("header row should be at or after title row")
	}
	if lay.DataStartRow < lay.HeaderRow {
		t.Errorf("data start should be at or after header row")
	}
}
