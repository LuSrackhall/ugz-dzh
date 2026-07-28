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

	// Paper1 Front 占位页（rows 1-27）— 保证 GetRows 列对齐
	fdp := mlFirstDataPageStart()
	for col := 1; col <= lay.TotalCols; col++ {
		for r := 1; r <= fdp-1; r++ {
			cell, _ := excelize.CoordinatesToCellName(col, r)
			wb.File.SetCellValue(sheet, cell, "")
		}
	}
	// 占位页底部结构过次页
	padCell := mlCellName(lay.BackStartCol+mlOffSummary, fdp-1)
	wb.File.SetCellValue(sheet, padCell, pageBreakLabel)
	redS, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "CC0000", Size: 10, Bold: true},
	})
	wb.File.SetCellStyle(sheet, padCell, padCell, redS)

	// 明细列标题（数据页列标题行 = Row+4 of first data page header）
	colHeaderRow := fdp + 4 // = 32
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 0), colHeaderRow), "工行")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 1), colHeaderRow), "建行")

	// 数据行 — 使用 Layout 坐标（承前页在第34行，第一条分录在第35行）
	// 列映射: +0=月, +1=日, +2=字, +3=号, +4=摘要, +5=借方, +6=贷方, +7=方向, +8=余额
	cfRow := fdp + lay.DataStartRow // = 34
	dr1 := cfRow + 1                // = 35
	dr2 := cfRow + 2                // = 36
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, dr1), "03")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, dr1), "05")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, dr1), "记")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, dr1), 1)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, dr1), "存入")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, dr1), "1000.00")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 0), dr1), "1000.00")

	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, dr2), "03")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, dr2), "10")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, dr2), "记")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, dr2), 2)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, dr2), "支出")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, dr2), "500.00")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, dr2), "200.00")
	wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 1), dr2), "-300.00")

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

	backIdx := lay.BackStartCol - 1
	sumIdx := backIdx + mlOffSummary
	var qtRow, ytdRow []string
	for _, r := range rows {
		if len(r) > sumIdx {
			switch r[sumIdx] {
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

	// 月结行现在写入 Back 侧（索引 backIdx+mlOffDebit=借方, backIdx+mlOffCredit=贷方）
	if len(qtRow) > backIdx+mlOffCredit && qtRow[backIdx+mlOffDebit] != "4000" {
		t.Errorf("本季合计 debit = %q, want %q (当月+本季累计全路径聚合)", qtRow[backIdx+mlOffDebit], "4000")
	}
	if len(qtRow) > backIdx+mlOffCredit && qtRow[backIdx+mlOffCredit] != "1000" {
		t.Errorf("本季合计 credit = %q, want %q (当月+本季累计全路径聚合)", qtRow[backIdx+mlOffCredit], "1000")
	}

	if len(ytdRow) > backIdx+mlOffCredit && ytdRow[backIdx+mlOffDebit] != "6500" {
		t.Errorf("本年累计 debit = %q, want %q (当月+本年累计全路径聚合)", ytdRow[backIdx+mlOffDebit], "6500")
	}
	if len(ytdRow) > backIdx+mlOffCredit && ytdRow[backIdx+mlOffCredit] != "1700" {
		t.Errorf("本年累计 credit = %q, want %q (当月+本年累计全路径聚合)", ytdRow[backIdx+mlOffCredit], "1700")
	}
}
