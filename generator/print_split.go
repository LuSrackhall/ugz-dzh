package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ledger/generator/layout"

	"github.com/xuri/excelize/v2"
)

// 打印版（金额分栏）导出。
//
// 与查看版共用全部生成与计算逻辑；本文件独立承载打印版专属样式：
// 每个金额格均分为 12 个小列，数字按人民币位拆分填入
// （十 亿 千 百 | 十 万 千 | 百 十 元 | 角 | 分）。
//
// 分组边框语义（表头从左到右：十 亿 千 百 十 万 千 百 十 元 角 分）：
//   - 组界加粗：「十|亿」「百|十万千」「千|百十元」共 3 处（;）
//   - 红色单细线：「元|角」共 1 处（.）
//   - 其余为普通细线

const printSplitCols = 12 // 每个金额格拆分的小列数

// printSplitHeaders 12 小列表头文字。
var printSplitHeaders = []string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}

// printBoldBefore 第 i 个小列左侧是加粗边框（组界 ;），i 为小列序号 0-based。
// 组界位于 [十|亿]=idx1、[百|十万千]=idx4、[千|百十元]=idx7。
var printBoldBefore = map[int]bool{1: true, 4: true, 7: true}

// printRedBefore 「元|角」之间红色单细线（.），即 idx9 左侧。
var printRedBefore = map[int]bool{9: true}

// ExportPrintVersion 将当前工作薄复制到 {OutputDir}/print/ 下，
// 对副本做金额分栏改造后保存。查看版文件不受影响。
func (wb *Workbook) ExportPrintVersion() error {
	printDir := filepath.Join(wb.OutputDir, "print")
	if err := os.MkdirAll(printDir, 0o755); err != nil {
		return fmt.Errorf("创建打印版目录: %w", err)
	}

	src := wb.currentPath()
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("读取查看版文件: %w", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("打开打印版底稿: %w", err)
	}

	for _, sheet := range f.GetSheetList() {
		if strings.HasPrefix(sheet, sheetPrefixGL) {
			if err := wb.splitGLSheet(f, sheet); err != nil {
				return fmt.Errorf("打印版 %s: %w", sheet, err)
			}
		} else if strings.HasPrefix(sheet, sheetPrefixML) {
			if err := wb.splitMLSheet(f, sheet); err != nil {
				return fmt.Errorf("打印版 %s: %w", sheet, err)
			}
		}
	}

	out := filepath.Join(printDir, wb.Month+".xlsx")
	if err := f.SaveAs(out); err != nil {
		return fmt.Errorf("保存打印版: %w", err)
	}
	return f.Close()
}

// ── GL ──

// splitGLSheet 对 GL（含合并 GL）sheet 拆分借/贷/余额三列。
func (wb *Workbook) splitGLSheet(f *excelize.File, sheet string) error {
	lay := glLayout()
	rows, _ := f.GetRows(sheet)
	lastRow := len(rows)

	// 表头行集合：首页 + 各过次页后的表头（大标题行 / 小列表头行）
	titleRows := map[int]bool{}
	subRows := map[int]bool{}
	collectGLHeaderRows(lay, rows, titleRows, subRows)

	// 金额列（改造前列号）：正反面各 借/贷/余
	var moneyCols []int
	for _, off := range []int{glColDebit, glColCredit, glColBalance} {
		moneyCols = append(moneyCols, lay.FrontStartCol+off, lay.BackStartCol+off)
	}

	return splitMoneyColumns(f, sheet, moneyCols, lastRow, titleRows, subRows)
}

// collectGLHeaderRows 收集 GL 的表头行：首页固定 + 每个过次页后的新表头。
func collectGLHeaderRows(lay layout.GLLayout, rows [][]string, titleRows, subRows map[int]bool) {
	titleRows[lay.HeaderRow+1] = true
	subRows[lay.SubHeaderRow+1] = true
	for i, r := range rows {
		row := i + 1
		if hasPageBreakAt(r, lay) {
			// 过次页行 row → 下页上边距 row+1+BottomMargin → 标题=上边距+1，小表头=标题+2
			topMargin := row + 1 + lay.BottomMarginRows
			titleRows[topMargin+1] = true
			subRows[topMargin+2] = true
		}
	}
}

// ── ML ──

// splitMLSheet 对 ML sheet 拆分借/贷/余额 + 明细1-14 列。
func (wb *Workbook) splitMLSheet(f *excelize.File, sheet string) error {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	lastRow := len(rows)

	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows

	// ML 每页 block：h1=start+1（借/贷/余大标题），h4=start+4（小列表头）
	titleRows := map[int]bool{}
	subRows := map[int]bool{}
	for start := 1; start <= lastRow; start += blockRows {
		titleRows[start+1] = true
		subRows[start+4] = true
	}

	// 金额列：Back 借/贷/余 + 明细1-14
	var moneyCols []int
	for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
		moneyCols = append(moneyCols, lay.BackStartCol+off)
	}
	for i := 0; i < mlMaxDetails; i++ {
		moneyCols = append(moneyCols, mlDetailCol(lay, i))
	}

	return splitMoneyColumns(f, sheet, moneyCols, lastRow, titleRows, subRows)
}

// ── 核心拆分 ──

// splitMoneyColumns 将 moneyCols 中每个金额列拆为 12 小列。
// 必须从最右列开始处理：插入列会平移右侧列号，从右往左可保证
// 未处理列（左侧）的列号始终有效。
// 表头在全部列拆分完成后统一写入（避免被后续插入平移）。
func splitMoneyColumns(
	f *excelize.File,
	sheet string,
	moneyCols []int,
	lastRow int,
	titleRows, subRows map[int]bool,
) error {
	// 从右往左排序
	cols := make([]int, len(moneyCols))
	copy(cols, moneyCols)
	for i := 0; i < len(cols); i++ {
		for j := i + 1; j < len(cols); j++ {
			if cols[j] > cols[i] {
				cols[i], cols[j] = cols[j], cols[i]
			}
		}
	}

	// 记录每个金额格的原始宽度（12 小列均分用）
	widths := make(map[int]float64, len(cols))
	for _, col := range cols {
		w, err := f.GetColWidth(sheet, cellColLetter(col))
		if err != nil {
			return err
		}
		widths[col] = float64(w)
	}

	// 从右往左逐列拆分（仅插列+均分宽+数据填位）
	for _, col := range cols {
		if err := splitOneMoneyColumn(f, sheet, col, lastRow, widths[col]); err != nil {
			return err
		}
	}

	// 全部拆分完成后统一写表头。
	// 改造后第 k 个金额列（按原列号升序）的首列 = 原首列 + k*11。
	sorted := make([]int, len(cols))
	copy(sorted, cols)
	for i := 0; i < len(sorted); i++ { // 升序
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for k, origCol := range sorted {
		newCol := origCol + k*(printSplitCols-1)
		for sr := range subRows {
			writePrintSubHeader(f, sheet, newCol, sr)
		}
		for tr := range titleRows {
			retitlePrintMoneyHeader(f, sheet, newCol, tr)
		}
	}
	return nil
}

// splitOneMoneyColumn 拆分单个金额列（仅插列+均分宽+数据填位，表头由调用方统一写）：
//  1. 在 col+1 前插入 11 列
//  2. 12 小列均分原宽
//  3. 数据行金额拆位填入 12 小列
func splitOneMoneyColumn(
	f *excelize.File,
	sheet string,
	col int,
	lastRow int,
	origWidth float64,
) error {
	// 读全部数据值（改造前行号在插入后不变——只动列）
	type cellVal struct {
		row int
		val string
	}
	var vals []cellVal
	for row := 1; row <= lastRow; row++ {
		v, _ := f.GetCellValue(sheet, cellName(col, row))
		if v != "" {
			vals = append(vals, cellVal{row, v})
		}
	}

	// 插入 11 列
	if err := f.InsertCols(sheet, cellColLetter(col+1), printSplitCols-1); err != nil {
		return fmt.Errorf("插入列: %w", err)
	}

	// 12 小列均分原宽
	unit := origWidth / float64(printSplitCols)
	for i := 0; i < printSplitCols; i++ {
		f.SetColWidth(sheet, cellColLetter(col+i), cellColLetter(col+i), unit)
	}

	// 数据行金额拆位填入
	for _, cv := range vals {
		digits := splitAmountToDigits(cv.val)
		if digits == nil {
			continue // 无法解析（如文字），保持原样不动
		}
		// 清原格（值移到小列后原格不再显示完整数）
		f.SetCellValue(sheet, cellName(col, cv.row), "")
		for i, d := range digits {
			if d != "" {
				f.SetCellValue(sheet, cellName(col+i, cv.row), d)
			}
		}
	}
	return nil
}

// writePrintSubHeader 在小列表头行的 [col, col+11] 写入 十亿千百… 与分组边框。
func writePrintSubHeader(f *excelize.File, sheet string, col, row int) {
	thinGreen := "006100"
	baseStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 6, Color: thinGreen},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 6, Color: thinGreen},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "left", Color: thinGreen, Style: 2}},
	})
	redStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 6, Color: thinGreen},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "left", Color: "CC0000", Style: 1}},
	})

	for i, h := range printSplitHeaders {
		cell := cellName(col+i, row)
		f.SetCellValue(sheet, cell, h)
		switch {
		case printBoldBefore[i]:
			f.SetCellStyle(sheet, cell, cell, boldStyle)
		case printRedBefore[i]:
			f.SetCellStyle(sheet, cell, cell, redStyle)
		default:
			f.SetCellStyle(sheet, cell, cell, baseStyle)
		}
	}
}

// retitlePrintMoneyHeader 将大标题行原单格标题扩展为跨 12 列合并。
// 原"借 方"文字保留在首格。
func retitlePrintMoneyHeader(f *excelize.File, sheet string, col, row int) {
	v, _ := f.GetCellValue(sheet, cellName(col, row))
	if v == "" {
		return // 该列此行无大标题（如明细列）
	}
	// 合并前先清除旧样式冲突：直接合并 [col, col+11]
	f.MergeCell(sheet, cellName(col, row), cellName(col+printSplitCols-1, row))
}

// splitAmountToDigits 把金额字符串（如 "1234.56"、"-11700"、"0"）拆成 12 位
// （十亿千百十万千百十元角分），返回长度 12 的字符串切片，高位空位为 ""。
// 负数忽略符号（账本用方向列表达借贷）。无法解析时返回 nil。
func splitAmountToDigits(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "-")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return nil
	}
	var cents int64
	if idx := strings.Index(s, "."); idx >= 0 {
		intPart, frac := s[:idx], s[idx+1:]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		var iv, fv int64
		if _, err := fmt.Sscanf(intPart, "%d", &iv); err != nil {
			return nil
		}
		if _, err := fmt.Sscanf(frac, "%d", &fv); err != nil {
			return nil
		}
		cents = iv*100 + fv
	} else {
		if _, err := fmt.Sscanf(s, "%d", &cents); err != nil {
			return nil
		}
		cents *= 100
	}
	if cents < 0 {
		cents = -cents
	}
	digits := make([]string, printSplitCols)
	if cents == 0 {
		digits[printSplitCols-1] = "0"
		return digits
	}
	for i := printSplitCols - 1; i >= 0 && cents > 0; i-- {
		digits[i] = string(rune('0' + cents%10))
		cents /= 10
	}
	if cents > 0 {
		return nil // 超过千亿位，不应发生
	}
	return digits
}
