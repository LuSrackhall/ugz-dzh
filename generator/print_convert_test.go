package generator

import (
	"strings"
	"testing"
)

func TestSplitCNY(t *testing.T) {
	cases := []struct {
		name  string
		cents int64
		want  string // 12 位连写，空位用 '_' 表示
	}{
		{"零", 0, "____________"},
		{"一分", 1, "___________1"},
		{"五元整", 500, "_________500"},
		{"一角二分", 12, "__________12"},
		{"普通金额", 1234567, "_____1234567"},
		{"十亿上限减一", 9999999999999, "999999999999"},
		{"负数取绝对值", -12345, "_______12345"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitCNY(tc.cents)
			var b strings.Builder
			for _, s := range got {
				if s == "" {
					b.WriteByte('_')
				} else {
					b.WriteString(s)
				}
			}
			if b.String() != tc.want {
				t.Fatalf("splitCNY(%d) = %q, want %q", tc.cents, b.String(), tc.want)
			}
		})
	}
}

func TestDigitColLabels(t *testing.T) {
	want := [12]string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"}
	if digitColLabels != want {
		t.Fatalf("digitColLabels = %v, want %v", digitColLabels, want)
	}
}

func TestDividerStyles(t *testing.T) {
	// 组界加粗：十|亿、百|十、千|百
	for _, i := range []int{0, 3, 6} {
		if dividerStyles[i] != divThickGreen {
			t.Errorf("dividerStyles[%d] = %d, want 加粗(%d)", i, dividerStyles[i], divThickGreen)
		}
	}
	// 元|角 红色单细线
	if dividerStyles[8] != divThinRed {
		t.Errorf("dividerStyles[8] = %d, want 红细线(%d)", dividerStyles[8], divThinRed)
	}
	// 其余绿细线
	for i, v := range dividerStyles {
		if i == 0 || i == 3 || i == 6 || i == 8 {
			continue
		}
		if v != divThinGreen {
			t.Errorf("dividerStyles[%d] = %d, want 绿细线(%d)", i, v, divThinGreen)
		}
	}
}

func TestShiftColsByInserts(t *testing.T) {
	// 原始插入点：列 9、11、14 各右侧插 11 列
	inserts := []int{9, 11, 14}
	cases := []struct {
		col  int
		want int
	}{
		{8, 8},   // 在所有插入点左侧，不动
		{9, 9},   // 等于插入点：该列本身是金额列首格，不动
		{10, 21}, // 越过 9 → +11
		{12, 34}, // 越过 9、11 → +22（比较基于原始列号）
		{15, 48}, // 越过全部三次 → +33
	}
	for _, tc := range cases {
		if got := shiftColsByInserts(tc.col, inserts); got != tc.want {
			t.Errorf("shiftColsByInserts(%d) = %d, want %d", tc.col, got, tc.want)
		}
	}
}
