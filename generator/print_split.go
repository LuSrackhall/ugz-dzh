package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
//
// 实现方式：复制查看版 xlsx 为底稿 → 卸载全部合并区 → 从右往左对每个
// 金额列插入 11 小列 → 按新列号重建合并区 → 写小列表头。
// （excelize 的 InsertCols 会拉伸跨越插入点的合并区，故必须先卸载。）

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

	// 小列表头行：首页 SubHeaderRow+1 + 每个过次页后的对应行。
	// GL 翻页结构：过次页(row) → 下边距(+1) → 上边距(+2) → 标题(+3) → 科目(+4) → 空(+5) → 表头1(+6) → 表头2(+7)
	subRows := map[int]bool{}
	dataRows := map[int]bool{}
	subRows[lay.SubHeaderRow+1] = true
	for r := lay.DataStartRow + 1 + lay.TopMarginRows; r <= lastRow; r++ {
		dataRows[r] = true
	}
	for i, r := range rows {
		row := i + 1
		if hasPageBreakAt(r, lay) {
			topMargin := row + 1 + lay.BottomMarginRows
			subRows[topMargin+5] = true
			for b := row; b <= topMargin+6 && b <= lastRow; b++ {
				delete(dataRows, b)
			}
		}
	}

	// 金额列（改造前列号）：正反面各 借/贷/余
	var moneyCols []int
	for _, off := range []int{glColDebit, glColCredit, glColBalance} {
		moneyCols = append(moneyCols, lay.FrontStartCol+off, lay.BackStartCol+off)
	}

	return splitMoneyColumns(f, sheet, moneyCols, lastRow, subRows, dataRows)
}

// ── ML ──

// splitMLSheet 对 ML sheet 拆分借/贷/余额 + 明细1-14 列。
func (wb *Workbook) splitMLSheet(f *excelize.File, sheet string) error {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	lastRow := len(rows)

	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows

	// ML 每页 block（start=上边距行）：h4=start+4 小列表头行；
	// 数据区 = start+DataStartRow .. start+DataStartRow+pageSize（含过次页）
	subRows := map[int]bool{}
	dataRows := map[int]bool{}
	for start := 1; start <= lastRow; start += blockRows {
		subRows[start+4] = true
		for d := start + lay.DataStartRow; d <= start+lay.DataStartRow+pageSize && d <= lastRow; d++ {
			dataRows[d] = true
		}
	}

	// 金额列：Back 借/贷/余 + 明细1-14
	var moneyCols []int
	for _, off := range []int{mlOffDebit, mlOffCredit, mlOffBalance} {
		moneyCols = append(moneyCols, lay.BackStartCol+off)
	}
	for i := 0; i < mlMaxDetails; i++ {
		moneyCols = append(moneyCols, mlDetailCol(lay, i))
	}

	return splitMoneyColumns(f, sheet, moneyCols, lastRow, subRows, dataRows)
}

// ── 核心拆分 ──

// mergeRef 一个合并区记录（改造前坐标）。
type mergeRef struct {
	c1, r1, c2, r2 int
}

// splitMoneyColumns 将 moneyCols 中每个金额列拆为 12 小列。
// 流程：卸载全部合并区 → 从右往左插列拆分 → 重建合并区 → 写表头。
func splitMoneyColumns(
	f *excelize.File,
	sheet string,
	moneyCols []int,
	lastRow int,
	subRows, dataRows map[int]bool,
) error {
	// 1. 记录并卸载全部合并区（先收集完再逐个卸载）
	// MergeCell 格式: []string{"C2:H2", "值"}
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return err
	}
	var refs []mergeRef
	for _, m := range merges {
		parts := strings.Split(m[0], ":")
		if len(parts) != 2 {
			continue
		}
		c1, r1, err1 := excelize.CellNameToCoordinates(parts[0])
		c2, r2, err2 := excelize.CellNameToCoordinates(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		refs = append(refs, mergeRef{c1, r1, c2, r2})
	}
	for _, ref := range refs {
		if err := f.UnmergeCell(sheet, cellName(ref.c1, ref.r1), cellName(ref.c2, ref.r2)); err != nil {
			return fmt.Errorf("卸载合并区: %w", err)
		}
	}

	// 2. 从右往左逐列拆分
	cols := make([]int, len(moneyCols))
	copy(cols, moneyCols)
	sortDesc(cols)

	widths := make(map[int]float64, len(cols))
	for _, col := range cols {
		w, err := f.GetColWidth(sheet, cellColLetter(col))
		if err != nil {
			return err
		}
		widths[col] = float64(w)
	}
	for _, col := range cols {
		if err := splitOneMoneyColumn(f, sheet, col, lastRow, widths[col], dataRows); err != nil {
			return err
		}
	}

	// 3. 重建合并区：跨金额列的合并区扩展覆盖其 12 小列；其余原样平移。
	// 改造后列号换算：origCol 在第 k 个金额格内（k 为升序序号）→ newCol = origCol + k*11
	sortedAsc := make([]int, len(cols))
	copy(sortedAsc, cols)
	sortAsc(sortedAsc)
	mapCol := func(origCol int) int {
		k := 0
		for i, c := range sortedAsc {
			if origCol > c {
				k = i + 1
			}
		}
		return origCol + k*(printSplitCols-1)
	}

	for _, ref := range refs {
		nc1, nc2 := mapCol(ref.c1), mapCol(ref.c2)
		if err := f.MergeCell(sheet, cellName(nc1, ref.r1), cellName(nc2, ref.r2)); err != nil {
			return fmt.Errorf("重建合并区 %d:%d-%d:%d: %w", ref.c1, ref.r1, ref.c2, ref.r2, err)
		}
	}

	// 4. 统一写小列表头（新列号）
	for k, origCol := range sortedAsc {
		newCol := origCol + k*(printSplitCols-1)
		for sr := range subRows {
			writePrintSubHeader(f, sheet, newCol, sr)
		}
	}
	return nil
}

// splitOneMoneyColumn 拆分单个金额列（仅插列+均分宽+数据填位）。
func splitOneMoneyColumn(
	f *excelize.File,
	sheet string,
	col int,
	lastRow int,
	origWidth float64,
	dataRows map[int]bool,
) error {
	type cellVal struct {
		row int
		val string
	}
	var vals []cellVal
	for row := 1; row <= lastRow; row++ {
		if !dataRows[row] {
			continue // 非数据区（页码/科目/标题/边距等）一律不动
		}
		v, _ := f.GetCellValue(sheet, cellName(col, row))
		if v != "" {
			vals = append(vals, cellVal{row, v})
		}
	}

	if err := f.InsertCols(sheet, cellColLetter(col+1), printSplitCols-1); err != nil {
		return fmt.Errorf("插入列: %w", err)
	}

	unit := origWidth / float64(printSplitCols)
	for i := 0; i < printSplitCols; i++ {
		f.SetColWidth(sheet, cellColLetter(col+i), cellColLetter(col+i), unit)
	}

	for _, cv := range vals {
		digits := splitAmountToDigits(cv.val)
		if digits == nil {
			continue
		}
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

// splitAmountToDigits 把金额字符串拆成 12 位（十亿千百十万千百十元角分），
// 高位空位为 ""。负数忽略符号。无法解析时返回 nil。
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
		return nil
	}
	return digits
}

// sortDesc / sortAsc 简单排序（列数少，无需 sort 包泛型开销）。
func sortDesc(a []int) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] > a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func sortAsc(a []int) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}
