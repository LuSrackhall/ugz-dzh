package cmd

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ledger/balance"
	"ledger/voucher"
)

// TestUndefinedVoucherSubjects 未定义科目清单：排序、计数、样例摘要；
// 空格未归一名称按 fullPath 口径拼接后比对。
func TestUndefinedVoucherSubjects(t *testing.T) {
	cfg := &balance.GlobalConfig{Tree: map[string]balance.AccountNode{
		"库存现金":     {},
		"管理费用-办公费": {},
	}}
	entries := []voucher.Entry{
		{GeneralAccount: "库存现金", Summary: "提现"},
		{GeneralAccount: "管理费用", DetailAccount: "办公费", Summary: "购办公用品"},
		{GeneralAccount: "管埋费用", Summary: "OCR错名", SourceFile: "记字第0009号.md"},
		{GeneralAccount: "管埋费用", Summary: "OCR错名2"},
		{GeneralAccount: "  应收款  ", DetailAccount: " 张三 ", Summary: "空白未归一"},
	}
	got := undefinedVoucherSubjects(cfg, entries)
	if len(got) != 2 {
		t.Fatalf("未定义科目数 = %d (%+v), want 2", len(got), got)
	}
	// 排序按字节序：应(U+5E94) < 管(U+7BA1)
	if got[0].Account != "应收款-张三" {
		t.Errorf("got[0].Account = %q, want 应收款-张三", got[0].Account)
	}
	if got[1].Account != "管埋费用" || got[1].Count != 2 || got[1].Sample != "OCR错名" || got[1].SampleFile != "记字第0009号.md" {
		t.Errorf("got[1] = %+v, want 管埋费用/2/OCR错名/记字第0009号.md", got[1])
	}
}

// TestSplitEntryPath 与 balance.splitPath 同口径。
func TestSplitEntryPath(t *testing.T) {
	cases := []struct{ in, gen, det string }{
		{"库存现金", "库存现金", ""},
		{"银行存款-工商银行", "银行存款", "工商银行"},
		{"应收款-张三-备注", "应收款", "张三-备注"},
	}
	for _, c := range cases {
		g, d := splitEntryPath(c.in)
		if g != c.gen || d != c.det {
			t.Errorf("splitEntryPath(%q) = (%q,%q), want (%q,%q)", c.in, g, d, c.gen, c.det)
		}
	}
}

func strictTestVoucher(t *testing.T, dir string) string {
	t.Helper()
	vdir := filepath.Join(dir, "vouchers-tmp")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `记字第0001号
2026年01月15日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>提现</td><td>库存现金</td><td></td><td>1,000.00</td><td></td></tr>
<tr><td>提现</td><td>银行存款</td><td>工商银行</td><td></td><td>1,000.00</td></tr>
</table>`
	if err := os.WriteFile(filepath.Join(vdir, "记字第0001号.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return vdir
}

func runCmd(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// fillDirections 补全审核表空方向（模拟 agent/人工确认动作）。
func fillDirections(t *testing.T, path, dir string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records[1:] {
		if len(r) > 1 && strings.TrimSpace(r[1]) == "" {
			r[1] = dir
		}
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.WriteAll(records)
	w.Flush()
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGenerateStrictModeEndToEnd 先定义后生成全链路：
// 未登记 → generate 拒绝（清单指引）→ scan → 确认 → import → generate 成功。
func TestGenerateStrictModeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := runCmd(t, "init", "-s", "2026-01", "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	configPath := filepath.Join(dir, "2026", "2026.json")
	vdir := strictTestVoucher(t, dir)

	// ① 未登记 → 拒绝
	if err := runCmd(t, "generate", "-v", vdir, "-o", dir); err == nil || !strings.Contains(err.Error(), "未定义科目") {
		t.Fatalf("未登记科目应拒绝生成，err=%v", err)
	}

	// ② scan → 确认 → import
	cand := filepath.Join(dir, "scan.csv")
	if err := runCmd(t, "subjects", "scan", "-v", vdir, "-o", cand, "-j", configPath); err != nil {
		t.Fatalf("scan: %v", err)
	}
	fillDirections(t, cand, "借")
	if err := runCmd(t, "subjects", "import", "-f", cand, "-j", configPath); err != nil {
		t.Fatalf("import: %v", err)
	}

	// ③ 重新 generate → 成功
	if err := runCmd(t, "generate", "-v", vdir, "-o", dir); err != nil {
		t.Fatalf("登记后仍失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026", "2026-01.xlsx")); err != nil {
		t.Error("账本未生成")
	}
}

// TestGenerateAllowNewEscape --allow-new 显式逃生：未登记科目自动登记（旧行为）。
func TestGenerateAllowNewEscape(t *testing.T) {
	dir := t.TempDir()
	if err := runCmd(t, "init", "-s", "2026-01", "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	configPath := filepath.Join(dir, "2026", "2026.json")
	vdir := strictTestVoucher(t, dir)

	if err := runCmd(t, "generate", "-v", vdir, "-o", dir, "--allow-new"); err != nil {
		t.Fatalf("--allow-new 应放行: %v", err)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"库存现金", "银行存款-工商银行"} {
		if _, ok := cfg.Tree[a]; !ok {
			t.Errorf("--allow-new 未自动登记 %s", a)
		}
	}
}
