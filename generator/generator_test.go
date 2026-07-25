package generator

import (
	"testing"

	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

func TestSheetNameGL(t *testing.T) {
	tests := []struct {
		account string
		want    string
	}{
		{"库存现金", "总分类账-库存现金"},
		{"管理费用-办公费", "总分类账-管理费用-办公费"},
		{"银行存款-工商银行", "总分类账-银行存款-工商银行"},
	}
	for _, tt := range tests {
		got := sheetNameGL(tt.account)
		if got != tt.want {
			t.Errorf("sheetNameGL(%q) = %q, want %q", tt.account, got, tt.want)
		}
	}
}

func TestSheetNameML(t *testing.T) {
	tests := []struct {
		general string
		want    string
	}{
		{"管理费用", "多科目明细账-管理费用"},
		{"应收款", "多科目明细账-应收款"},
	}
	for _, tt := range tests {
		got := sheetNameML(tt.general)
		if got != tt.want {
			t.Errorf("sheetNameML(%q) = %q, want %q", tt.general, got, tt.want)
		}
	}
}

func TestCentsToYuan(t *testing.T) {
	tests := []struct {
		cents int64
		want  float64
	}{
		{0, 0.0},
		{100, 1.00},
		{12345, 123.45},
		{-500, -5.00},
		{-1, -0.01},
		{99, 0.99},
	}
	for _, tt := range tests {
		got := centsToYuan(tt.cents)
		if got != tt.want {
			t.Errorf("centsToYuan(%d) = %v, want %v", tt.cents, got, tt.want)
		}
	}
}

func TestYuanStrToCents(t *testing.T) {
	tests := []struct {
		s    string
		want int64
	}{
		{"0", 0},
		{"1.00", 100},
		{"123.45", 12345},
		{"-5.00", -500},
		{"0.01", 1},
		{"0.99", 99},
		{"", 0},
		{"100", 10000},
	}
	for _, tt := range tests {
		got, err := yuanStrToCents(tt.s)
		if err != nil {
			t.Errorf("yuanStrToCents(%q) unexpected error: %v", tt.s, err)
			continue
		}
		if got != tt.want {
			t.Errorf("yuanStrToCents(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestDirectionFor(t *testing.T) {
	tests := []struct {
		debit, credit int64
		wantDir       string
		wantBal       int64
	}{
		{1000, 0, "借", 1000},
		{0, 500, "贷", 500},
		{100, 100, "平", 0},
		{200, 100, "借", 100},
		{100, 200, "贷", 100},
	}
	for _, tt := range tests {
		gotDir, gotBal := directionFor(tt.debit, tt.credit)
		if gotDir != tt.wantDir || gotBal != tt.wantBal {
			t.Errorf("directionFor(%d, %d) = (%q, %d), want (%q, %d)",
				tt.debit, tt.credit, gotDir, gotBal, tt.wantDir, tt.wantBal)
		}
	}
}

func TestPrevMonth(t *testing.T) {
	tests := []struct {
		month string
		want  string
	}{
		{"2026-01", "2025-12"},
		{"2026-02", "2026-01"},
		{"2026-12", "2026-11"},
		{"2025-01", "2024-12"},
	}
	for _, tt := range tests {
		got := prevMonth(tt.month)
		if got != tt.want {
			t.Errorf("prevMonth(%q) = %q, want %q", tt.month, got, tt.want)
		}
	}
}

func TestEntryMonth(t *testing.T) {
	e := voucher.Entry{Date: "2026-03-15", Summary: "摘要", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 100}
	got := entryMonth(e)
	if got != "2026-03" {
		t.Errorf("entryMonth = %q, want %q", got, "2026-03")
	}

	e2 := voucher.Entry{Date: "", Summary: "摘要", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 100}
	if got := entryMonth(e2); got != "" {
		t.Errorf("entryMonth empty date = %q, want empty", got)
	}
}

func TestCellName(t *testing.T) {
	tests := []struct {
		col, row int
		want     string
	}{
		{1, 1, "A1"},
		{7, 3, "G3"},
		{26, 1, "Z1"},
		{27, 2, "AA2"},
	}
	for _, tt := range tests {
		got := cellName(tt.col, tt.row)
		if got != tt.want {
			t.Errorf("cellName(%d, %d) = %q, want %q", tt.col, tt.row, got, tt.want)
		}
	}
}

func TestGetMLGenerals(t *testing.T) {
	entries := []voucher.Entry{
		{Date: "2026-01-05", Summary: "购办公用品", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 500},
		{Date: "2026-01-10", Summary: "差旅报销", GeneralAccount: "管理费用", DetailAccount: "差旅费", CreditCents: 300},
		{Date: "2026-01-15", Summary: "收现金", GeneralAccount: "库存现金", DebitCents: 1000},
	}
	got := getMLGenerals(entries)
	if !got["管理费用"] {
		t.Error("expected 管理费用 to be in ML generals")
	}
	if got["库存现金"] {
		t.Error("库存现金 should not be in ML generals (no detail)")
	}
}

func TestComputeActivity(t *testing.T) {
	entries := []voucher.Entry{
		{Date: "2026-01-05", Summary: "购办公用品", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 500},
		{Date: "2026-01-05", Summary: "购办公用品", GeneralAccount: "管理费用", DetailAccount: "办公费", CreditCents: 200},
		{Date: "2026-01-10", Summary: "收现金", GeneralAccount: "库存现金", DebitCents: 1000},
	}
	act := ComputeActivity(entries)

	mgmt := act["管理费用-办公费"]
	if mgmt.Debit != 500 || mgmt.Credit != 200 {
		t.Errorf("管理费用-办公费 activity = %+v, want debit=500 credit=200", mgmt)
	}

	cash := act["库存现金"]
	if cash.Debit != 1000 || cash.Credit != 0 {
		t.Errorf("库存现金 activity = %+v, want debit=1000 credit=0", cash)
	}
}

func TestCollectChangedSheets(t *testing.T) {
	entries := []voucher.Entry{
		{Date: "2026-01-05", Summary: "test", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 100},
		{Date: "2026-01-10", Summary: "test", GeneralAccount: "库存现金", CreditCents: 100},
	}
	sheets := CollectChangedSheets(entries)
	if !sheets["总分类账-管理费用-办公费"] {
		t.Error("expected 总分类账-管理费用-办公费 to be changed")
	}
	if !sheets["总分类账-库存现金"] {
		t.Error("expected 总分类账-库存现金 to be changed")
	}
}

func TestMLDetailStartCol(t *testing.T) {
	if mlDetailStartCol != 8 {
		t.Errorf("mlDetailStartCol = %d, want 8 (H column)", mlDetailStartCol)
	}
}

func TestMLPrintMarkCol(t *testing.T) {
	got := mlPrintMarkCol()
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())
	want := lay.FrontStartCol + 7 + 14 // 24
	if got != want {
		t.Errorf("mlPrintMarkCol() = %d, want %d", got, want)
	}
}

// TestGLColumnLayoutConsistency 验证写入使用 Layout 坐标后，GetRows 能从正确索引位置读回数据。
// 这是保护列映射正确性的核心 TDD 测试，拦截任何因索引偏移导致的数据错乱。
func TestGLColumnLayoutConsistency(t *testing.T) {
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())
	f := excelize.NewFile()
	sheet := "Sheet1"

	// 用 Layout 坐标写入一行完整数据
	row := 1
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+0, row), "2026-01-05")
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+1, row), "1")
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+2, row), "测试摘要")
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+3, row), 100.00)
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+4, row), 50.00)
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+5, row), "借")
	f.SetCellValue(sheet, cellName(lay.FrontStartCol+6, row), 50.00)

	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) < 1 {
		t.Fatal("no rows returned")
	}
	r := rows[0]

	// 验证各列在正确的 index 位置
	bIdx := lay.BindingLeftCols
	if len(r) <= bIdx+6 {
		t.Fatalf("row too short (len=%d, need at least %d)", len(r), bIdx+7)
	}
	if r[bIdx+0] != "2026-01-05" {
		t.Errorf("date at index %d: got %q, want %q", bIdx+0, r[bIdx+0], "2026-01-05")
	}
	if r[bIdx+2] != "测试摘要" {
		t.Errorf("summary at index %d: got %q, want %q", bIdx+2, r[bIdx+2], "测试摘要")
	}
	if r[bIdx+5] != "借" {
		t.Errorf("direction at index %d: got %q, want %q", bIdx+5, r[bIdx+5], "借")
	}
}
