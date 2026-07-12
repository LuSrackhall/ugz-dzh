package layout

import (
	"testing"
)

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

func TestComputeLayout_Basic(t *testing.T) {
	spec := DefaultGLSpec()
	lay := ComputeLayout(spec)

	// 正面区起始 = 左边距
	if lay.FrontLeftMM != spec.LeftMarginMM {
		t.Errorf("front start: want %g, got %g", spec.LeftMarginMM, lay.FrontLeftMM)
	}

	// 正面区宽度 = (纸宽 - 边距*2 - 页间隙)/2
	wantWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	if lay.FrontWidthMM != wantWidth {
		t.Errorf("front width: want %g, got %g", wantWidth, lay.FrontWidthMM)
	}

	// 正面区 + 页间隙 + 反面区 = 纸宽
	total := lay.FrontWidthMM + lay.PageGapWidthMM + lay.BackWidthMM + spec.LeftMarginMM + spec.RightMarginMM
	if total > spec.PaperWidthMM+0.01 || total < spec.PaperWidthMM-0.01 {
		t.Errorf("width sum: want %g, got %g", spec.PaperWidthMM, total)
	}

	// Excel 列分配
	if lay.FrontStartCol <= lay.BindingLeftCols {
		t.Errorf("front should start after binding cols")
	}
	if lay.BackStartCol <= lay.FrontStartCol {
		t.Errorf("back should start after front")
	}
	if lay.TotalCols <= lay.BackStartCol {
		t.Errorf("total should include back area")
	}

	// 正面和反面列数一致
	if len(lay.ExcelColumns) != len(spec.ColProportions) {
		t.Errorf("excel columns: want %d, got %d", len(spec.ColProportions), len(lay.ExcelColumns))
	}
}

func TestComputeLayout_ColWidthSum(t *testing.T) {
	spec := DefaultGLSpec()
	lay := ComputeLayout(spec)

	var sum float64
	for _, c := range lay.Columns {
		sum += c.WidthMM
	}
	if sum < lay.FrontWidthMM-0.01 || sum > lay.FrontWidthMM+0.01 {
		t.Errorf("col widths: want %g, got %g", lay.FrontWidthMM, sum)
	}
}

func TestComputeLayout_Rows(t *testing.T) {
	spec := DefaultGLSpec()
	lay := ComputeLayout(spec)

	if lay.TitleRow <= lay.PageNumRow {
		t.Errorf("title row should be after page num row")
	}
	if lay.AccountRow <= lay.TitleRow {
		t.Errorf("account row should be after title row")
	}
	if lay.HeaderRow <= lay.AccountRow {
		t.Errorf("header row should be after account row")
	}
	if lay.DataStartRow <= lay.HeaderRow {
		t.Errorf("data start should be after header row")
	}
}
