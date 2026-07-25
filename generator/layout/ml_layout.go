package layout

// MLSpec 定义多科目明细账页面的物理约束和内容结构。
// 不包含任何 Renderer 逻辑。
type MLSpec struct {
	PaperWidthMM  float64
	PaperHeightMM float64
	LeftMarginMM  float64
	RightMarginMM float64
	PageGapMM     float64
	TitleRowCount     int
	TitleSplitRatio   float64
	ColHeaderRowCount int
	DataRowsPerPage   int
	ColProportions []MLColProportion
}

// MLColProportion 定义多科目明细账一列在内容区中的宽度占比。
type MLColProportion struct {
	Name  string
	Ratio float64
}

// MLLayout 是 MLComputeLayout 的输出结果，包含所有坐标信息。
type MLLayout struct {
	FrontLeftMM  float64
	FrontWidthMM float64
	PageGapLeftMM  float64
	PageGapWidthMM float64
	BackLeftMM  float64
	BackWidthMM float64
	Columns []MLColumnPos
	BindingLeftCols  int
	FrontStartCol    int
	PageGapStartCol  int
	BackStartCol     int
	BindingRightCols int
	TotalCols        int
	ExcelColumns []MLExcelCol
	TitleRow     int
	PageNumRow   int
	AccountRow   int
	HeaderRow    int
	DataStartRow int
	TitleColLeft     int
	TitleColRight    int
	AccountColLeft   int
	AccountColRight  int
	TitleColSpan     int
	AccountColSpan   int
}

// MLColumnPos 列在一侧内容区中的位置（mm）
type MLColumnPos struct {
	Name    string
	StartMM float64
	WidthMM float64
}

// MLExcelCol Excel 列号
type MLExcelCol struct {
	Name string
	Col  int
}

// DefaultMLSpec 返回多科目明细账的默认布局配置（A4 横向，12 列/区）。
func DefaultMLSpec() MLSpec {
	return MLSpec{
		PaperWidthMM:      297,
		PaperHeightMM:     210,
		LeftMarginMM:      15,
		RightMarginMM:     15,
		PageGapMM:         8,
		TitleRowCount:     3,
		TitleSplitRatio:   0.5,
		ColHeaderRowCount: 1,
		DataRowsPerPage:   20,
		ColProportions: []MLColProportion{
			{Name: "日期", Ratio: 8},
			{Name: "凭证号", Ratio: 7},
			{Name: "摘要", Ratio: 15},
			{Name: "借方金额", Ratio: 10},
			{Name: "贷方金额", Ratio: 10},
			{Name: "方向", Ratio: 4},
			{Name: "余额", Ratio: 8},
			{Name: "明细1~4", Ratio: 8},
			{Name: "明细5~6", Ratio: 8},
			{Name: "明细7~8", Ratio: 8},
			{Name: "明细9~10", Ratio: 8},
			{Name: "金额分栏", Ratio: 6},
		},
	}
}

// MLComputeLayout 从 MLSpec 计算所有坐标。纯函数。
func MLComputeLayout(spec MLSpec) MLLayout {
	contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	frontLeft := spec.LeftMarginMM
	pageGapLeft := frontLeft + contentWidth
	backLeft := pageGapLeft + spec.PageGapMM
	bindingLeftCols := 2
	bindingRightCols := 2

	var cols []MLColumnPos
	var startMM float64
	for _, p := range spec.ColProportions {
		w := contentWidth * p.Ratio / 100.0
		cols = append(cols, MLColumnPos{
			Name:    p.Name,
			StartMM: startMM,
			WidthMM: w,
		})
		startMM += w
	}

	nCol := len(spec.ColProportions)
	frontStart := bindingLeftCols + 1
	pageGapStart := frontStart + nCol
	backStart := pageGapStart + 1
	total := backStart + nCol + bindingRightCols

	var exc []MLExcelCol
	for i := range cols {
		exc = append(exc, MLExcelCol{Name: cols[i].Name, Col: frontStart + i})
	}

	titleCols := int(float64(nCol) * spec.TitleSplitRatio)
	if titleCols < 1 {
		titleCols = 1
	}
	accountCols := nCol - titleCols

	return MLLayout{
		FrontLeftMM:       frontLeft,
		FrontWidthMM:      contentWidth,
		PageGapLeftMM:     pageGapLeft,
		PageGapWidthMM:    spec.PageGapMM,
		BackLeftMM:        backLeft,
		BackWidthMM:       contentWidth,
		Columns:           cols,
		BindingLeftCols:   bindingLeftCols,
		FrontStartCol:     frontStart,
		PageGapStartCol:   pageGapStart,
		BackStartCol:      backStart,
		BindingRightCols:  bindingRightCols,
		TotalCols:         total,
		ExcelColumns:      exc,
		TitleRow:          1,
		PageNumRow:        0,
		AccountRow:        2,
		HeaderRow:         4,
		DataStartRow:      5,
		TitleColLeft:      frontStart,
		TitleColRight:     frontStart + titleCols - 1,
		AccountColLeft:    frontStart + titleCols,
		AccountColRight:   frontStart + nCol - 1,
		TitleColSpan:      titleCols,
		AccountColSpan:    accountCols,
	}
}

// MLMMToExcelColWidth 将 mm 宽度近似转换为 Excel 列宽单位。
func MLMMToExcelColWidth(mm float64) float64 {
	const pxPerMM = 96.0 / 25.4
	const pxPerColUnit = 7.0
	return mm * pxPerMM / pxPerColUnit
}

// MLMMToExcelRowHeight 将 mm 高度转换为 Excel 行高（磅）。
func MLMMToExcelRowHeight(mm float64) float64 {
	return mm * 72.0 / 25.4
}
