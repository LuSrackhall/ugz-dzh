// Package generator — 打印版多区域打印区域。
//
// 背景：打印版 sheet 的正/反面内容分布在左右两个列区（GL 奇偶块交替、ML 滑动窗口），
// 整表打印时"每块的另一半侧"变成整页空白，且默认页序（先下后右）把正反面拆到两段。
// Excel/WPS 对多区域打印区域（_xlnm.Print_Area 多范围）的语义是"每区域独立成页、
// 按列出顺序打印"（已在 WPS Mac 实测验证）：只列出有内容的半块、按阅读顺序排列，
// 导出 PDF 即无空白页且正反面天然配对，双面打印直接成书。
package generator

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// areaRect 打印区域矩形（打印版列行号，1 起）。
type areaRect struct {
	c1, r1, c2, r2 int
	// blank 空白补页矩形：区域内无内容时 WPS 可能跳页，写一个空格确保渲染成页
	blank bool
}

// glAreaPlan 计算总分类账打印 sheet 的多区域打印区域。
// GL 奇偶块交替写正/反面列区：0 基偶数块=正面（左列区）、奇数块=反面（右列区），
// 每块恰好一页 → 按块序列出即 [正1, 反1, 正2, 反2 …]。
// 块数为奇数时补一张空白反面页（右页起两列书口间隙列，必为空），
// 保证整本导出时下一科目从正面开始配对。
func glAreaPlan(lastRow, blockRows, breakPrintCol, maxCol int) []areaRect {
	if lastRow < 1 || blockRows < 1 || breakPrintCol < 2 || maxCol < breakPrintCol {
		return nil
	}
	blocks := (lastRow + blockRows - 1) / blockRows
	rects := make([]areaRect, 0, blocks+1)
	for k := 0; k < blocks; k++ {
		r1 := k*blockRows + 1
		r2 := min(r1+blockRows-1, lastRow)
		if k%2 == 0 {
			rects = append(rects, areaRect{c1: 1, r1: r1, c2: breakPrintCol - 1, r2: r2})
		} else {
			rects = append(rects, areaRect{c1: breakPrintCol, r1: r1, c2: maxCol, r2: r2})
		}
	}
	if blocks%2 == 1 {
		lastBlockStart := (blocks - 1) * blockRows
		rects = append(rects, areaRect{
			c1: breakPrintCol, r1: lastBlockStart + 1, c2: breakPrintCol + 1,
			r2: lastBlockStart + blockRows, blank: true,
		})
	}
	return rects
}

// mlAreaPlan 计算多科目明细账打印 sheet 的多区域打印区域。
// 滑动窗口：块0=(空, 占位正面F1)，中间块k=(反面Bk, 正面Fk+1)，末块=(反面B末, 空)。
// 阅读序 [F1, B1, F2, B2 …] = [右块0, (左块k, 右块k)…, 左末块]，页数恒为偶数，无需补页。
func mlAreaPlan(lastRow, blockRows, breakPrintCol, maxCol int) []areaRect {
	if lastRow < 1 || blockRows < 1 || breakPrintCol < 2 || maxCol < breakPrintCol {
		return nil
	}
	blocks := (lastRow + blockRows - 1) / blockRows
	rect := func(left bool, k int) areaRect {
		r1 := k*blockRows + 1
		r2 := min(r1+blockRows-1, lastRow)
		if left {
			return areaRect{c1: 1, r1: r1, c2: breakPrintCol - 1, r2: r2}
		}
		return areaRect{c1: breakPrintCol, r1: r1, c2: maxCol, r2: r2}
	}
	rects := make([]areaRect, 0, 2*blocks)
	rects = append(rects, rect(false, 0)) // F1 占位页
	for k := 1; k <= blocks-2; k++ {
		rects = append(rects, rect(true, k), rect(false, k))
	}
	if blocks >= 2 {
		rects = append(rects, rect(true, blocks-1)) // 末块反面（末块右侧结构为空）
	}
	return rects
}

// writeSheetPrintArea 把区域列表写为 sheet 级多区域打印区域（_xlnm.Print_Area）。
func writeSheetPrintArea(f *excelize.File, sheet string, rects []areaRect) error {
	if len(rects) == 0 {
		return nil
	}
	quoted := strings.ReplaceAll(sheet, "'", "''")
	parts := make([]string, 0, len(rects))
	for _, rc := range rects {
		if rc.blank {
			// 空白补页：写一个空格，防止 WPS 把"无内容"区域当空页跳过
			_ = f.SetCellValue(sheet, cellAxis(rc.c1, rc.r1), " ")
		}
		parts = append(parts, fmt.Sprintf("'%s'!$%s$%d:$%s$%d", quoted,
			colLetter(rc.c1), rc.r1, colLetter(rc.c2), rc.r2))
	}
	return f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: strings.Join(parts, ","),
		Scope:    sheet,
	})
}
