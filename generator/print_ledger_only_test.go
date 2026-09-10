package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// ledgerOnly 开关（print-config.json 顶层）：打印版仅含账页——
// 移除日记账/报表/期初期末表等非账页 Sheet，导出 PDF 即纯账页成册。

func TestRemoveNonLedgerSheets(t *testing.T) {
	f := excelize.NewFile()
	sheets := []string{
		"2026-01期初",
		"总分类账-管理费用-办公费",
		"明细账-公益支出-补助费用", // 分离明细账页（ML 家族）
		"多科目明细账-管理费用",
		"银行存款日记账",
		"资产负债表",
		"2026-01期末",
	}
	for _, s := range sheets {
		if _, err := f.NewSheet(s); err != nil {
			t.Fatalf("创建 Sheet: %v", err)
		}
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("删默认页: %v", err)
	}

	removed := removeNonLedgerSheets(f)
	if removed != 4 {
		t.Errorf("removed = %d, want 4（日记账/报表/期初/期末）", removed)
	}
	got := f.GetSheetList()
	want := []string{"总分类账-管理费用-办公费", "明细账-公益支出-补助费用", "多科目明细账-管理费用"}
	if len(got) != len(want) {
		t.Fatalf("剩余 Sheet = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("位置 %d = %q, want %q（全序: %v）", i, got[i], want[i], got)
		}
	}
	if f.GetActiveSheetIndex() < 0 || got[f.GetActiveSheetIndex()] != want[0] {
		t.Errorf("激活页应为首张账页, got idx=%d", f.GetActiveSheetIndex())
	}
}

// TestTransformToPrintLedgerOnly 双产物语义：
//   - TransformToPrint（print/ 完整版）**不删任何 sheet**（默认零影响）；
//   - TransformToPrintLedgerOnly（pdf/ 纯账页版）移除非账页、保留三类账页。
func TestTransformToPrintLedgerOnly(t *testing.T) {
	dir := t.TempDir()
	viewPath := filepath.Join(dir, "2026-01.xlsx")
	sheets := []string{
		"2026-01期初",
		"总分类账-管理费用-办公费",
		"明细账-管理费用-办公费",
		"多科目明细账-管理费用",
		"资产负债表",
		"2026-01期末",
	}
	f := excelize.NewFile()
	for _, sh := range sheets {
		if _, err := f.NewSheet(sh); err != nil {
			t.Fatalf("创建: %v", err)
		}
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("删默认页: %v", err)
	}
	if err := f.SaveAs(viewPath); err != nil {
		t.Fatalf("保存查看版: %v", err)
	}
	f.Close()

	// 完整打印版：全部 sheet 保留
	fullPath := filepath.Join(dir, "print", "2026-01.xlsx")
	if err := TransformToPrint(viewPath, fullPath); err != nil {
		t.Fatalf("TransformToPrint: %v", err)
	}
	ff, err := excelize.OpenFile(fullPath)
	if err != nil {
		t.Fatalf("打开完整版: %v", err)
	}
	if got := len(ff.GetSheetList()); got != len(sheets) {
		t.Errorf("完整打印版应含全部 %d 张 sheet（默认零影响），得到 %d: %v", len(sheets), got, ff.GetSheetList())
	}
	ff.Close()

	// 纯账页版：非账页移除、三类账页保留
	pdfPath := filepath.Join(dir, "pdf", "2026-01.xlsx")
	removed, err := TransformToPrintLedgerOnly(viewPath, pdfPath)
	if err != nil {
		t.Fatalf("TransformToPrintLedgerOnly: %v", err)
	}
	if removed != 3 {
		t.Errorf("removed = %d, want 3（期初/资产负债表/期末）", removed)
	}
	pf, err := excelize.OpenFile(pdfPath)
	if err != nil {
		t.Fatalf("打开纯账页版: %v", err)
	}
	defer pf.Close()
	got := pf.GetSheetList()
	for _, want := range []string{"总分类账-管理费用-办公费", "明细账-管理费用-办公费", "多科目明细账-管理费用"} {
		found := false
		for _, sh := range got {
			if sh == want {
				found = true
			}
		}
		if !found {
			t.Errorf("纯账页版缺账页 %s（got %v）", want, got)
		}
	}
	for _, sh := range got {
		if strings.HasPrefix(sh, "2026-01") || sh == "资产负债表" {
			t.Errorf("非账页 %s 应被移除（got %v）", sh, got)
		}
	}
}
