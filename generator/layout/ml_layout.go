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
	BackColProportions  []MLColProportion
	FrontColProportions []MLColProportion
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
	BackColumns []MLColumnPos
	FrontColumns []MLColumnPos
	BindingLeftCols  int
	FrontStartCol    int
	PageGapStartCol  int
	BackStartCol     int
	BindingRightCols int
	TotalCols        int
	BackColCount     int
	FrontColCount    int
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
	BackTitleColLeft      int
	BackTitleColRight     int
	FrontTitleColLeft     int
	FrontTitleColRight    int
	BackAccountColLeft    int
	BackAccountColRight   int
	FrontAccountColLeft   int
	FrontAccountColRight  int
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

// DefaultMLSpec 返回多科目明细账的默认布局配置（A4 横向，双面打印）。
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
		BackColProportions: []MLColProportion{
			{Name: "日期", Ratio: 8},
			{Name: "凭证号", Ratio: 7},
			{Name: "摘要", Ratio: 15},
			{Name: "借方金额", Ratio: 10},
			{Name: "贷方金额", Ratio: 10},
			{Name: "方向", Ratio: 4},
			{Name: "余额", Ratio: 10},
			{Name: "明细1", Ratio: 9},
			{Name: "明细2", Ratio: 9},
			{Name: "明细3", Ratio: 9},
			{Name: "明细4", Ratio: 9},
		},
		FrontColProportions: []MLColProportion{
			{Name: "明细5", Ratio: 10},
			{Name: "明细6", Ratio: 10},
			{Name: "明细7", Ratio: 10},
			{Name: "明细8", Ratio: 10},
			{Name: "明细9", Ratio: 10},
			{Name: "明细10", Ratio: 10},
			{Name: "明细11", Ratio: 10},
			{Name: "明细12", Ratio: 10},
			{Name: "明细13", Ratio: 10},
			{Name: "明细14", Ratio: 10},
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

	// Back 列（左侧区域 = 纸背数据）
	var backCols []MLColumnPos
	var startMM float64
	for _, p := range spec.BackColProportions {
		w := contentWidth * p.Ratio / 100.0
		backCols = append(backCols, MLColumnPos{Name: p.Name, StartMM: startMM, WidthMM: w})
		startMM += w
	}

	// Front 列（右侧区域 = 纸正数据）
	var frontCols []MLColumnPos
	startMM = 0
	for _, p := range spec.FrontColProportions {
		w := contentWidth * p.Ratio / 100.0
		frontCols = append(frontCols, MLColumnPos{Name: p.Name, StartMM: startMM, WidthMM: w})
		startMM += w
	}

	backColCount := len(spec.BackColProportions)
	frontColCount := len(spec.FrontColProportions)
	backStart := bindingLeftCols + 1           // = 3（左侧 = Back 数据）
	pageGapStart := backStart + backColCount   // = 3 + 11 = 14
	frontStart := pageGapStart + 1             // = 15（右侧 = Front 数据）
	total := frontStart + frontColCount + bindingRightCols

	// Front 区 Excel 列映射
	var exc []MLExcelCol
	for i := range frontCols {
		exc = append(exc, MLExcelCol{Name: frontCols[i].Name, Col: frontStart + i})
	}

	// 向后兼容：Title/Account 按 Front 区拆分（与旧渲染器一致）
	titleCols := frontColCount / 2
	if titleCols < 1 {
		titleCols = 1
	}
	accountCols := frontColCount - titleCols

	return MLLayout{
		FrontLeftMM:       frontLeft,
		FrontWidthMM:      contentWidth,
		PageGapLeftMM:     pageGapLeft,
		PageGapWidthMM:    spec.PageGapMM,
		BackLeftMM:        backLeft,
		BackWidthMM:       contentWidth,
		BackColumns:       backCols,
		FrontColumns:      frontCols,
		BindingLeftCols:   bindingLeftCols,
		BackStartCol:      backStart,
		PageGapStartCol:   pageGapStart,
		FrontStartCol:     frontStart,
		BindingRightCols:  bindingRightCols,
		TotalCols:         total,
		BackColCount:      backColCount,
		FrontColCount:     frontColCount,
		ExcelColumns:      exc,
		TitleRow:          1,
		PageNumRow:        0,
		AccountRow:        2,
		HeaderRow:         4,
		DataStartRow:      5,
		// 向后兼容：旧 Title/Account 列（Front 区拆分）
		TitleColLeft:      frontStart,
		TitleColRight:     frontStart + titleCols - 1,
		AccountColLeft:    frontStart + titleCols,
		AccountColRight:   frontStart + frontColCount - 1,
		TitleColSpan:      titleCols,
		AccountColSpan:    accountCols,
		// 新 Back/Front 标题/科目列（各自占满整区宽度）
		BackTitleColLeft:      backStart,
		BackTitleColRight:     backStart + backColCount - 1,
		FrontTitleColLeft:     frontStart,
		FrontTitleColRight:    frontStart + frontColCount - 1,
		BackAccountColLeft:    backStart,
		BackAccountColRight:   backStart + backColCount - 1,
		FrontAccountColLeft:   frontStart,
		FrontAccountColRight:  frontStart + frontColCount - 1,
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
