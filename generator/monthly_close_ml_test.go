package generator

import (
	"testing"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// TestWriteMLMonthClosings_CumulativeAggregation 验证多科目明细账
// "本季合计"和"本年累计"的 A-G 列使用全路径(key: general-detail)
// 聚合所有明细科目的累计值，而非仅当月发生额。
func TestWriteMLMonthClosings_CumulativeAggregation(t *testing.T) {
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

	// 标题行 — 占位列保证 GetRows 列对齐
	for col := 1; col <= 22; col++ {
		cell, _ := excelize.CoordinatesToCellName(col, 1)
		wb.File.SetCellValue(sheet, cell, "")
		cell, _ = excelize.CoordinatesToCellName(col, 2)
		wb.File.SetCellValue(sheet, cell, "")
	}
	wb.File.SetCellValue(sheet, "H2", "工行")
	wb.File.SetCellValue(sheet, "I2", "建行")

	// 数据行
	wb.File.SetCellValue(sheet, "A3", "2026-03-05")
	wb.File.SetCellValue(sheet, "B3", "记-1")
	wb.File.SetCellValue(sheet, "C3", "存入")
	wb.File.SetCellValue(sheet, "D3", "1000.00")
	wb.File.SetCellValue(sheet, "H3", "1000.00")

	wb.File.SetCellValue(sheet, "A4", "2026-03-10")
	wb.File.SetCellValue(sheet, "B4", "记-2")
	wb.File.SetCellValue(sheet, "C4", "支出")
	wb.File.SetCellValue(sheet, "D4", "500.00")
	wb.File.SetCellValue(sheet, "E4", "200.00")
	wb.File.SetCellValue(sheet, "I4", "-300.00")

	// 当月分录：2 个明细科目各有发生额
	entries := []voucher.Entry{
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 100000, CreditCents: 0},
		{GeneralAccount: "银行存款", DetailAccount: "建行", DebitCents: 50000, CreditCents: 20000},
	}

	initials := map[string]int64{
		"银行存款": 1500000,
	}

	// 截至上月的本年累计（key 为全路径 general-detail）
	ytdDebit := map[string]int64{
		"银行存款-工行": 300000,
		"银行存款-建行": 200000,
	}
	ytdCredit := map[string]int64{
		"银行存款-工行": 100000,
		"银行存款-建行": 50000,
	}

	// 截至上月的本季累计（key 为全路径 general-detail）
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

	var qtRow, ytdRow []string
	for _, r := range rows {
		if len(r) >= 3 {
			switch r[2] {
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

	// 本季合计 D/E 列 = 当月发生额 + 截至上月的本季累计（全路径聚合所有明细）
	// 当月: debit=100000+50000=150000, credit=0+20000=20000
	// qtd 累计: debit=150000+100000=250000, credit=50000+30000=80000
	// 预期: debit=400000(4000.00), credit=100000(1000.00)
	if len(qtRow) > 3 && qtRow[3] != "4000.00" {
		t.Errorf("本季合计 D(debit) = %q, want %q (当月+本季累计全路径聚合)", qtRow[3], "4000.00")
	}
	if len(qtRow) > 4 && qtRow[4] != "1000.00" {
		t.Errorf("本季合计 E(credit) = %q, want %q (当月+本季累计全路径聚合)", qtRow[4], "1000.00")
	}

	// 本年累计 D/E 列 = 当月发生额 + 截至上月的本年累计（全路径聚合所有明细）
	// ytd 累计: debit=300000+200000=500000, credit=100000+50000=150000
	// 预期: debit=650000(6500.00), credit=170000(1700.00)
	if len(ytdRow) > 3 && ytdRow[3] != "6500.00" {
		t.Errorf("本年累计 D(debit) = %q, want %q (当月+本年累计全路径聚合)", ytdRow[3], "6500.00")
	}
	if len(ytdRow) > 4 && ytdRow[4] != "1700.00" {
		t.Errorf("本年累计 E(credit) = %q, want %q (当月+本年累计全路径聚合)", ytdRow[4], "1700.00")
	}
}
