package voucher

import "testing"

// 审计二审 H2：红字（负数）四种书写格式必须统一解析为负数。
func TestParseAmountRedInkFormats(t *testing.T) {
	cases := []struct {
		in   string
		want int64 // 分
	}{
		{"-500", -50000},          // ASCII 减号
		{"(500)", -50000},         // 括号红字
		{"－500", -50000},          // 全角减号
		{"−500", -50000},          // Unicode 减号（U+2212）
		{"(-500)", -50000},        // 括号+负号（不重复取反）
		{"(11,700.00)", -1170000}, // 括号+千分位
		{"500", 50000},            // 正数不受影响
		{"1,500.50", 150050},      // 千分位
	}
	for _, c := range cases {
		got, ok := parseAmountToCents(c.in)
		if !ok {
			t.Errorf("%q 解析失败（ok=false），want %d", c.in, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("%q → %d 分, want %d 分", c.in, got, c.want)
		}
	}
	// 空串不合法
	if _, ok := parseAmountToCents(""); ok {
		t.Error("空串应解析失败（ok=false）")
	}
}
