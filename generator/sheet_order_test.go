package generator

import (
	"testing"

	"ledger/balance"

	"github.com/xuri/excelize/v2"
)

func newOrderTestWorkbook(t *testing.T, sheets []string, order []string, mergeGL []string) *Workbook {
	t.Helper()
	f := excelize.NewFile()
	for _, s := range sheets {
		if _, err := f.NewSheet(s); err != nil {
			t.Fatalf("创建测试 Sheet %s: %v", s, err)
		}
	}
	// 删除 excelize 默认页（须先切走激活态，且确保还有其他 Sheet 存在）
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("删除默认 Sheet1: %v", err)
	}
	for _, s := range f.GetSheetList() {
		if s == "Sheet1" {
			t.Fatal("默认 Sheet1 未删除干净")
		}
	}
	return &Workbook{
		File:   f,
		Config: &balance.GlobalConfig{Settings: balance.GlobalSettings{Order: order, MergeGLAccounts: mergeGL}},
		Month:  "2026-01",
	}
}

func sheetList(t *testing.T, wb *Workbook) []string {
	t.Helper()
	return wb.File.GetSheetList()
}

func TestReorderSubjectSheets(t *testing.T) {
	// 乱序创建：GL 叶子混排 + ML 在 GL 前 + 合并账页混在 GL 里
	wb := newOrderTestWorkbook(t, []string{
		"2026-01期初",
		"多科目明细账-管理费用",
		"总分类账-应付款-张三",
		"总分类账-固定资产", // 合并父级账页（与叶子 GL 同名前缀）
		"总分类账-库存现金",
		"总分类账-银行存款-工商银行",
		"总分类账-管理费用-办公费",
		"现金日记账",
		"2026-01期末",
	}, []string{"银行存款", "管理费用"}, []string{"固定资产"})

	wb.reorderSubjectSheets()

	got := sheetList(t, wb)
	want := []string{
		"2026-01期初",
		// GL 区块：科目顺序列出的排前（银行存款→管理费用），未列出的按名排序
		"总分类账-银行存款-工商银行",
		"总分类账-管理费用-办公费",
		"总分类账-库存现金",
		"总分类账-应付款-张三",
		// 合并区块居中
		"总分类账-固定资产",
		// ML 区块
		"多科目明细账-管理费用",
		"现金日记账",
		"2026-01期末",
	}
	if len(got) != len(want) {
		t.Fatalf("Sheet 数 = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("位置 %d = %q, want %q（全序: %v）", i, got[i], want[i], got)
		}
	}
}

func TestReorderSubjectSheetsEmptyOrderDeterministic(t *testing.T) {
	// 空 科目顺序：仍按区块分区 + 名称排序（消除 map 随机遍历的不确定性）
	wb := newOrderTestWorkbook(t, []string{
		"2026-01期初",
		"多科目明细账-管理费用",
		"总分类账-库存现金",
		"多科目明细账-应付款",
		"总分类账-银行存款-工商银行",
		"现金日记账",
		"2026-01期末",
	}, nil, nil)

	wb.reorderSubjectSheets()

	got := sheetList(t, wb)
	// 空配置：区块分区 + Unicode 码点排序（库 U+5E93 < 银 U+94F6；应 U+5E94 < 管 U+7BA1）
	want := []string{
		"2026-01期初",
		"总分类账-库存现金",
		"总分类账-银行存款-工商银行",
		"多科目明细账-应付款",
		"多科目明细账-管理费用",
		"现金日记账",
		"2026-01期末",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("位置 %d = %q, want %q（全序: %v）", i, got[i], want[i], got)
		}
	}
}

func TestReorderSubjectSheetsNoFixedTail(t *testing.T) {
	// 无尾部固定账页（异常输入）时不得 panic / 不得误动
	wb := newOrderTestWorkbook(t, []string{
		"2026-01期初",
		"总分类账-库存现金",
	}, []string{"库存现金"}, nil)
	wb.reorderSubjectSheets()
	if got := sheetList(t, wb); len(got) != 2 {
		t.Errorf("Sheet 数 = %d, want 2: %v", len(got), got)
	}
}
