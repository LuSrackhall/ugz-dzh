package generator

import (
	"testing"

	"ledger/balance"
	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// TestWriteMLMonthClosings_CumulativeAggregation 验证多科目明细账
// "本季合计"和"本年累计"的 A-G 列使用全路径(key: general-detail)
// 聚合所有明细科目的累计值，而非仅当月发生额。
func TestWriteMLMonthClosings_CumulativeAggregation(t *testing.T) {
	lay := layout.MLComputeLayout(layout.DefaultMLSpec())
	cfg := &balance.GlobalConfig{
		Tree: map[string]balance.AccountNode{
			"银行存款-工行": {
				Balances: map[string]balance.MonthBalance{
					"2026-01": {Debit: 200000, Credit: 50000},
					"2026-02": {Debit: 100000, Credit: 50000},
				},
			},
			"银行存款-建行": {
				Balances: map[string]balance.MonthBalance{
					"2026-01": {Debit: 150000, Credit: 30000},
					"2026-02": {Debit: 50000, Credit: 20000},
				},
			},
		},
	}

	wb := &Workbook{
		File:   excelize.NewFile(),
		Config: cfg,
		Month:  "2026-03",
	}

	sheet := "多科目明细账-银行存款"
	wb.File.NewSheet(sheet)

	// Paper1 Front 占位行 — 保证 GetRows 列对齐
	for col := 1; col <= lay.TotalCols; col++ {
		for r := 1; r <= 5; r++ {
			cell, _ := excelize.CoordinatesToCellName(col, r)
			wb.File.SetCellValue(sheet, cell, "")
		}
	}

	// 明细列标题（数据页列标题行）
	colHeaderRow := 6 + lay.DataStartRow - 1 // = 11
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 0), colHeaderRow), "工行")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 1), colHeaderRow), "建行")

	// 数据行 — 使用 Back 侧坐标
	dataRow := 6 + lay.DataStartRow // = 12
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, dataRow), "2026-03-05")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, dataRow), "记-1")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, dataRow), "存入")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, dataRow), "1000.00")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 0), dataRow), "1000.00")

	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, dataRow+1), "2026-03-10")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, dataRow+1), "记-2")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, dataRow+1), "支出")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, dataRow+1), "500.00")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, dataRow+1), "200.00")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 1), dataRow+1), "-300.00")

	entries := []voucher.Entry{
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 100000, CreditCents: 0},
		{GeneralAccount: "银行存款", DetailAccount: "建行", DebitCents: 50000, CreditCents: 20000},
	}

	initials := map[string]int64{
		"银行存款": 1500000,
	}

	ytdDebit := map[string]int64{
		"银行存款-工行": 300000,
		"银行存款-建行": 200000,
	}
	ytdCredit := map[string]int64{
		"银行存款-工行": 100000,
		"银行存款-建行": 50000,
	}

	qtdDebit := map[string]int64{
		"银行存款-工行": 150000,
		"银行存款-建行": 100000,
	}
	qtdCredit := map[string]int64{
		"银行存款-工行": 50000,
		"银行存款-建行": 30000,
	}

	changedSheets := map[string]bool{sheet: true}

	err := wb.WriteMLMonthClosings(entries, initials, ytdDebit, ytdCredit, qtdDebit, qtdCredit, changedSheets)
	if err != nil {
		t.Fatalf("WriteMLMonthClosings: %v", err)
	}

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}

	bIdx := lay.BindingLeftCols
	var qtRow, ytdRow []string
	for _, r := range rows {
		if len(r) >= bIdx+3 {
			switch r[bIdx+2] {
			case "本季合计":
				qtRow = r
			case "本年累计":
				ytdRow = r
			}
		}
	}
	if qtRow == nil {
		t.Fatal("未找到'本季合计'行")
	}
	if ytdRow == nil {
		t.Fatal("未找到'本年累计'行")
	}

	// 在 Layout 坐标下，借贷金额列在 index bIdx+3 和 bIdx+4
	if len(qtRow) > bIdx+4 && qtRow[bIdx+3] != "4000" {
		t.Errorf("本季合计 D(debit) = %q, want %q (当月+本季累计全路径聚合)", qtRow[bIdx+3], "4000")
	}
	if len(qtRow) > bIdx+4 && qtRow[bIdx+4] != "1000" {
		t.Errorf("本季合计 E(credit) = %q, want %q (当月+本季累计全路径聚合)", qtRow[bIdx+4], "1000")
	}

	if len(ytdRow) > bIdx+4 && ytdRow[bIdx+3] != "6500" {
		t.Errorf("本年累计 D(debit) = %q, want %q (当月+本年累计全路径聚合)", ytdRow[bIdx+3], "6500")
	}
	if len(ytdRow) > bIdx+4 && ytdRow[bIdx+4] != "1700" {
		t.Errorf("本年累计 E(credit) = %q, want %q (当月+本年累计全路径聚合)", ytdRow[bIdx+4], "1700")
	}
}
