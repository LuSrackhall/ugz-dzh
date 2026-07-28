// Package layout 提供物理尺寸驱动的页面布局计算。
//
// 设计原则：
//   1. 以 mm 为单位定义物理约束，ComputeLayout 计算所有坐标
//   2. Renderer（Excel/其他）只消费 Layout，不知晓物理单位
//   3. 区域独立：列比例调整不影响其他区域
package layout

// GLSpec 定义总分类账页面的物理约束和内容结构。
// 不包含任何 Renderer 逻辑。
type GLSpec struct {
	PaperWidthMM  float64
	PaperHeightMM float64
	LeftMarginMM  float64
	RightMarginMM float64
	PageGapMM     float64
	TitleRowCount     int
	TitleSplitRatio   float64
	ColHeaderRowCount int
	DataRowsPerPage   int
	ColProportions []GLColProportion
}

// GLColProportion 定义总分类账一列在内容区中的宽度占比。
type GLColProportion struct {
	Name  string
	Ratio float64
}

// GLLayout 是 GLComputeLayout 的输出结果，包含所有坐标信息。
type GLLayout struct {
	FrontLeftMM  float64
	FrontWidthMM float64
	PageGapLeftMM  float64
	PageGapWidthMM float64
	BackLeftMM  float64
	BackWidthMM float64
	Columns []GLColumnPos
	BindingLeftCols  int
	FrontStartCol    int
	PageGapStartCol  int
	BackStartCol     int
	BindingRightCols int
	TotalCols        int
	ExcelColumns []GLExcelCol
	TitleRow     int
	PageNumRow   int
	HeaderRow    int
	SubHeaderRow int
	DataStartRow int
	TitleColLeft     int
	TitleColRight    int
	AccountColLeft   int
	AccountColRight  int
	TitleColSpan     int
	AccountColSpan   int
}

// GLColumnPos 列在一侧内容区中的位置（mm）
type GLColumnPos struct {
	Name    string
	StartMM float64
	WidthMM float64
}

// GLExcelCol Excel 列号
type GLExcelCol struct {
	Name string
	Col  int
}

// DefaultGLSpec 返回总分类账的默认布局配置（A4 横向）
func DefaultGLSpec() GLSpec {
	return GLSpec{
		PaperWidthMM:      297,
		PaperHeightMM:     210,
		LeftMarginMM:      15,
		RightMarginMM:     15,
		PageGapMM:         8,
		TitleRowCount:     3,
		TitleSplitRatio:   0.5,
		ColHeaderRowCount: 2,
		DataRowsPerPage:   20,
		ColProportions: []GLColProportion{
			{Name: "月", Ratio: 2},
			{Name: "日", Ratio: 2},
			{Name: "字", Ratio: 2},
			{Name: "号", Ratio: 2},
			{Name: "摘要", Ratio: 32},
			{Name: "借方金额", Ratio: 18},
			{Name: "✓", Ratio: 2},
			{Name: "贷方金额", Ratio: 18},
			{Name: "✓", Ratio: 2},
			{Name: "借或贷", Ratio: 2},
			{Name: "余额", Ratio: 16},
			{Name: "✓", Ratio: 2},
		},
	}
}

// GLComputeLayout 从 GLSpec 计算所有坐标。纯函数。
func GLComputeLayout(spec GLSpec) GLLayout {
	contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	frontLeft := spec.LeftMarginMM
	pageGapLeft := frontLeft + contentWidth
	backLeft := pageGapLeft + spec.PageGapMM
	bindingLeftCols := 2
	bindingRightCols := 2

	var cols []GLColumnPos
	var startMM float64
	for _, p := range spec.ColProportions {
		w := contentWidth * p.Ratio / 100.0
		cols = append(cols, GLColumnPos{
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

	var exc []GLExcelCol
	for i := range cols {
		exc = append(exc, GLExcelCol{Name: cols[i].Name, Col: frontStart + i})
	}

	titleCols := int(float64(nCol) * spec.TitleSplitRatio)
	if titleCols < 1 {
		titleCols = 1
	}
	accountCols := nCol - titleCols

	return GLLayout{
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
		TitleRow:          0,
		PageNumRow:        1,
		HeaderRow:         3,
			SubHeaderRow:      4,
		DataStartRow:      5,
		TitleColLeft:      frontStart,
		TitleColRight:     frontStart + titleCols - 1,
		AccountColLeft:    frontStart + titleCols,
		AccountColRight:   frontStart + nCol - 1,
		TitleColSpan:      titleCols,
		AccountColSpan:    accountCols,
	}
}

// GLMMToExcelColWidth 将 mm 宽度近似转换为 Excel 列宽单位。
func GLMMToExcelColWidth(mm float64) float64 {
	const pxPerMM = 96.0 / 25.4
	const pxPerColUnit = 3.5
	return mm * pxPerMM / pxPerColUnit
}

// GLMMToExcelRowHeight 将 mm 高度转换为 Excel 行高（磅）。
func GLMMToExcelRowHeight(mm float64) float64 {
	return mm * 72.0 / 25.4
}
