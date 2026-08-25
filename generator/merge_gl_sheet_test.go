package generator

import (
	"testing"
	"strings"

	"ledger/balance"
	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

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
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())
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

	// Layout: margin(R1空), title(R2), account(R3), blank(R4), year-header(R5), sub-header(R6), data(R7+)
	if len(rows) < 8 {
		t.Fatalf("expected at least 8 rows, got %d", len(rows))
	}
	// Title at GetRows[1], col C (FrontStartCol=3 → GetRows index 2)
	if len(rows[1]) < 3 || !strings.Contains(rows[1][2], "总  分  类  账") {
		t.Errorf("row 1 title = %v", rows[1])
	}
	// Top header at GetRows[4]: 摘要 at col F (FrontStartCol+3=6, GetRows index 5)
	if len(rows[4]) < 7 || !strings.HasPrefix(rows[4][6], "摘") {
		t.Errorf("row 4 headers col 7 = %q, want 摘要", getRowCol(rows, 4, 6))
	}
	// Data row 1 at GetRows[6]: month=01, summary=[电脑]购电脑
	if got := getRowCol(rows, 6, lay.BindingLeftCols+4); got != "[电脑] 购电脑" {
		t.Errorf("data row 1 summary = %q, want [电脑] 购电脑", got)
	}
	if got := getRowCol(rows, 6, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("data row 1 month = %q, want 01", got)
	}
	// Data row 2 at GetRows[7]: summary=[打印机]购打印机
	if got := getRowCol(rows, 7, lay.BindingLeftCols+4); got != "[打印机] 购打印机" {
		t.Errorf("data row 2 summary = %q, want [打印机] 购打印机", got)
	}
	// Money columns
	if got := getRowCol(rows, 6, lay.BindingLeftCols+glColDebit); got == "" || got == "0" {
		t.Errorf("data row 1 debit empty, got %q", got)
	}
	if got := getRowCol(rows, 7, lay.BindingLeftCols+glColDebit); got == "" || got == "0" {
		t.Errorf("data row 2 debit empty, got %q", got)
	}
	if got := getRowCol(rows, 6, lay.BindingLeftCols+glColBalance); got == "" || got == "0" {
		t.Errorf("data row 1 balance empty, got %q", got)
	}
	if got := getRowCol(rows, 7, lay.BindingLeftCols+glColBalance); got == "" || got == "0" {
		t.Errorf("data row 2 balance empty, got %q", got)
	}
}

func TestAppendMergeEntries_SummaryFormat(t *testing.T) {
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())
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
	// Data row 1 at GetRows[6]: no detail prefix
	if got := getRowCol(rows, 6, lay.BindingLeftCols+4); got != "购买设备" {
		t.Errorf("data row 1 summary = %q, want 购买设备", got)
	}
	// Data row 2 at GetRows[7]: with detail prefix
	if got := getRowCol(rows, 7, lay.BindingLeftCols+4); got != "[办公费] 购买设备" {
		t.Errorf("data row 2 summary = %q, want [办公费] 购买设备", got)
	}
}

func TestAppendMergeEntries_MultipleDetails(t *testing.T) {
	lay := layout.GLComputeLayout(layout.DefaultGLSpec())
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
	if len(rows) < 9 {
		t.Fatalf("expected at least 9 rows, got %d", len(rows))
	}
	// Data row 1 at GetRows[6]
	if got := getRowCol(rows, 6, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("data row 1 month = %q, want 01", got)
	}
	// Data row 2 at GetRows[7]
	if got := getRowCol(rows, 7, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("data row 2 month = %q, want 01", got)
	}
}

func getRowCol(rows [][]string, row, col int) string {
	if row < len(rows) && col < len(rows[row]) {
		return rows[row][col]
	}
	return ""
}
