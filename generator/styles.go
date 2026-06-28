package generator

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// TableStyles 共享样式定义
type TableStyles struct {
	HeaderBgColor  string  // 表头背景色
	HeaderFontBold bool    // 表头加粗
	DataFontSize   float64 // 数据行字号
	BorderColor    string  // 边框颜色
	BorderWidth    int     // 边框宽度
}

// DefaultStyles 默认样式
var DefaultStyles = TableStyles{
	HeaderBgColor:  "#D9E1F2",
	HeaderFontBold: true,
	DataFontSize:   9,
	BorderColor:    "#808080",
	BorderWidth:    1,
}

// ApplyToExcel 将样式应用到 Excel
func (s *TableStyles) ApplyToExcel(f *excelize.File, sheet string, startCell, endCell string, styleType string) {
	var style *excelize.Style

	switch styleType {
	case "header":
		style = &excelize.Style{
			Font: &excelize.Font{
				Bold: s.HeaderFontBold,
				Size: 10,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{s.HeaderBgColor},
				Pattern: 1,
			},
			Border: []excelize.Border{
				{Type: "left", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "right", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "top", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "bottom", Color: s.BorderColor, Style: s.BorderWidth},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		}
	case "data":
		style = &excelize.Style{
			Font: &excelize.Font{
				Size: s.DataFontSize,
			},
			Border: []excelize.Border{
				{Type: "left", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "right", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "top", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "bottom", Color: s.BorderColor, Style: s.BorderWidth},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		}
	case "amount":
		style = &excelize.Style{
			Font: &excelize.Font{
				Size: s.DataFontSize,
			},
			Border: []excelize.Border{
				{Type: "left", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "right", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "top", Color: s.BorderColor, Style: s.BorderWidth},
				{Type: "bottom", Color: s.BorderColor, Style: s.BorderWidth},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		}
	default:
		return
	}

	styleID, err := f.NewStyle(style)
	if err != nil {
		return
	}

	f.SetCellStyle(sheet, startCell, endCell, styleID)
}

// ToCSS 生成 CSS 样式字符串
func (s *TableStyles) ToCSS() string {
	return fmt.Sprintf(`
	.header {
		background-color: %s;
		font-weight: %s;
		font-size: 10pt;
		text-align: center;
		vertical-align: middle;
	}
	.data {
		font-size: %.0fpt;
		text-align: center;
		vertical-align: middle;
	}
	.amount-cell {
		font-size: %.0fpt;
		text-align: center;
		vertical-align: middle;
		display: inline-block;
		width: 1.2em;
		border-left: 1px solid %s;
	}
	td {
		border: %dpx solid %s;
		padding: 2px 4px;
	}
	`, s.HeaderBgColor,
		map[bool]string{true: "bold", false: "normal"}[s.HeaderFontBold],
		s.DataFontSize,
		s.DataFontSize,
		s.BorderColor,
		s.BorderWidth, s.BorderColor)
}
