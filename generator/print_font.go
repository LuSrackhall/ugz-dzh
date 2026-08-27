// Package generator — 打印版字体统一：宋体加粗。
//
// applyPrintFont 把 sheet 中所有已有样式（含空样式格）的字体统一为 宋体+加粗，
// 保留原字号/颜色/斜体等其余属性。按"原样式→新样式"映射缓存，相同样式只创建一次。
// 注意：必须在金额列展开变换之后调用（拆位小格样式由变换生成，也要统一）；
// 且对 sid=0 的格子跳过——无样式格不写（与查看版渲染一致，避免整表膨胀）。
package generator

import (
	"strings"

	"github.com/xuri/excelize/v2"
)

// printFontName 打印版"默认"区域（表头/标签/摘要等）字体名（可在 print-config.json 的 字体.默认 配置）。
const printFontName = "宋体"

// applyPrintFont 统一 sheet 内所有非零样式的字体为 宋体+Bold（保留其他属性）。
func applyPrintFont(f *excelize.File, sheet string) {
	styleMap := make(map[int]int) // 原styleID → 新styleID
	rows, err := f.GetRows(sheet)
	if err != nil {
		return
	}
	maxRow := len(rows)
	maxCol := 0
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	for r := 1; r <= maxRow; r++ {
		for c := 1; c <= maxCol; c++ {
			cell, _ := excelize.CoordinatesToCellName(c, r)
			sid, err := f.GetCellStyle(sheet, cell)
			if err != nil || sid == 0 {
				continue // 无样式格跳过
			}
			nid, ok := styleMap[sid]
			if !ok {
				st, err := f.GetStyle(sid)
				if err != nil {
					continue
				}
				font := st.Font
				if font == nil {
					font = &excelize.Font{}
				}
				// 已显式指定字体族（如 ML 数据数字 Noteworthy、Front 标题仿宋）→ 保持原字体，
				// 不统一宋体。查看版从不设 Family，故不会误伤查看版样式。
				if font.Family != "" {
					continue
				}
				font.Family = printCfg.字体.默认
				font.Bold = true
				st.Font = font
				nid, err = f.NewStyle(st)
				if err != nil {
					continue
				}
				styleMap[sid] = nid
			}
			_ = f.SetCellStyle(sheet, cell, cell, nid)
		}
	}
}

// isLedgerSheet 判断是否总分类账/多科目明细账 sheet。
func isLedgerSheet(name string) bool {
	return strings.HasPrefix(name, sheetPrefixGL) || strings.HasPrefix(name, sheetPrefixML)
}
