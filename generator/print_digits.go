// Package generator — 打印版位格输出：金额拆位与分组边框常量。
//
// 本文件是打印版与查看版之间唯一的"样式知识"集合：人民币位拆分规则、
// 小列标签、分组竖线样式表。查看版生成代码不依赖本文件；打印变换器消费本文件。
//
// 列数参数化：
//   - GL / ML 借贷余金额列：12 列（十亿…分）
//   - ML 借贷余：11 列（亿…分，去十亿位）
//   - ML 明细列：10 列（千万…分，去十亿/亿位）
// 所有函数按列数 n 参数化：splitCNY(cents, n)、digitColLabels(n)、dividerStyles(n)。
// 元在索引 n-3（n=12→9、11→8、10→7），元|角 红细线分隔符索引 = n-4。
package generator

import "github.com/xuri/excelize/v2"

// printDigitFontSize 小格字号（pt）。打印缩放下 11pt 放不进小格，实机校准后可调。
const printDigitFontSize = 7.0

// 分组竖线样式常量。
const (
	divThinGreen  = 0 // 普通绿色细线
	divThickGreen = 1 // 绿色加粗（人民币分段组界）
	divThinRed    = 2 // 红色单细线（元|角）
)

// digitColLabels 返回 n 列的小列标签（从左到右，十亿/亿/千万…分）。
//
//	n=12: 十 亿 千 百 十 万 千 百 十 元 角 分 （十亿…分）
//	n=11: 亿 千 百 十 万 千 百 十 元 角 分   （亿…分，去十亿）
//	n=10: 千 百 十 万 千 百 十 元 角 分      （千万…分，去十亿/亿）
//
// 索引规则：i=0 最高位；元在 n-3；角 n-2；分 n-1。
func digitColLabels(n int) []string {
	switch n {
	case 12:
		return []string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}
	case 11:
		return []string{"亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}
	case 10:
		return []string{"千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}
	}
	return nil
}

// dividerStyles 返回 n 列的分组竖线样式表（长度 n-1，第 i 小列与其右邻之间）。
//
// 需求分组串「十; 亿, 千, 百; 十, 万, 千; 百, 十, 元. 角, 分」：
//   - ';'（加粗）位于 十|亿(0)、百万|十万(3)、千|百(6) —— 人民币分段组界，共 3 处
//   - '.'（红色单细线）位于 元|角 —— 共 1 处
//   - 其余为普通绿色细线
//
// 元|角 分隔符索引 = n-4（n=12→9、11→7？不对——见下）。
func dividerStyles(n int) []int {
	labels := digitColLabels(n)
	// 定位 元|角 分隔符：元在 labels 中的索引为 yuanIdx，其右邻分隔符索引 = yuanIdx
	// （分隔符 i 位于标签 i 与 i+1 之间）。
	yuanIdx := -1
	for i, lb := range labels {
		if lb == "元" {
			yuanIdx = i
			break
		}
	}
	out := make([]int, n-1)
	for i := 0; i < n-1; i++ {
		switch {
		case i == yuanIdx: // 元|角
			out[i] = divThinRed
		case i == 0 || i == 3 || i == 6: // 加粗组界（十|亿 / 百万|十万 / 千|百）
			out[i] = divThickGreen
		default:
			out[i] = divThinGreen
		}
	}
	return out
}

// pow10 整数幂表（10^0 .. 10^13），供 splitCNY 使用。
var pow10 = [14]int64{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000, 10000000000, 100000000000, 1000000000000, 10000000000000}

// splitCNY 将分值拆为 n 个人民币位字符串（n=12 十亿…分 / n=11 亿…分 / n=10 千万…分）。
//
// 规则：
//   - 负数取绝对值填格（金额恒按非负处理，正负由"借/贷"方向列表达）
//   - 0 返回全空（金额为 0 时 n 格全部留空）
//   - 高位无效零留空（前导零抑制）；个位对齐「元」列（索引 n-3）
//
// 索引 i 的位权 = 10^(n-3-i) 元（i=0..n-1）。即 n=12 时 i=0 十亿(10^9)、…、
// i=9 元(10^0)、i=10 角(10^-1)、i=11 分(10^-2)。
func splitCNY(cents int64, n int) []string {
	out := make([]string, n)
	if cents == 0 {
		return out
	}
	v := cents
	if v < 0 {
		v = -v
	}
	started := false
	for i := 0; i < n; i++ {
		d := (v / pow10[n-1-i]) % 10
		if d != 0 {
			started = true
		}
		if started {
			out[i] = string(rune('0' + d))
		}
	}
	return out
}

// dividerBorder 将分组样式常量转为 (颜色, excelize 边框样式)。
// 加粗用 Style 2（medium），细线用 Style 1（thin）—— 与查看版金额样式一致。
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

// borderOf 从指定 styleID 读取某一边的边框定义；无则返回 nil。
// 同一打印版文件内 styleID 跨 Sheet 重建仍然有效（样式表是工作簿级）。
func borderOf(f *excelize.File, styleID int, side string) *excelize.Border {
	st, err := f.GetStyle(styleID)
	if err != nil {
		return nil
	}
	for i := range st.Border {
		if st.Border[i].Type == side {
			b := st.Border[i]
			return &b
		}
	}
	return nil
}
