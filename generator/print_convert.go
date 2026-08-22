// Package generator — 打印版位格转换共享工具。
//
// 打印版与查看版唯一差异是金额栏位格化：金额列拆 12 小列（总宽守恒），
// 数字按人民币位拆分填入。本文件包含拆位算法、分组边框常量表、
// 插列/边框派生等 GL 与 ML 共用的工具函数。
package generator

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ConvertToPrint 读取查看版 xlsx，将账页位格化后另存为打印版。
// viewPath 为已落盘的查看版文件；printPath 通常位于 print/ 子目录，
// 文件名与查看版相同。查看版文件本身不会被修改。
func ConvertToPrint(viewPath, printPath string) error {
	f, err := excelize.OpenFile(viewPath)
	if err != nil {
		return fmt.Errorf("打开查看版 %s: %w", viewPath, err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		switch {
		case strings.HasPrefix(sheet, sheetPrefixML):
			err = convertMLSheet(f, sheet)
		case strings.HasPrefix(sheet, sheetPrefixGL):
			err = convertGLSheet(f, sheet)
		default:
			continue // 期初/期末表等不动
		}
		if err != nil {
			return fmt.Errorf("转换 %s: %w", sheet, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(printPath), 0o755); err != nil {
		return fmt.Errorf("创建打印目录: %w", err)
	}
	if err := f.SaveAs(printPath); err != nil {
		return fmt.Errorf("保存打印版 %s: %w", printPath, err)
	}
	return nil
}

// 12 小列从左到右的标签（十亿 → 分）。
var digitColLabels = [12]string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}

// 小格字体字号（74% 打印缩放下 11pt 放不进小格，实机校准后可调）。
const printDigitFontSize = 7.0

// 分组边框常量值（dividerStyles 表元素语义）。
const (
	divThinGreen  = 0 // 普通绿色细线
	divThickGreen = 1 // 绿色加粗（人民币分段组界）
	divThinRed    = 2 // 红色单细线（元|角）
)

// dividerStyles[i] = 第 i 小列与其右邻之间竖线的样式（i: 0..10）。
// 需求分组「十； 亿，千，百； 十，万，千； 百，十，元。 角，分」：
// 组界分隔符位于 十|亿(0)、百|十(3)、千|百(6)，元|角(8) 为红细线。
var dividerStyles = [11]int{
	divThickGreen, // 十|亿
	divThinGreen,
	divThinGreen,
	divThickGreen, // 百|十
	divThinGreen,
	divThinGreen,
	divThickGreen, // 千|百
	divThinGreen,
	divThinRed, // 元|角
	divThinGreen,
	divThinGreen,
}

// splitCNY 将分值拆为 12 个人民币位字符串（十亿…元角分）。
// 负数取绝对值填格（正负由方向列表达）；0 返回全空；
// 高位无效零留空，首个有效位起填数字。
func splitCNY(cents int64) [12]string {
	var out [12]string
	if cents == 0 {
		return out
	}
	v := cents
	if v < 0 {
		v = -v
	}
	started := false
	for i := 11; i >= 0; i-- {
		d := (v / int64(math.Pow10(i))) % 10
		if d != 0 {
			started = true
		}
		if started {
			out[11-i] = string(rune('0' + d))
		}
	}
	return out
}

// moneyColumn describes 一个待拆分的原始金额列（插列前坐标）。
type moneyColumn struct {
	col int // 原始 Excel 列号（1-based）
	w   float64
}

// splitMoneyColumns 从右往左对每个金额列插入 11 列并设置小列宽。
// 返回每个原列对应的 12 个小列起始列号（插列后坐标），顺序与输入一致。
func splitMoneyColumns(f *excelize.File, sheet string, cols []moneyColumn) ([][12]int, error) {
	// 记录原始列号：后续插列不影响尚未处理列的"原始"坐标语义
	orig := make([]int, len(cols))
	for i, c := range cols {
		orig[i] = c.col
	}
	splits := make([][12]int, len(cols))
	for i := len(cols) - 1; i >= 0; i-- {
		c := cols[i]
		w, err := f.GetColWidth(sheet, cellColLetter(c.col))
		if err != nil {
			return nil, err
		}
		if w <= 0 {
			w = c.w
		}
		if err := f.InsertCols(sheet, cellColLetter(c.col+1), 11); err != nil {
			return nil, err
		}
		unit := w / 12.0
		for k := 0; k < 12; k++ {
			f.SetColWidth(sheet, cellColLetter(c.col+k), cellColLetter(c.col+k), unit)
			splits[i][k] = orig[i] + k
		}
	}
	return splits, nil
}

// shiftColsByInserts 计算「原始」列号 col 在完成全部插列后的新列号。
// inserts 为各插入点的原始列号（在其右侧插 11 列）。
// col 等于插入点时不位移——该列本身是某金额列的第 1 小格。
// 比较始终基于原始列号，避免累加后的列号重复越过后续插入点。
func shiftColsByInserts(col int, inserts []int) int {
	orig := col
	for _, at := range inserts {
		if orig > at {
			col += 11
		}
	}
	return col
}

// borderSpec 单个小格的边框需求。
type borderSpec struct {
	leftColor  string
	leftStyle  int
	rightColor string
	rightStyle int
	top        *excelize.Border
	bottom     *excelize.Border
}

// dividerBorder 将分组样式常量转为边框定义（竖线归属左格的右边框）。
func dividerBorder(v int) (color string, style int) {
	switch v {
	case divThickGreen:
		return "#006100", 2
	case divThinRed:
		return "CC0000", 1
	default:
		return "#006100", 1
	}
}
