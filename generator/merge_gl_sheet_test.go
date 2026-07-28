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

	// Layout: title(R1), account(R2), blank(R3), year-header(R4), sub-header(R5:月|日), data(R6+)
	if len(rows) < 7 {
		t.Fatalf("expected at least 7 rows, got %d", len(rows))
	}
	// Title at GetRows[0], col C
	if len(rows[0]) < 3 || !strings.Contains(rows[0][2], "总    分    类    账") {
		t.Errorf("row 0 title = %v", rows[0])
	}
	// Top header at GetRows[3]: 摘要 at col F (FrontStartCol+3=col6, GetRows index 5)
	if len(rows[6]) < 7 || rows[6][6] != "摘要" {
		t.Errorf("row 3 headers col 7 = %q, want 摘要", getRowCol(rows, 3, 6))
	}
	// Data row 1 at GetRows[5]: month=01, summary=[电脑]购电脑
	if got := getRowCol(rows, 8, lay.BindingLeftCols+4); got != "[电脑] 购电脑" {
		t.Errorf("row 8 summary = %q, want [电脑] 购电脑", got)
	}
	if got := getRowCol(rows, 8, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("row 8 month = %q, want 01", got)
	}
	// Data row 2 at GetRows[6]: summary=[打印机]购打印机
	if got := getRowCol(rows, 9, lay.BindingLeftCols+4); got != "[打印机] 购打印机" {
		t.Errorf("row 9 summary = %q, want [打印机] 购打印机", got)
	}
	// Money columns
	if got := getRowCol(rows, 8, lay.BindingLeftCols+glColDebit); got == "" || got == "0" {
		t.Errorf("row 5 debit empty, got %q", got)
	}
	if got := getRowCol(rows, 9, lay.BindingLeftCols+glColDebit); got == "" || got == "0" {
		t.Errorf("row 6 debit empty, got %q", got)
	}
	if got := getRowCol(rows, 8, lay.BindingLeftCols+glColBalance); got == "" || got == "0" {
		t.Errorf("row 5 balance empty, got %q", got)
	}
	if got := getRowCol(rows, 9, lay.BindingLeftCols+glColBalance); got == "" || got == "0" {
		t.Errorf("row 6 balance empty, got %q", got)
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
	// Data row 1: no detail prefix
	if got := getRowCol(rows, 8, lay.BindingLeftCols+4); got != "购买设备" {
		t.Errorf("row 8 summary = %q, want 购买设备", got)
	}
	// Data row 2: with detail prefix
	if got := getRowCol(rows, 9, lay.BindingLeftCols+4); got != "[办公费] 购买设备" {
		t.Errorf("row 9 summary = %q, want [办公费] 购买设备", got)
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
	if len(rows) < 11 {
		t.Fatalf("expected at least 11 rows, got %d", len(rows))
	}
	// Data row 1 month
	if got := getRowCol(rows, 8, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("row 8 month = %q, want 01", got)
	}
	// Data row 2 month
	if got := getRowCol(rows, 9, lay.BindingLeftCols+0); got != "01" {
		t.Errorf("row 9 month = %q, want 01", got)
	}
}

func getRowCol(rows [][]string, row, col int) string {
	if row < len(rows) && col < len(rows[row]) {
		return rows[row][col]
	}
	return ""
}
