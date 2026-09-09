package generator

import (
	"os"
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

func TestLoadPrintConfigLedgerOnly(t *testing.T) {
	defer func() { printCfg.LedgerOnly = false }() // 恢复全局状态（printCfg 为包级单例）

	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	if err := os.WriteFile(p, []byte(`{"ledgerOnly": true, "platforms": {}}`), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("LoadPrintConfig: %v", err)
	}
	if !printCfg.LedgerOnly {
		t.Error("ledgerOnly=true 应生效")
	}

	// 缺省（不配置）→ false
	if err := os.WriteFile(p, []byte(`{"platforms": {}}`), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("LoadPrintConfig: %v", err)
	}
	if printCfg.LedgerOnly {
		t.Error("未配置 ledgerOnly 应为 false（打印版含全部 sheet，现状不变）")
	}
}

func TestPrintConfigTemplateHasLedgerOnly(t *testing.T) {
	if !strings.Contains(PrintConfigTemplate(), `"ledgerOnly"`) {
		t.Error("配置模板应包含 ledgerOnly 字段（显式存在原则）")
	}
}

// TestTransformToPrintLedgerOnlyKeepsSplitPages 全链复现：ledgerOnly 下分离明细账页
// 必须保留（曾实测丢失——定位 removeNonLedgerSheets 与 TransformToPrint 的边界）。
func TestTransformToPrintLedgerOnlyKeepsSplitPages(t *testing.T) {
	defer func() { printCfg.LedgerOnly = false }()
	printCfg.LedgerOnly = true

	dir := t.TempDir()
	viewPath := filepath.Join(dir, "2026-01.xlsx")
	f := excelize.NewFile()
	for _, s := range []string{
		"2026-01期初",
		"总分类账-管理费用-办公费",
		"明细账-管理费用-办公费",
		"多科目明细账-管理费用",
		"资产负债表",
		"2026-01期末",
	} {
		if _, err := f.NewSheet(s); err != nil {
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

	printPath := filepath.Join(dir, "print", "2026-01.xlsx")
	if err := TransformToPrint(viewPath, printPath); err != nil {
		t.Fatalf("TransformToPrint: %v", err)
	}

	pf, err := excelize.OpenFile(printPath)
	if err != nil {
		t.Fatalf("打开打印版: %v", err)
	}
	defer pf.Close()
	got := pf.GetSheetList()
	for _, want := range []string{"总分类账-管理费用-办公费", "明细账-管理费用-办公费", "多科目明细账-管理费用"} {
		found := false
		for _, s := range got {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("打印版缺账页 %s（got %v）", want, got)
		}
	}
	for _, s := range got {
		if strings.HasPrefix(s, "2026-01") || s == "资产负债表" {
			t.Errorf("非账页 %s 应被移除（got %v）", s, got)
		}
	}
}
