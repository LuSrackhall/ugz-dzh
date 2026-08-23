package generator

import "testing"

func TestSplitCNY(t *testing.T) {
	cases := []struct {
		name  string
		cents int64
		want  [12]string
	}{
		{
			name: "零全空", cents: 0,
			want: [12]string{},
		},
		{
			name: "12345.67元", cents: 1234567,
			// 万=1 千=2 百=3 十=4 元=5 角=6 分=7；十万及以上留空
			want: [12]string{"", "", "", "", "", "1", "2", "3", "4", "5", "6", "7"},
		},
		{
			name: "9.20元", cents: 920,
			// 元=9 角=2 分=0；角位零非前导，须填"0"；分位零也填（在 started 之后）
			want: [12]string{"", "", "", "", "", "", "", "", "", "9", "2", "0"},
		},
		{
			name: "1000000000.00元(十亿整)", cents: 100000000000,
			// 十亿=1，其后亿/千万/.../元均为 0（started 后须填 0），角分全 0
			want: [12]string{"1", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"},
		},
		{
			name: "负数取绝对值 -123.45", cents: -12345,
			want: [12]string{"", "", "", "", "", "", "", "1", "2", "3", "4", "5"},
		},
		{
			name: "整百 200.00", cents: 20000,
			// 200.00 → 百=2；started 后十/元/角/分均填 0；千位(索引6)在前导，留空
			want: [12]string{"", "", "", "", "", "", "", "2", "0", "0", "0", "0"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitCNY(c.cents)
			for i := 0; i < 12; i++ {
				if got[i] != c.want[i] {
					t.Errorf("位 %d(%s): got %q want %q; full got=%v want=%v",
						i, digitColLabels[i], got[i], c.want[i], got, c.want)
					return
				}
			}
		})
	}
}

func TestDividerStyles(t *testing.T) {
	// 加粗（;）位于索引 0、3、6；红细（.）位于索引 9；其余细线。
	wantThick := map[int]bool{0: true, 3: true, 6: true}
	wantRed := map[int]bool{9: true}
	for i := 0; i < 11; i++ {
		got := dividerStyles[i]
		switch {
		case wantThick[i]:
			if got != divThickGreen {
				t.Errorf("索引 %d: 期望加粗 divThickGreen, got %d", i, got)
			}
		case wantRed[i]:
			if got != divThinRed {
				t.Errorf("索引 %d: 期望红细 divThinRed, got %d", i, got)
			}
		default:
			if got != divThinGreen {
				t.Errorf("索引 %d: 期望普通细线 divThinGreen, got %d", i, got)
			}
		}
	}
	if len(dividerStyles) != 11 {
		t.Errorf("dividerStyles 长度 = %d, 期望 11", len(dividerStyles))
	}
	if len(digitColLabels) != 12 {
		t.Errorf("digitColLabels 长度 = %d, 期望 12", len(digitColLabels))
	}
	// 元在索引 9、角在索引 10 —— 元|角分隔符索引 9，红色线须落于此。
	if digitColLabels[9] != "元" || digitColLabels[10] != "角" {
		t.Errorf("元/角标签位置错误: 索引9=%q 索引10=%q", digitColLabels[9], digitColLabels[10])
	}
	if dividerStyles[9] != divThinRed {
		t.Errorf("元|角(索引9)应为红细线, got %d", dividerStyles[9])
	}
}

func TestDividerBorder(t *testing.T) {
	c, s := dividerBorder(divThickGreen)
	if c != "#006100" || s != 2 {
		t.Errorf("加粗边框: got (%q,%d) want (#006100,2)", c, s)
	}
	c, s = dividerBorder(divThinRed)
	if c != "CC0000" || s != 1 {
		t.Errorf("红细边框: got (%q,%d) want (CC0000,1)", c, s)
	}
	c, s = dividerBorder(divThinGreen)
	if c != "#006100" || s != 1 {
		t.Errorf("普通细线: got (%q,%d) want (#006100,1)", c, s)
	}
}
