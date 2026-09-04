package generator

import (
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestWriteIncomeStatementUnclassifiedListed v2 §2.2：收支结余表未分类科目
// 必须单列展示（此前对未分类科目静默 return，无痕丢行），且不计入收入/支出合计。
func TestWriteIncomeStatementUnclassifiedListed(t *testing.T) {
	f := excelize.NewFile()
	wb := &Workbook{File: f, Month: "2026-01"}

	activity := map[string]Activity{
		"管埋费用": {Debit: 10000},  // OCR 错名 → 未分类
		"管理费用": {Debit: 20000},  // 费用
		"经营收入": {Credit: 50000}, // 收入
	}
	ytdDebit := map[string]int64{"管理费用": 5000}
	ytdCredit := map[string]int64{"经营收入": 10000}

	if err := wb.writeIncomeStatement(ytdDebit, ytdCredit, activity); err != nil {
		t.Fatalf("writeIncomeStatement: %v", err)
	}

	foundHeader, foundSubject := false, false
	var incomeTotalCell, expenseTotalCell string
	for row := 1; row <= 60; row++ {
		r := strconv.Itoa(row)
		a, _ := f.GetCellValue("收支结余表", "A"+r)
		b, _ := f.GetCellValue("收支结余表", "B"+r)
		c, _ := f.GetCellValue("收支结余表", "C"+r)
		d, _ := f.GetCellValue("收支结余表", "D"+r)
		if a == "未分类科目（类别未知，不计入合计）:" {
			foundHeader = true
		}
		if a == "管埋费用" {
			foundSubject = true
			want := "借 100.00 / 贷 0.00 元"
			if b != want {
				t.Errorf("未分类科目行金额 = %q, want %q", b, want)
			}
		}
		if a == "收入合计" {
			incomeTotalCell = b
		}
		if c == "支出合计" {
			expenseTotalCell = d
		}
	}
	if !foundHeader {
		t.Error("收支结余表缺少未分类科目段标题")
	}
	if !foundSubject {
		t.Error("未分类科目'管埋费用'未被列出")
	}
	if incomeTotalCell != "600" {
		t.Errorf("收入合计 = %q, want 600（未分类不得计入）", incomeTotalCell)
	}
	if expenseTotalCell != "250" {
		t.Errorf("支出合计 = %q, want 250（未分类不得计入）", expenseTotalCell)
	}
}
