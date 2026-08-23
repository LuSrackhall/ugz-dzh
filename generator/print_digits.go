// Package generator — 打印版位格输出：金额拆位与分组边框常量。
//
// 本文件是打印版与查看版之间唯一的"样式知识"集合：人民币 12 位拆分规则、
// 小列标签、分组竖线样式表。查看版生成代码不依赖本文件；打印变换器消费本文件。
package generator

import "github.com/xuri/excelize/v2"

// digitColLabels 12 小列从左到右的标签（十亿 → 分）。
// 索引：0=十亿 1=亿 2=千万 3=百万 4=十万 5=万 6=千 7=百 8=十 9=元 10=角 11=分
var digitColLabels = [12]string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}

// printDigitFontSize 小格字号（pt）。打印缩放下 11pt 放不进小格，实机校准后可调。
const printDigitFontSize = 7.0

// 分组竖线样式常量。
const (
	divThinGreen  = 0 // 普通绿色细线
	divThickGreen = 1 // 绿色加粗（人民币分段组界）
	divThinRed    = 2 // 红色单细线（元|角）
)

// dividerStyles[i] = 第 i 小列与其右邻之间竖线的样式（i: 0..10）。
//
// 需求分组串「十; 亿, 千, 百; 十, 万, 千; 百, 十, 元. 角, 分」：
//   - ';'（加粗）位于 十|亿(0)、百万|十万(3)、千|百(6) —— 人民币分段组界，共 3 处
//   - '.'（红色单细线）位于 元|角(9) —— 共 1 处
//   - 其余为普通绿色细线
//
// 注意：元在索引 9、角在索引 10，故 元|角 分隔符索引为 9（非 8）。
var dividerStyles = [11]int{
	divThickGreen, // [0]  十|亿      ;
	divThinGreen,  // [1]  亿|千万    ,
	divThinGreen,  // [2]  千万|百万  ,
	divThickGreen, // [3]  百万|十万  ;
	divThinGreen,  // [4]  十万|万    ,
	divThinGreen,  // [5]  万|千      ,
	divThickGreen, // [6]  千|百      ;
	divThinGreen,  // [7]  百|十      ,
	divThinGreen,  // [8]  十|元      ,
	divThinRed,    // [9]  元|角      .
	divThinGreen,  // [10] 角|分      ,
}

// pow10 整数幂表（10^0 .. 10^11），供 splitCNY 使用。
var pow10 = [12]int64{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000, 10000000000, 100000000000}

// splitCNY 将分值拆为 12 个人民币位字符串（十亿…元角分）。
//
// 规则：
//   - 负数取绝对值填格（金额恒按非负处理，正负由"借/贷"方向列表达）
//   - 0 返回全空（金额为 0 时 12 格全部留空）
//   - 高位无效零留空（前导零抑制）；个位对齐「元」列（索引 9）
//
// 索引 i 的位权 = 10^(9-i) 元 = 10^(11-i) 分（i=0..11）。即：
//
//	i=0 十亿(10^9)、…、i=9 元(10^0)、i=10 角(10^-1)、i=11 分(10^-2)
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
	for i := 0; i < 12; i++ {
		d := (v / pow10[11-i]) % 10
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
