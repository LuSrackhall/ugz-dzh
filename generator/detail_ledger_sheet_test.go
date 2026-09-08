package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ledger/balance"
	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// 分离明细账页（全局设置.分离明细账科目）单元测试。
// 页面样式与多科目明细账同族（ML 那张多栏式账页纸，明细列自然为空）；
// 与合并 ML 页互不冲突共存（对仗 GL 叶子页与合并页共存）。

func TestDetailSplitMatch(t *testing.T) {
	wb := &Workbook{Config: &balance.GlobalConfig{}}
	// 条目混合：总账科目名 + 叶子全路径
	wb.Config.Settings.DetailSplit = []string{"公益支出", "管理费用-办公费"}

	// 总账名命中其下所有明细
	if !wb.detailSplit("公益支出-补助费用") {
		t.Error("general entry should split all its details")
	}
	if !wb.detailSplit("公益支出-其他福利") {
		t.Error("general entry should split all its details (second)")
	}
	// 叶子全路径精确命中
	if !wb.detailSplit("管理费用-办公费") {
		t.Error("exact leaf path should match")
	}
	// 未配置总账/未配置明细不命中
	if wb.detailSplit("管理费用-差旅费") {
		t.Error("sibling detail of leaf-path entry must not match")
	}
	if wb.detailSplit("银行存款-工行") {
		t.Error("unrelated account must not match")
	}
	// 总账本体（非叶子路径）不命中——分离页只按叶子建
	if wb.detailSplit("公益支出") {
		t.Error("general account itself is not a leaf, must not match")
	}

	// 空名单短路
	empty := &Workbook{Config: &balance.GlobalConfig{}}
	if empty.detailSplit("公益支出-补助费用") {
		t.Error("empty list must never match")
	}
}

// TestAppendMLEntriesKeepsSplitDetailColumns 验证分离配置下合并 ML 明细列
// **照常保留**（互不冲突共存，对仗 GL 叶子页与合并页共存）。
func TestAppendMLEntriesKeepsSplitDetailColumns(t *testing.T) {
	cfg := &balance.GlobalConfig{}
	cfg.Settings.DetailSplit = []string{"管理费用"}

	wb := &Workbook{File: excelize.NewFile(), Config: cfg, Month: "2026-03"}
	entries := []voucher.Entry{
		{Date: "2026-03-05", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 300},
		{Date: "2026-03-06", GeneralAccount: "管理费用", DetailAccount: "差旅费", DebitCents: 700},
	}
	if err := wb.AppendMLEntries(entries, map[string]int64{}); err != nil {
		t.Fatalf("AppendMLEntries: %v", err)
	}

	name := sheetNameML("管理费用")
	if idx, err := wb.File.GetSheetIndex(name); err != nil || idx < 0 {
		t.Fatalf("merged ML sheet %s should exist", name)
	}
	detailIdx, details, err := wb.readMLDetailHeaders(name)
	if err != nil {
		t.Fatalf("readMLDetailHeaders: %v", err)
	}
	if _, ok := detailIdx["办公费"]; !ok {
		t.Errorf("merged ML must KEEP 办公费 column (no conflict), got %v", details)
	}
	if _, ok := detailIdx["差旅费"]; !ok {
		t.Errorf("merged ML must KEEP 差旅费 column, got %v", details)
	}
}

// TestAppendDetailLedgerEntriesBuildsPage 验证独立页生成（ML 样式，总账名配置）：
// 页名/页头科目区/期初行/分录/余额链/明细列全空。
func TestAppendDetailLedgerEntriesBuildsPage(t *testing.T) {
	cfg := &balance.GlobalConfig{}
	cfg.Settings.DetailSplit = []string{"管理费用"} // 总账名：其下所有明细分离

	wb := &Workbook{File: excelize.NewFile(), Config: cfg, Month: "2026-03"}
	initials := map[string]int64{"管理费用-办公费": 50000}
	entries := []voucher.Entry{
		{Date: "2026-03-05", VoucherNum: 1, Summary: "买耗材", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 30000},
		{Date: "2026-03-08", VoucherNum: 2, Summary: "报销", GeneralAccount: "管理费用", DetailAccount: "办公费", CreditCents: 12000},
	}
	if err := wb.AppendDetailLedgerEntries(entries, initials); err != nil {
		t.Fatalf("AppendDetailLedgerEntries: %v", err)
	}

	name := sheetNameDetail("管理费用-办公费")
	if idx, err := wb.File.GetSheetIndex(name); err != nil || idx < 0 {
		t.Fatalf("standalone sheet %s should exist", name)
	}

	lay := mlLayout()
	// 数据页页头"会计科目"区 = 叶子全路径（Row+2 = fdp+2 行，detail12+1 列）
	acctCol := mlDetailCol(lay, 11) + 1
	acct, _ := wb.File.GetCellValue(name, mlCellName(acctCol, mlFirstDataPageStart()+2))
	if acct != "管理费用-办公费" {
		t.Errorf("page header account = %q, want 管理费用-办公费", acct)
	}

	// 期初行（首数据页承前页位置）：ML 语义 initial≠0 → "期初余额"，余额 500
	cfRow := mlFirstDataPageStart() + lay.DataStartRow
	label, _ := wb.File.GetCellValue(name, mlCellName(lay.BackStartCol+mlOffSummary, cfRow))
	if label != "期初余额" {
		t.Errorf("initial row label = %q, want 期初余额", label)
	}
	bal, _ := wb.File.GetCellValue(name, mlCellName(lay.BackStartCol+mlOffBalance, cfRow))
	if parseTestYuan(bal) != 500 {
		t.Errorf("initial balance = %q, want 500", bal)
	}

	// 分录两行：500 + 300 - 120 = 680
	lastBal, _ := wb.File.GetCellValue(name, mlCellName(lay.BackStartCol+mlOffBalance, cfRow+2))
	if parseTestYuan(lastBal) != 680 {
		t.Errorf("running balance = %q, want 680", lastBal)
	}

	// 明细列全空（单明细科目页无下级明细）：抽查明细1 与 明细5 列首条分录行
	for _, i := range []int{0, 4} {
		col := mlDetailCol(lay, i)
		v, _ := wb.File.GetCellValue(name, mlCellName(col, cfRow+1))
		if strings.TrimSpace(v) != "" {
			t.Errorf("detail column %d must be empty on split page, got %q", i, v)
		}
	}
}

// TestAppendDetailLedgerEntriesInitialOnly 验证仅期初无分录的命中科目：
// 1 月建页写期初行；非 1 月无调整额不建页。
func TestAppendDetailLedgerEntriesInitialOnly(t *testing.T) {
	cfg := &balance.GlobalConfig{}
	cfg.Settings.DetailSplit = []string{"管理费用-办公费"}

	// 非 1 月、无调整额：不建页
	wb := &Workbook{File: excelize.NewFile(), Config: cfg, Month: "2026-03"}
	if err := wb.AppendDetailLedgerEntries(nil, map[string]int64{"管理费用-办公费": 50000}); err != nil {
		t.Fatalf("AppendDetailLedgerEntries: %v", err)
	}
	if idx, err := wb.File.GetSheetIndex(sheetNameDetail("管理费用-办公费")); err == nil && idx >= 0 {
		t.Error("page must not be built in non-January month without adjustment")
	}

	// 1 月：建页 + 期初行（ML 语义 initial≠0 → "期初余额"）+ 结构过次页补齐
	wb2 := &Workbook{File: excelize.NewFile(), Config: cfg, Month: "2026-01"}
	if err := wb2.AppendDetailLedgerEntries(nil, map[string]int64{"管理费用-办公费": 50000}); err != nil {
		t.Fatalf("AppendDetailLedgerEntries (Jan): %v", err)
	}
	name := sheetNameDetail("管理费用-办公费")
	lay := mlLayout()
	cfRow := mlFirstDataPageStart() + lay.DataStartRow
	label, _ := wb2.File.GetCellValue(name, mlCellName(lay.BackStartCol+mlOffSummary, cfRow))
	if label != "期初余额" {
		t.Errorf("January initial label = %q, want 期初余额", label)
	}
}

// TestWriteMLMonthClosingsSplitDetail 验证分离页月结四行（ML 家族专职）落页。
func TestWriteMLMonthClosingsSplitDetail(t *testing.T) {
	cfg := &balance.GlobalConfig{}
	cfg.Settings.DetailSplit = []string{"管理费用-办公费"}

	wb := &Workbook{File: excelize.NewFile(), Config: cfg, Month: "2026-03"}
	initials := map[string]int64{"管理费用-办公费": 50000}
	entries := []voucher.Entry{
		{Date: "2026-03-05", VoucherNum: 1, Summary: "买耗材", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 30000},
	}
	if err := wb.AppendDetailLedgerEntries(entries, initials); err != nil {
		t.Fatalf("AppendDetailLedgerEntries: %v", err)
	}

	changedSheets := map[string]bool{sheetNameDetail("管理费用-办公费"): true}
	if err := wb.WriteMLMonthClosings(entries, nil, nil, nil, nil, changedSheets); err != nil {
		t.Fatalf("WriteMLMonthClosings: %v", err)
	}

	name := sheetNameDetail("管理费用-办公费")
	rows, _ := wb.File.GetRows(name)
	lay := mlLayout()
	sumIdx := lay.BindingLeftCols + mlOffSummary
	balIdx := lay.BindingLeftCols + mlOffBalance
	found := map[string]bool{}
	endBal := ""
	for _, r := range rows {
		if len(r) > sumIdx {
			switch strings.TrimSpace(r[sumIdx]) {
			case "本月合计", "本季合计", "本年累计", "期末余额":
				found[strings.TrimSpace(r[sumIdx])] = true
				if strings.TrimSpace(r[sumIdx]) == "期末余额" && len(r) > balIdx {
					endBal = strings.TrimSpace(r[balIdx])
				}
			}
		}
	}
	for _, want := range []string{"本月合计", "本季合计", "本年累计", "期末余额"} {
		if !found[want] {
			t.Errorf("split page missing closing row %q", want)
		}
	}
	// 期末余额 = 500 + 300 = 800
	if parseTestYuan(endBal) != 800 {
		t.Errorf("split page final balance = %q, want 800", endBal)
	}
}

// TestGenerateWorkbookDetailSplit 全流程（GenerateWorkbook，总账名配置）：
// 其下所有明细各得独立页、GL 叶子页照常（互不影响）、合并 ML 列保留
// （互不冲突共存）、合并父级进配置报错、配置移除后孤儿页收敛。
func TestGenerateWorkbookDetailSplit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "科目余额总览.json")

	writeCfg := func(split []string) {
		cfg := map[string]any{
			"全局设置": map[string]any{
				"启动月":        "2026-01",
				"科目顺序":       []string{"银行存款", "实收资本", "管理费用"},
				"科目映射表":      map[string]string{},
				"合并总账科目":     []string{},
				"总分类账忽略科目":   []string{},
				"多科目明细账忽略科目": []string{},
				"分离明细账科目":    split,
			},
			"科目树": map[string]any{
				"银行存款-工行":  map[string]any{"科目属性": "借", "余额": map[string]any{}},
				"实收资本":     map[string]any{"科目属性": "贷", "余额": map[string]any{}},
				"管理费用-办公费": map[string]any{"科目属性": "借", "余额": map[string]any{}},
				"管理费用-差旅费": map[string]any{"科目属性": "借", "余额": map[string]any{}},
			},
			"自动识别科目": []any{},
			"手动调整科目": []any{},
			"明细列顺序":  map[string]any{},
		}
		b, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(configPath, b, 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	entries := []voucher.Entry{
		{Date: "2026-01-05", VoucherNum: 1, Summary: "办公费", GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 30000},
		{Date: "2026-01-06", VoucherNum: 2, Summary: "差旅费", GeneralAccount: "管理费用", DetailAccount: "差旅费", DebitCents: 70000, CreditCents: 0},
		{Date: "2026-01-07", VoucherNum: 3, Summary: "现金付", GeneralAccount: "银行存款", DetailAccount: "工行", CreditCents: 100000},
	}

	// 1) 总账名配置：其下所有明细各得独立页
	writeCfg([]string{"管理费用"})
	if err := GenerateWorkbook(configPath, "2026-01", dir, entries); err != nil {
		t.Fatalf("GenerateWorkbook: %v", err)
	}
	f, err := excelize.OpenFile(filepath.Join(dir, "2026-01.xlsx"))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	sheets := f.GetSheetList()
	has := func(name string) bool {
		for _, s := range sheets {
			if s == name {
				return true
			}
		}
		return false
	}
	idx := func(name string) int {
		for i, s := range sheets {
			if s == name {
				return i
			}
		}
		return -1
	}
	if !has("明细账-管理费用-办公费") || !has("明细账-管理费用-差旅费") {
		t.Errorf("all details of 配置总账 must split, sheets=%v", sheets)
	}
	if !has("总分类账-管理费用-办公费") {
		t.Errorf("GL leaf page must stay (no interference), sheets=%v", sheets)
	}
	// 合并 ML 列保留（互不冲突共存）
	mlSheet := sheetNameML("管理费用")
	detIdx, details, err := readMLDetailHeadersFrom(f, mlSheet)
	if err != nil {
		t.Fatalf("read headers: %v", err)
	}
	if _, ok := detIdx["办公费"]; !ok {
		t.Errorf("merged ML must KEEP 办公费 column, got %v", details)
	}
	if _, ok := detIdx["差旅费"]; !ok {
		t.Errorf("merged ML must KEEP 差旅费 column, got %v", details)
	}
	// 排序：分离页在 ML 区块之后（ML 家族分离形态殿后）
	if !(idx("明细账-管理费用-办公费") > idx(mlSheet)) {
		t.Errorf("sort order: split pages(%d) must follow merged ML(%d)", idx("明细账-管理费用-办公费"), idx(mlSheet))
	}
	// 页头科目区 = 全路径（ML 样式页头）
	lay := mlLayout()
	acctCol := mlDetailCol(lay, 11) + 1
	acct, _ := f.GetCellValue("明细账-管理费用-办公费", mlCellName(acctCol, mlFirstDataPageStart()+2))
	if acct != "管理费用-办公费" {
		t.Errorf("page header account = %q, want 管理费用-办公费", acct)
	}
	f.Close()

	// 2) 合并父级进配置：报错
	writeCfg2 := map[string]any{
		"全局设置": map[string]any{
			"启动月":        "2026-01",
			"科目顺序":       []string{},
			"科目映射表":      map[string]string{},
			"合并总账科目":     []string{"管理费用"},
			"总分类账忽略科目":   []string{},
			"多科目明细账忽略科目": []string{},
			"分离明细账科目":    []string{"管理费用"},
		},
		"科目树":    map[string]any{},
		"自动识别科目": []any{},
		"手动调整科目": []any{},
		"明细列顺序":  map[string]any{},
	}
	b2, _ := json.MarshalIndent(writeCfg2, "", "  ")
	if err := os.WriteFile(configPath, b2, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := GenerateWorkbook(configPath, "2026-01", dir, entries); err == nil {
		t.Fatal("expected error when merge parent is also split")
	} else if !strings.Contains(err.Error(), "不能同时配置为分离明细账科目") {
		t.Errorf("unexpected error: %v", err)
	}

	// 3) 配置移除：孤儿页收敛
	writeCfg(nil)
	if err := GenerateWorkbook(configPath, "2026-01", dir, entries); err != nil {
		t.Fatalf("GenerateWorkbook (no split): %v", err)
	}
	f2, err := excelize.OpenFile(filepath.Join(dir, "2026-01.xlsx"))
	if err != nil {
		t.Fatalf("reopen xlsx: %v", err)
	}
	defer f2.Close()
	for _, s := range f2.GetSheetList() {
		if strings.HasPrefix(s, sheetPrefixDetail) {
			t.Errorf("orphan split page %s must be removed after config change", s)
		}
	}
}

// readMLDetailHeadersFrom 从已打开的工作簿读合并 ML 明细列头（测试助手）。
func readMLDetailHeadersFrom(f *excelize.File, mlSheet string) (map[string]int, []string, error) {
	lay := layout.MLComputeLayout(layout.DefaultMLSpec())
	detailIdx := make(map[string]int)
	details := make([]string, mlMaxDetails)
	rows, err := f.GetRows(mlSheet)
	if err != nil {
		return nil, nil, err
	}
	colHeaderRow := mlFirstDataPageStart() + 5
	if len(rows) < colHeaderRow {
		return detailIdx, details, nil
	}
	rowData := rows[colHeaderRow-1]
	for i := 0; i < mlMaxDetails; i++ {
		var colIdx int
		if i < 4 {
			colIdx = lay.BindingLeftCols + 9 + i
		} else {
			colIdx = lay.FrontStartCol - 1 + (i - 4)
		}
		label := ""
		if colIdx < len(rowData) {
			label = strings.TrimSpace(rowData[colIdx])
		}
		details[i] = label
		if label != "" {
			detailIdx[label] = i
		}
	}
	return detailIdx, details, nil
}

// parseTestYuan 宽松解析金额（excelize GetCellValue 对数字格式返回不稳定）。
func parseTestYuan(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%g", &v); err != nil {
		return -1e18
	}
	return v
}
