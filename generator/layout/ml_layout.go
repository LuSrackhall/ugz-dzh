package layout

// MLSpec 定义多科目明细账页面的物理约束和内容结构。
// 不包含任何 Renderer 逻辑。
type MLSpec struct {
	PaperWidthMM      float64
	PaperHeightMM     float64
	LeftMarginMM      float64
	RightMarginMM     float64
	PageGapMM         float64
	TitleRowCount     int
	ColHeaderRowCount int
	DataRowsPerPage   int
	TopMarginRows     int
	BottomMarginRows  int

	// 两套独立列比例
	BackColProportions  []MLColProportion // 左半：7基础 + 明细1~4
	FrontColProportions []MLColProportion // 右半：明细5~14
}

// MLColProportion 定义多科目明细账一列在内容区中的宽度占比。
type MLColProportion struct {
	Name  string
	Ratio float64
}

// MLLayout 是 MLComputeLayout 的输出结果，包含所有坐标信息。
type MLLayout struct {
	FrontLeftMM, FrontWidthMM float64
	PageGapLeftMM, PageGapWidthMM float64
	BackLeftMM, BackWidthMM float64

	// 两套列坐标
	BackColumns  []MLColumnPos // 左半：基础7列 + 明细1~4
	FrontColumns []MLColumnPos // 右半：明细5~14

	BindingLeftCols  int // 2
	BackStartCol     int // 左半起始列（Back 区）
	PageGapStartCol  int // 间隙列
	FrontStartCol    int // 右半起始列（Front 区）
	BindingRightCols int // 2
	TotalCols        int

	BackColCount  int // Back 侧数据列数
	FrontColCount int // Front 侧数据列数

	ExcelColumns []MLExcelCol // 列名 → Excel 编号映射（仅用于兼容，非核心路径）

	TitleRow, PageNumRow, AccountRow, HeaderRow, DataStartRow int // 行号，两侧共享
	TopMarginRows, BottomMarginRows int

	// 合并单元格坐标（两侧独立）
	BackTitleColLeft, BackTitleColRight           int
	FrontTitleColLeft, FrontTitleColRight         int
	BackAccountColLeft, BackAccountColRight       int
	FrontAccountColLeft, FrontAccountColRight     int
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

// DefaultMLSpec 返回多科目明细账的默认布局配置（A4 横向，两套独立列比例）。
func DefaultMLSpec() MLSpec {
	return MLSpec{
		PaperWidthMM:      297,
		PaperHeightMM:     210,
		LeftMarginMM:      15,
		RightMarginMM:     15,
		PageGapMM:         8,
		TitleRowCount:     3,
		ColHeaderRowCount: 1,
		DataRowsPerPage:   20,
		TopMarginRows:     1,
		BottomMarginRows:  1,
		BackColProportions: []MLColProportion{
			{Name: "日期", Ratio: 4},
			{Name: "凭证号", Ratio: 4},
			{Name: "摘要", Ratio: 15},
			{Name: "借方金额", Ratio: 10},
			{Name: "贷方金额", Ratio: 10},
			{Name: "方向", Ratio: 4},
			{Name: "余额", Ratio: 8},
			{Name: "明细1", Ratio: 6},
			{Name: "明细2", Ratio: 6},
			{Name: "明细3", Ratio: 6},
			{Name: "明细4", Ratio: 6},
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
	bindingLeftCols := 1 // Back 书口（非装订，最左）
	bindingRightCols := 1 // Front 书口（非装订，最右）

	// Back 实际渲染 13 列（月日字号4 + 摘要/借/贷/方向/余额5 + 明细1~4），
	// 而 BackColProportions 仅 11 项（日期/凭证号 为复合列，各含 2 子列）。
	backColCount := 13
	frontColCount := len(spec.FrontColProportions)

	// Back 侧列坐标（左半内容：基础7列 + 明细1~4）
	var backCols []MLColumnPos
	var startMM float64
	for _, p := range spec.BackColProportions {
		w := contentWidth * p.Ratio / 100.0
		backCols = append(backCols, MLColumnPos{
			Name:    p.Name,
			StartMM: startMM,
			WidthMM: w,
		})
		startMM += w
	}

	// Front 侧列坐标（右半内容：明细5~14）
	startMM = 0
	var frontCols []MLColumnPos
	for _, p := range spec.FrontColProportions {
		w := contentWidth * p.Ratio / 100.0
		frontCols = append(frontCols, MLColumnPos{
			Name:    p.Name,
			StartMM: startMM,
			WidthMM: w,
		})
		startMM += w
	}

	// Excel 列号布局（对齐 GL 的正反逻辑，但 ML 装订在中间、书口在两侧）：
	//   Back（反面，左半）：书口在左、装订在右（col 15-16）
	//   Front（正面，右半）：装订在左（col 17-18）、书口在右
	// A:     Back 书口（1.2）           → col 1
	// B-N:   Back 数据 (13 cols)        → col 2-14
	// O-P:   Back 装订（7.75×2）        → col 15-16
	// Q-R:   Front 装订（8.35×2）       → col 17-18
	// S-AB:  Front 数据 (10 cols: 明细5~14) → col 19-28
	// AC:    Front 书口（0）            → col 29
	backStart := bindingLeftCols + 1          // = col 2 (左半 = Back 数据区)
	pageGapStart := backStart + backColCount  // = col 15 (中间装订区起始 = Back 装订)
	frontStart := pageGapStart + 4            // = col 19 (Back装订2列 + Front装订2列 之后)
	total := frontStart + frontColCount + bindingRightCols - 1 // = col 29 (Front 书口末列)

	// 合并列名映射（Back + Front 两段）
	var exc []MLExcelCol
	for i, c := range backCols {
		exc = append(exc, MLExcelCol{Name: c.Name, Col: backStart + i})
	}
	for i, c := range frontCols {
		exc = append(exc, MLExcelCol{Name: c.Name, Col: frontStart + i})
	}

	// Back 侧标题/科目合并列（左半）
	backTitleSplit := int(float64(backColCount) * 0.5)
	if backTitleSplit < 1 {
		backTitleSplit = 1
	}

	// Front 侧标题/科目合并列（右半）
	frontTitleSplit := int(float64(frontColCount) * 0.5)
	if frontTitleSplit < 1 {
		frontTitleSplit = 1
	}

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
		DataStartRow:      8, // 上边距1行 + 7行页头
		TopMarginRows:     spec.TopMarginRows,
		BottomMarginRows:  spec.BottomMarginRows,
		BackTitleColLeft:      backStart,
		BackTitleColRight:     backStart + backTitleSplit - 1,
		BackAccountColLeft:    backStart + backTitleSplit,
		BackAccountColRight:   backStart + backColCount - 1,
		FrontTitleColLeft:     frontStart,
		FrontTitleColRight:    frontStart + frontTitleSplit - 1,
		FrontAccountColLeft:   frontStart + frontTitleSplit,
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
