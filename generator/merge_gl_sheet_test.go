package generator

import (
	"testing"
	"strings"

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

	// Layout: Row 1=title, Row 2=blank, Row 3=headers, Row 4=data start, Row 5=more data
	if len(rows) < 7 {
		t.Fatalf("expected at least 7 rows, got %d", len(rows))
	}

	// Row 1 (index 0): title
	if len(rows[1]) == 0 || !strings.Contains(rows[1][2], "总    分    类    账") {
		t.Errorf("row 1 title = %v, want %q", rows[1], "总    分    类    账")
	}

	// Row 3 (index 2): headers
	if len(rows[4]) < 7 || rows[4][4] != "摘要" {
		t.Errorf("row 3 headers: col 3 = %q, want %q", getRowCol(rows, 4, 2), "摘要")
	}

	// Row 6 (index 5): first data row — [电脑] 购电脑
	if got := getRowCol(rows, 5, 2); got != "[电脑] 购电脑" {
		t.Errorf("row 4 summary = %q, want %q", got, "[电脑] 购电脑")
	}
	if got := getRowCol(rows, 5, 0); got != "2026-01-05" {
		t.Errorf("row 4 date = %q, want %q", got, "2026-01-05")
	}

	// Row 7 (index 6): second data row — [打印机] 购打印机
	if got := getRowCol(rows, 6, 2); got != "[打印机] 购打印机" {
		t.Errorf("row 5 summary = %q, want %q", got, "[打印机] 购打印机")
	}

	// Money columns: debit (col 4, index 3) should have values
	if got := getRowCol(rows, 5, 3); got == "" || got == "0" {
		t.Errorf("row 4 debit should have value, got %q", got)
	}
	if got := getRowCol(rows, 6, 3); got == "" || got == "0" {
		t.Errorf("row 5 debit should have value, got %q", got)
	}

	// Balance column (col 7, index 6) should have values
	if got := getRowCol(rows, 5, 6); got == "" || got == "0" {
		t.Errorf("row 4 balance should have value, got %q", got)
	}
	if got := getRowCol(rows, 6, 6); got == "" || got == "0" {
		t.Errorf("row 5 balance should have value, got %q", got)
	}
}

func TestAppendMergeEntries_SummaryFormat(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"库存现金"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购买设备", GeneralAccount: "库存现金", DebitCents: 50000},
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购买设备", GeneralAccount: "库存现金", DetailAccount: "办公费", DebitCents: 50000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	sheet := "总分类账-库存现金"
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}

	// Row 6 (index 5): first data row — no detail prefix
	if got := getRowCol(rows, 5, 2); got != "购买设备" {
		t.Errorf("row 4 summary = %q, want %q", got, "[办公费] 购买设备")
	}

	// Row 7 (index 6): second data row — with detail prefix
	if got := getRowCol(rows, 6, 2); got != "[办公费] 购买设备" {
		t.Errorf("row 5 summary = %q, want %q", got, "[办公费] 购买设备")
	}
}

func TestAppendMergeEntries_MultipleDetails(t *testing.T) {
	wb := newTestWB(balance.GlobalSettings{
		MergeGLAccounts: []string{"固定资产"},
	})

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "购电脑", GeneralAccount: "固定资产", DetailAccount: "电脑", DebitCents: 10000},
		{Date: "2026-01-10", VoucherNum: 2, Summary: "购打印机", GeneralAccount: "固定资产", DetailAccount: "打印机", DebitCents: 5000},
		{Date: "2026-01-15", VoucherNum: 3, Summary: "购桌椅", GeneralAccount: "固定资产", DetailAccount: "桌椅", DebitCents: 3000},
	}

	err := wb.AppendMergeEntries(entries, nil)
	if err != nil {
		t.Fatalf("AppendMergeEntries: %v", err)
	}

	sheet := "总分类账-固定资产"
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}

	if len(rows) < 7 {
		t.Fatalf("expected at least 7 rows, got %d", len(rows))
	}

	// Row 4 (index 3): first data — date column
	if got := getRowCol(rows, 5, 0); got != "2026-01-05" {
		t.Errorf("row 4 date = %q, want %q", got, "2026-01-05")
	}

	// Row 5 (index 4): second data
	if got := getRowCol(rows, 6, 0); got != "2026-01-10" {
		t.Errorf("row 5 date = %q, want %q", got, "2026-01-10")
	}
}

// getRowCol safely retrieves a cell value from rows, returning empty string if out of bounds.
func getRowCol(rows [][]string, row, col int) string {
	if row < len(rows) && col < len(rows[row]) {
		return rows[row][col]
	}
	return ""
}
