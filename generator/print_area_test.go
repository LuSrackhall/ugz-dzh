package generator

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// GL 结构：奇偶块交替正面(左列区)/反面(右列区)，28 行一块。
func TestGLAreaPlan(t *testing.T) {
	const blockRows, breakCol, maxCol = 28, 51, 102

	t.Run("六块科目（如银行存款167行）：六区域，偶数块无补白", func(t *testing.T) {
		rects := glAreaPlan(167, blockRows, breakCol, maxCol)
		if len(rects) != 6 {
			t.Fatalf("期望 6 个区域（偶数块无补白），得到 %d", len(rects))
		}
		want := []areaRect{
			{c1: 1, r1: 1, c2: 50, r2: 28},
			{c1: 51, r1: 29, c2: 102, r2: 56},
			{c1: 1, r1: 57, c2: 50, r2: 84},
			{c1: 51, r1: 85, c2: 102, r2: 112},
			{c1: 1, r1: 113, c2: 50, r2: 140},
			{c1: 51, r1: 141, c2: 102, r2: 167},
		}
		for i, w := range want {
			if rects[i] != w {
				t.Errorf("区域%d = %+v, 期望 %+v", i, rects[i], w)
			}
		}
	})

	t.Run("单块科目（27行）：正面一页+补白反面页", func(t *testing.T) {
		rects := glAreaPlan(27, blockRows, breakCol, maxCol)
		if len(rects) != 2 {
			t.Fatalf("期望 2 个区域，得到 %d", len(rects))
		}
		if rects[0] != (areaRect{c1: 1, r1: 1, c2: 50, r2: 27}) {
			t.Errorf("正面区域 = %+v", rects[0])
		}
		if rects[1] != (areaRect{c1: 51, r1: 1, c2: 52, r2: 28, blank: true}) {
			t.Errorf("补白区域 = %+v", rects[1])
		}
	})

	t.Run("偶数块科目（56行）：无补白", func(t *testing.T) {
		rects := glAreaPlan(56, blockRows, breakCol, maxCol)
		if len(rects) != 2 || rects[1].blank {
			t.Fatalf("偶数块不应补白: %+v", rects)
		}
	})

	t.Run("尾块不足整块行数时行号截断", func(t *testing.T) {
		rects := glAreaPlan(30, blockRows, breakCol, maxCol) // 2块: 1-28, 29-30
		if len(rects) != 2 {
			t.Fatalf("期望 2 个区域，得到 %d", len(rects))
		}
		if rects[1] != (areaRect{c1: 51, r1: 29, c2: 102, r2: 30}) {
			t.Errorf("尾块反面区域 = %+v", rects[1])
		}
	})
}

// ML 结构：块=页（右半正面、左半反面，同页码）；块0=占位页（只写正面）。
// 打印序 [占位正, 页1正, 页1反, 页2正, 页2反, …] + 尾部补空白保持偶数。
func TestMLAreaPlan(t *testing.T) {
	const blockRows, breakCol, maxCol = 30, 82, 184

	t.Run("六块（180行）：11 区域+补空白=12（偶数，末页正面不遗漏）", func(t *testing.T) {
		rects := mlAreaPlan(180, blockRows, breakCol, maxCol)
		if len(rects) != 12 {
			t.Fatalf("期望 12 个区域，得到 %d（11 内容 + 1 补空白）", len(rects))
		}
		// 期望序列: R0, R1, L1, R2, L2, R3, L3, R4, L4, R5, L5, 空白
		want := []areaRect{
			{c1: 82, r1: 1, c2: 184, r2: 30},   // 占位页正面
			{c1: 82, r1: 31, c2: 184, r2: 60},  // 页1 正面（此前遗漏！）
			{c1: 1, r1: 31, c2: 81, r2: 60},    // 页1 反面
			{c1: 82, r1: 61, c2: 184, r2: 90},  // 页2 正面
			{c1: 1, r1: 61, c2: 81, r2: 90},    // 页2 反面
			{c1: 82, r1: 91, c2: 184, r2: 120}, // 页3 正面
			{c1: 1, r1: 91, c2: 81, r2: 120},   // 页3 反面
			{c1: 82, r1: 121, c2: 184, r2: 150},
			{c1: 1, r1: 121, c2: 81, r2: 150},
			{c1: 82, r1: 151, c2: 184, r2: 180},             // 页5 正面（末页，此前遗漏！）
			{c1: 1, r1: 151, c2: 81, r2: 180},               // 页5 反面
			{c1: 82, r1: 181, c2: 83, r2: 210, blank: true}, // 补空白（块后空行区，偶数配对）
		}
		for i, w := range want {
			if rects[i] != w {
				t.Errorf("区域%d = %+v, 期望 %+v", i, rects[i], w)
			}
		}
	})

	t.Run("两块（59行）：占位正面+页1正反+补空白", func(t *testing.T) {
		rects := mlAreaPlan(59, blockRows, breakCol, maxCol)
		if len(rects) != 4 {
			t.Fatalf("期望 4 个区域，得到 %d", len(rects))
		}
		// 页1 正面（右半行 31-59）必须在——旧实现遗漏此区域
		if rects[1] != (areaRect{c1: 82, r1: 31, c2: 184, r2: 59}) {
			t.Errorf("区域1（页1正面） = %+v", rects[1])
		}
		if rects[2] != (areaRect{c1: 1, r1: 31, c2: 81, r2: 59}) {
			t.Errorf("区域2（页1反面） = %+v", rects[2])
		}
		if !rects[3].blank {
			t.Errorf("区域3 应为补空白: %+v", rects[3])
		}
	})

	t.Run("单块（30行）：占位正面+补空白", func(t *testing.T) {
		rects := mlAreaPlan(30, blockRows, breakCol, maxCol)
		if len(rects) != 2 || rects[0].c1 != breakCol || !rects[1].blank {
			t.Fatalf("期望 [占位正面, 补空白]: %+v", rects)
		}
	})
}

func TestWriteSheetPrintArea(t *testing.T) {
	f := excelize.NewFile()
	const sheet = "总分类账-测试"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatal(err)
	}
	rects := []areaRect{
		{c1: 1, r1: 1, c2: 50, r2: 28},
		{c1: 51, r1: 29, c2: 102, r2: 56},
		{c1: 51, r1: 57, c2: 52, r2: 84, blank: true},
	}
	if err := writeSheetPrintArea(f, sheet, rects); err != nil {
		t.Fatalf("写打印区域: %v", err)
	}
	var found *excelize.DefinedName
	for _, dn := range f.GetDefinedName() {
		if dn.Name == "_xlnm.Print_Area" && dn.Scope == sheet {
			found = &dn
			break
		}
	}
	if found == nil {
		t.Fatal("未找到 _xlnm.Print_Area defined name")
	}
	want := "'" + sheet + "'!$A$1:$AX$28,'" + sheet + "'!$AY$29:$CX$56,'" + sheet + "'!$AY$57:$AZ$84"
	if strings.ReplaceAll(found.RefersTo, "'", "'") != want {
		t.Errorf("RefersTo = %s\n期望 = %s", found.RefersTo, want)
	}
	// 补白区域首格应写入了空格
	v, _ := f.GetCellValue(sheet, cellAxis(51, 57))
	if v != " " {
		t.Errorf("补白区域首格 = %q, 期望空格", v)
	}
}
