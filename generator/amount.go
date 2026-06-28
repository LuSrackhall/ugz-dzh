package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// centsToDigits 将金额（分）拆为 12 位数字数组
// 返回 [十亿, 亿, 千万, 百万, 十万, 万, 千, 百, 十, 元, 角, 分]
func centsToDigits(cents int64) [12]int {
	var result [12]int

	// 负数取绝对值，方向由"方向"列标记
	if cents < 0 {
		cents = -cents
	}

	// 从低位到高位填充
	for i := 11; i >= 0; i-- {
		result[i] = int(cents % 10)
		cents /= 10
	}

	return result
}

// writeAmountCells 将金额写入 Excel 的 12 个连续单元格
func writeAmountCells(f *excelize.File, sheet string, row, startCol int, cents int64, styleID int) {
	digits := centsToDigits(cents)

	for i, d := range digits {
		col := startCol + i
		cell, _ := excelize.CoordinatesToCellName(col, row)

		// 元角分位总是显示，高位为 0 时留空
		if d > 0 || i >= 9 {
			f.SetCellValue(sheet, cell, d)
		} else {
			f.SetCellValue(sheet, cell, "")
		}

		// 应用样式
		if styleID > 0 {
			f.SetCellStyle(sheet, cell, cell, styleID)
		}
	}
}

// formatAmountForDisplay 将金额格式化为带千分位的显示字符串（用于调试）
func formatAmountForDisplay(cents int64) string {
	if cents == 0 {
		return "0.00"
	}

	// 负数处理
	negative := false
	if cents < 0 {
		negative = true
		cents = -cents
	}

	// 分离元和分
	yuan := cents / 100
	fen := cents % 100

	// 格式化元部分（带千分位）
	yuanStr := strconv.FormatInt(yuan, 10)
	if len(yuanStr) > 3 {
		var parts []string
		for len(yuanStr) > 3 {
			parts = append([]string{yuanStr[len(yuanStr)-3:]}, parts...)
			yuanStr = yuanStr[:len(yuanStr)-3]
		}
		if len(yuanStr) > 0 {
			parts = append([]string{yuanStr}, parts...)
		}
		yuanStr = strings.Join(parts, ",")
	}

	// 组合结果
	result := fmt.Sprintf("%s.%02d", yuanStr, fen)
	if negative {
		result = "-" + result
	}

	return result
}
