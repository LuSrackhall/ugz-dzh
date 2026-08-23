// Package generator — 打印版位格输出：共享常量与工具函数。
package generator

import (
	"math"
	"sort"

	"ledger/generator/layout"
)

// 12 小列从左到右的标签（十亿 → 分）。
var digitColLabels = [12]string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}

// 小格字体字号（74% 打印缩放下 11pt 放不进小格，实机校准后可调）。
const printDigitFontSize = 7.0

// 分组边框常量值。
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
// 负数取绝对值填格；0 返回全空；高位无效零留空。
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

// dividerBorder 将分组样式常量转为边框定义。
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

// printColStart 计算打印版中逻辑 offset 对应的物理起始列号。
// moneyOffsets 中的每个 offset 展开为 12 列，其余为 1 列。
func printColStart(base, offset int, moneyOffsets []int) int {
	shift := 0
	for _, mo := range moneyOffsets {
		if offset > mo {
			shift += 11
		}
	}
	return base + offset + shift
}

// printTotalCols 计算打印版总物理列数。
func printTotalCols(logicalCols int, nMoneyCols int) int {
	return logicalCols + nMoneyCols*11
}

// glPrintMoneyOffsets GL 每面区的金额列 offset。
var glPrintMoneyOffsets = []int{glColDebit, glColCredit, glColBalance}

// mlBackPrintMoneyOffsets ML Back 侧金额列 offset（借/贷/余额 + 明细1-4）。
func mlBackPrintMoneyOffsets() []int {
	offs := []int{mlOffDebit, mlOffCredit, mlOffBalance}
	for i := 0; i < 4; i++ {
		offs = append(offs, mlDetailCol(layout.MLLayout{}, i))
	}
	return offs
}

// mlFrontPrintMoneyOffsets ML Front 侧金额列 offset（明细5-14）。
func mlFrontPrintMoneyOffsets() []int {
	var offs []int
	for i := 4; i < mlMaxDetails; i++ {
		offs = append(offs, mlDetailCol(layout.MLLayout{}, i))
	}
	return offs
}

// sortedMerge 将两个已排序的 int slice 合并去重。
func sortedMerge(a, b []int) []int {
	m := make(map[int]bool)
	for _, v := range a {
		m[v] = true
	}
	for _, v := range b {
		m[v] = true
	}
	out := make([]int, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
