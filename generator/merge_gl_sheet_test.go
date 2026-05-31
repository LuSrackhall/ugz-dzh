package generator

import (
	"testing"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// newTestWB creates a Workbook for testing with the given settings.
func newTestWB(settings balance.GlobalSettings) *Workbook {
	f := excelize.NewFile()
	cfg := &balance.GlobalConfig{Settings: settings}
	moneyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("#,##0.00"),
	})
	return &Workbook{File: f, Config: cfg, Month: "2026-01", moneyStyleID: moneyStyle}
}

func TestAppendMergeEntries_NoConfig(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购电脑", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 10000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries with empty config should not error: %v", err)
	}

	if idx, err := wb.File.GetSheetIndex("总分类账-固定资产"); err == nil && idx >= 0 {
		t.Error("no merge GL sheet should be created when MergeGLAccounts is empty")
	}
}

func TestAppendMergeEntries_Basic(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"固定资产"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购电脑", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 10000},
		{Date: "2026-01-10", VoucherNum: 2, Summary: "购打印机", GeneralAccount: "固定资产", DetailAccount: "打印机", DebitCents: 5000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	sheet := "总分类账-固定资产"
	if idx, err := wb.File.GetSheetIndex(sheet); err != nil || idx < 0 {
		t.Fatalf("sheet %q should exist", sheet)
	}

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}

	// Row 1: title, Row 2: headers, Rows 3-4: data
	if len(rows) < 4 {
		t.Fatalf("expected at least 4 rows, got %d", len(rows))
	}

	// Row 1: title
	if len(rows[0]) == 0 || rows[0][0] != "总分类账 — 固定资产" {
		t.Errorf("row 1 title = %v, want %q", rows[0], "总分类账 — 固定资产")
	}

	// Row 2: headers
	if len(rows[1]) < 7 || rows[1][2] != "摘要" {
		t.Errorf("row 2 headers: col 3 = %q, want %q", getRowCol(rows, 1, 2), "摘要")
	}

	// Row 3 (index 2): first data row — [电脑] 购电脑
	if got := getRowCol(rows, 2, 2); got != "[电脑] 购电脑" {
		t.Errorf("row 3 summary = %q, want %q", got, "[电脑] 购电脑")
	}
	if got := getRowCol(rows, 2, 0); got != "2026-01-05" {
		t.Errorf("row 3 date = %q, want %q", got, "2026-01-05")
	}

	// Row 4 (index 3): second data row — [打印机] 购打印机
	if got := getRowCol(rows, 3, 2); got != "[打印机] 购打印机" {
		t.Errorf("row 4 summary = %q, want %q", got, "[打印机] 购打印机")
	}

	// Money columns: debit (col 4, index 3) should have values
	if got := getRowCol(rows, 2, 3); got == "" || got == "0" {
		t.Errorf("row 3 debit should have value, got %q", got)
	}
	if got := getRowCol(rows, 3, 3); got == "" || got == "0" {
		t.Errorf("row 4 debit should have value, got %q", got)
	}

	// Balance column (col 7, index 6) should have values
	if got := getRowCol(rows, 2, 6); got == "" || got == "0" {
		t.Errorf("row 3 balance should have value, got %q", got)
	}
	if got := getRowCol(rows, 3, 6); got == "" || got == "0" {
		t.Errorf("row 4 balance should have value, got %q", got)
	}
}

func TestAppendMergeEntries_SummaryFormat(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"固定资产"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购买设备", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 8000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	rows, _ := wb.File.GetRows("总分类账-固定资产")
	if len(rows) < 3 {
		t.Fatal("expected at least 3 rows")
	}

	// Summary format: [子科目] 原摘要
	summary := getRowCol(rows, 2, 2)
	want := "[电脑] 购买设备"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestAppendMergeEntries_NoDetail(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"固定资产"},
	})

	// Entry with empty DetailAccount
	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "直接购入", GeneralAccount: "固定资产", DetailAccount: "", DebitCents: 12000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	rows, _ := wb.File.GetRows("总分类账-固定资产")
	if len(rows) < 3 {
		t.Fatal("expected at least 3 rows")
	}

	// Summary should be plain, no prefix
	summary := getRowCol(rows, 2, 2)
	if summary != "直接购入" {
		t.Errorf("summary = %q, want %q (no detail prefix)", summary, "直接购入")
	}
}

func TestAppendMergeEntries_MultipleDetails(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"固定资产"},
	})

	// Two entries with different detail accounts
	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购电脑", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 10000},
		{Date: "2026-01-10", VoucherNum: 2, Summary: "购打印机", GeneralAccount: "固定资产", DetailAccount: "打印机", DebitCents: 5000},
		{Date: "2026-01-15", VoucherNum: 3, Summary: "处置电脑", GeneralAccount: "固定资产", DetailAccount: "电脑", CreditCents: 2000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	rows, _ := wb.File.GetRows("总分类账-固定资产")
	if len(rows) < 5 {
		t.Fatalf("expected at least 5 rows (title + header + 3 data), got %d", len(rows))
	}

	// Row 1: entry 1 → balance = 10000 (debit 10000)
	dir1 := getRowCol(rows, 2, 5) // col 6 (direction)
	if dir1 != "借" {
		t.Errorf("row 3 direction = %q, want %q", dir1, "借")
	}

	// Row 2: entry 2 → balance = 10000 + 5000 = 15000
	dir2 := getRowCol(rows, 3, 5)
	if dir2 != "借" {
		t.Errorf("row 4 direction = %q, want %q", dir2, "借")
	}

	// Row 3: entry 3 → balance = 15000 - 2000 = 13000
	dir3 := getRowCol(rows, 4, 5)
	if dir3 != "借" {
		t.Errorf("row 5 direction = %q, want %q", dir3, "借")
	}

	// Verify debit and credit columns for entry 3
	credit3 := getRowCol(rows, 4, 4) // col 5 (credit)
	if credit3 == "" || credit3 == "0" {
		t.Errorf("row 5 credit should have value, got %q", credit3)
	}

	// Balance should accumulate across entries
	bal1 := getRowCol(rows, 2, 6) // col 7
	bal2 := getRowCol(rows, 3, 6)
	bal3 := getRowCol(rows, 4, 6)
	if bal1 == "" || bal2 == "" || bal3 == "" {
		t.Error("balance columns should have values")
	}
	// The third balance should be less than the second (due to credit reducing it)
	// We test that they are all non-empty and different (balance changes)
	if bal1 == bal2 && bal2 == bal3 {
		t.Error("balances should change across entries")
	}
}

func TestAppendEntries_GLSuppress(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		GLSuppressAccounts: []string{"管理费用"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "办公用品", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 500},
		{Date: "2026-01-10", VoucherNum: 2, Summary: "购设备", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 10000},
	}

	err := wb.AppendEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendEntries: %v", err)
	}

	// 管理费用-办公费 should NOT have a GL sheet (suppressed)
	if idx, err := wb.File.GetSheetIndex("总分类账-管理费用-办公费"); err == nil && idx >= 0 {
		t.Error("总分类账-管理费用-办公费 should not exist (GL suppressed)")
	}

	// 固定资产-电脑 SHOULD have a GL sheet (not suppressed)
	if idx, err := wb.File.GetSheetIndex("总分类账-固定资产-电脑"); err != nil || idx < 0 {
		t.Error("总分类账-固定资产-电脑 should exist (not suppressed)")
	}
}

func TestAppendMLEntries_MLSuppress(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MLSuppressAccounts: []string{"管理费用"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "办公用品", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 500},
	}

	err := wb.AppendMLEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMLEntries: %v", err)
	}

	// 管理费用 should NOT have an ML sheet (suppressed)
	if idx, err := wb.File.GetSheetIndex("多科目明细账-管理费用"); err == nil && idx >= 0 {
		t.Error("多科目明细账-管理费用 should not exist (ML suppressed)")
	}
}

// getRowCol safely retrieves a cell value from rows, returning empty string if out of bounds.
func getRowCol(rows [][]string, row, col int) string {
	if row >= len(rows) {
		return ""
	}
	if col >= len(rows[row]) {
		return ""
	}
	return rows[row][col]
}
