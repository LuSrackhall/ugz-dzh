package generator

import "testing"

func TestSplitCNY(t *testing.T) {
	cases := []struct {
		name  string
		cents int64
		n     int
		want  []string
	}{
		{
			name: "零全空", cents: 0, n: 12,
			want: make([]string, 12),
		},
		{
			name: "12列 12345.67元", cents: 1234567, n: 12,
			// 万=1 千=2 百=3 十=4 元=5 角=6 分=7；十万及以上留空
			want: []string{"", "", "", "", "", "1", "2", "3", "4", "5", "6", "7"},
		},
		{
			name: "12列 9.20元", cents: 920, n: 12,
			// 元=9 角=2 分=0；角位零非前导，须填"0"；分位零也填（在 started 之后）
			want: []string{"", "", "", "", "", "", "", "", "", "9", "2", "0"},
		},
		{
			name: "12列 十亿整", cents: 100000000000, n: 12,
			// 十亿=1，其后亿/千万/.../元均为 0（started 后须填 0），角分全 0
			want: []string{"1", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"},
		},
		{
			name: "12列 负数取绝对值 -123.45", cents: -12345, n: 12,
			want: []string{"", "", "", "", "", "", "", "1", "2", "3", "4", "5"},
		},
		{
			name: "12列 整百 200.00", cents: 20000, n: 12,
			// 200.00 → 百=2；started 后十/元/角/分均填 0；千位(索引6)在前导，留空
			want: []string{"", "", "", "", "", "", "", "2", "0", "0", "0", "0"},
		},
		{
			name: "11列 12345.67元", cents: 1234567, n: 11,
			// 11列: 亿 千 百 十 万 千 百 十 元 角 分 → 万=1 千=2 百=3 十=4 元=5 角=6 分=7
			want: []string{"", "", "", "", "1", "2", "3", "4", "5", "6", "7"},
		},
		{
			name: "11列 9.20元", cents: 920, n: 11,
			// 11列: 亿千 百十 万千 百十 元角分 → 元=索引8 角=9 分=10
			want: []string{"", "", "", "", "", "", "", "", "9", "2", "0"},
		},
		{
			name: "10列 12345.67元", cents: 1234567, n: 10,
			// 10列: 千 百 十 万 千 百 十 元 角 分 → 万=1 千=2 百=3 十=4 元=5 角=6 分=7
			want: []string{"", "", "", "1", "2", "3", "4", "5", "6", "7"},
		},
		{
			name: "10列 9.20元", cents: 920, n: 10,
			// 10列: 千百十万 千百十元角分 → 元=索引7 角=8 分=9
			want: []string{"", "", "", "", "", "", "", "9", "2", "0"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitCNY(c.cents, c.n)
			if len(got) != len(c.want) {
				t.Fatalf("长度: got %d want %d", len(got), len(c.want))
			}
			labels := digitColLabels(c.n)
			for i := 0; i < len(c.want); i++ {
				if got[i] != c.want[i] {
					t.Errorf("位 %d(%s): got %q want %q; full got=%v want=%v",
						i, labels[i], got[i], c.want[i], got, c.want)
					return
				}
			}
		})
	}
}

func TestDividerStyles(t *testing.T) {
	// 12 列：加粗（;）位于索引 0、3、6；红细（.）位于索引 9；其余细线。
	ds12 := dividerStyles(12)
	if len(ds12) != 11 {
		t.Fatalf("dividerStyles(12) 长度 = %d, 期望 11", len(ds12))
	}
	wantThick := map[int]bool{0: true, 3: true, 6: true}
	for i := 0; i < 11; i++ {
		switch {
		case wantThick[i]:
			if ds12[i] != divThickGreen {
				t.Errorf("12列 索引 %d: 期望加粗 divThickGreen, got %d", i, ds12[i])
			}
		case i == 9:
			if ds12[i] != divThinRed {
				t.Errorf("12列 索引 %d(元|角): 期望红细 divThinRed, got %d", i, ds12[i])
			}
		default:
			if ds12[i] != divThinGreen {
				t.Errorf("12列 索引 %d: 期望普通细线 divThinGreen, got %d", i, ds12[i])
			}
		}
	}
	// 11 列：元在索引 8、角在索引 9 → 元|角 分隔符索引 8。
	ds11 := dividerStyles(11)
	if len(ds11) != 10 {
		t.Fatalf("dividerStyles(11) 长度 = %d, 期望 10", len(ds11))
	}
	labels11 := digitColLabels(11)
	if labels11[8] != "元" || labels11[9] != "角" {
		t.Errorf("11列 元/角标签位置错误: 索引8=%q 索引9=%q", labels11[8], labels11[9])
	}
	if ds11[8] != divThinRed {
		t.Errorf("11列 元|角(索引8)应为红细线, got %d", ds11[8])
	}
	// 10 列：元在索引 7、角在索引 8 → 元|角 分隔符索引 7。
	ds10 := dividerStyles(10)
	if len(ds10) != 9 {
		t.Fatalf("dividerStyles(10) 长度 = %d, 期望 9", len(ds10))
	}
	labels10 := digitColLabels(10)
	if labels10[7] != "元" || labels10[8] != "角" {
		t.Errorf("10列 元/角标签位置错误: 索引7=%q 索引8=%q", labels10[7], labels10[8])
	}
	if ds10[7] != divThinRed {
		t.Errorf("10列 元|角(索引7)应为红细线, got %d", ds10[7])
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
