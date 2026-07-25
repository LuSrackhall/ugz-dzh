// Package layout 提供物理尺寸驱动的页面布局计算。
//
// 设计原则：
//   1. 以 mm 为单位定义物理约束，ComputeLayout 计算所有坐标
//   2. Renderer（Excel/其他）只消费 Layout，不知晓物理单位
//   3. 区域独立：列比例调整不影响其他区域
package layout

// LayoutSpec 定义页面的物理约束和内容结构。
// 不包含任何 Renderer 逻辑。
type LayoutSpec struct {
	// 纸张尺寸（mm）
	PaperWidthMM  float64 // A4 横向 = 297
	PaperHeightMM float64 // A4 横向 = 210

	// 模拟边距（mm）——由空白行列模拟
	LeftMarginMM  float64 // 装订边
	RightMarginMM float64 // 装订边
	PageGapMM     float64 // 正面与反面之间的间隙

	// 内容参数
	TitleRowCount     int     // 标题区行数（含空行）
	TitleSplitRatio   float64 // 标题区列分割：占正面区前 TitleSplitRatio（如 0.5），科目信息占剩余部分
	ColHeaderRowCount int     // 列标题行数
	DataRowsPerPage   int

	// 列比例（正面/反面内容区内的列宽分配）
	// 总和应为 100.0
	ColProportions []ColProportion
}

// ColProportion 定义一列在内容区中的宽度占比。
type ColProportion struct {
	Name  string
	Ratio float64 // 占内容区宽度的百分比（如 15.0 = 15%）
}

// Layout 是 ComputeLayout 的输出结果。
// 包含所有坐标信息，Renderer 只消费此结构。
type Layout struct {
	// ── mm 坐标 ──

	// 正面内容区（相对纸张左边缘）
	FrontLeftMM  float64
	FrontWidthMM float64

	// 页间隙（正面与反面之间）
	PageGapLeftMM  float64
	PageGapWidthMM float64

	// 反面内容区
	BackLeftMM  float64
	BackWidthMM float64

	// 列坐标（相对于 FrontLeftMM/BackLeftMM）
	Columns []ColumnPos

	// ── Excel 映射 ──

	BindingLeftCols  int // 左边距空列数
	FrontStartCol    int
	PageGapStartCol  int
	BackStartCol     int
	BindingRightCols int
	TotalCols        int

	// 列号（Excel）
	ExcelColumns []ExcelCol

	// 行号（0-indexed）
	TitleRow     int // 总分类账
	PageNumRow   int // 分第 n 页
	AccountRow   int // 科目名称（目前 TitleRow=0, PageNumRow=0, AccountRow=0 合并在同一行）
	HeaderRow    int // 列标题
	DataStartRow int // 数据第 1 行

	// 标题区列分割
	TitleColLeft     int // 标题"总分类账"起始列
	TitleColRight    int // 标题"总分类账"结束列
	AccountColLeft   int // 科目信息起始列
	AccountColRight  int // 科目信息结束列
	TitleColSpan     int // 标题区总列数
	AccountColSpan   int // 科目信息区总列数
}

// ColumnPos 列在一侧内容区中的位置（mm）
type ColumnPos struct {
	Name    string
	StartMM float64 // 相对内容区起点的偏移
	WidthMM float64
}

// ExcelCol Excel 列号
type ExcelCol struct {
	Name string
	Col  int // 1-indexed
}

// DefaultGLSpec 返回总分类账的默认布局配置（A4 横向）
func DefaultGLSpec() LayoutSpec {
	return LayoutSpec{
		PaperWidthMM:      297,
		PaperHeightMM:     210,
		LeftMarginMM:      15,  // 装订边（空列模拟）
		RightMarginMM:     15,
		PageGapMM:         8,   // 正反面间隙
		TitleRowCount:     3,   // 分第n页、总分类账、科目名称（纵向三行）
		TitleSplitRatio:   0.5, // 标题和科目信息各占一半
		ColHeaderRowCount: 1,
		DataRowsPerPage:   20,
		ColProportions: []ColProportion{
			{Name: "日期", Ratio: 10},
			{Name: "凭证号", Ratio: 9},
			{Name: "摘要", Ratio: 25},
			{Name: "借方金额", Ratio: 16},
			{Name: "贷方金额", Ratio: 16},
			{Name: "方向", Ratio: 5},
			{Name: "余额", Ratio: 14},
			{Name: "金额分栏", Ratio: 5},
		},
	}
}
// ComputeLayout 从 Spec 计算所有坐标。纯函数。
func ComputeLayout(spec LayoutSpec) Layout {
	contentWidth := (spec.PaperWidthMM - spec.LeftMarginMM - spec.RightMarginMM - spec.PageGapMM) / 2
	frontLeft := spec.LeftMarginMM
	pageGapLeft := frontLeft + contentWidth
	backLeft := pageGapLeft + spec.PageGapMM
	bindingLeftCols := 2
	bindingRightCols := 2

	// 列坐标（mm）
	var cols []ColumnPos
	var startMM float64
	for _, p := range spec.ColProportions {
		w := contentWidth * p.Ratio / 100.0
		cols = append(cols, ColumnPos{
			Name:    p.Name,
			StartMM: startMM,
			WidthMM: w,
		})
		startMM += w
	}

	// Excel 列号映射
	nCol := len(spec.ColProportions)
	frontStart := bindingLeftCols + 1
	pageGapStart := frontStart + nCol
	backStart := pageGapStart + 1
	total := backStart + nCol + bindingRightCols

	var exc []ExcelCol
	for i := range cols {
		exc = append(exc, ExcelCol{Name: cols[i].Name, Col: frontStart + i})
	}

	// 标题区列分割
	titleCols := int(float64(nCol) * spec.TitleSplitRatio)
	if titleCols < 1 {
		titleCols = 1
	}
	accountCols := nCol - titleCols

	return Layout{
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

// MMToExcelColWidth 将 mm 宽度近似转换为 Excel 列宽单位。
// Excel 列宽 1 单位 ≈ 7 像素 (96 DPI)，1mm ≈ 3.78 像素。
// 这是近似值，后续可以根据渲染效果微调。
func MMToExcelColWidth(mm float64) float64 {
	const pxPerMM = 96.0 / 25.4
	const pxPerColUnit = 7.0
	return mm * pxPerMM / pxPerColUnit
}

// MMToExcelRowHeight 将 mm 高度转换为 Excel 行高（磅）。
// 1 磅 = 1/72 英寸，1mm ≈ 2.835 磅。
func MMToExcelRowHeight(mm float64) float64 {
	return mm * 72.0 / 25.4
}
