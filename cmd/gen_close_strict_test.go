package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ledger/balance"
)

// TestGenCloseStrictChain S4（红队 2026-09-04：gen-close 自锁）：
// 收紧模式下完整走通 generate → gen-close → generate -f（损益归零）。
// gen-close 引用的"本年收益"由 gen-close 预登记，generate 不再被自家闸门拦截。
func TestGenCloseStrictChain(t *testing.T) {
	dir := t.TempDir()
	if err := runCmd(t, "init", "-s", "2026-01", "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	configPath := filepath.Join(dir, "2026", "2026.json")

	// ① 登记科目（先定义后生成）
	csvPath := filepath.Join(dir, "审核表.csv")
	content := "科目,方向,期初余额,备注\n银行存款,借,,\n经营收入,贷,,\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "subjects", "import", "-f", csvPath, "-j", configPath); err != nil {
		t.Fatalf("import: %v", err)
	}

	// ② 凭证 + 生成 2026-01
	vdir := filepath.Join(dir, "vouchers-tmp")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	voucher := "记字第0001号\n2026年01月20日\n<table>\n" +
		"<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>\n" +
		"<tr><td>收经营款</td><td>银行存款</td><td></td><td>1,000.00</td><td></td></tr>\n" +
		"<tr><td>收经营款</td><td>经营收入</td><td></td><td></td><td>1,000.00</td></tr>\n" +
		"</table>"
	if err := os.WriteFile(filepath.Join(vdir, "记字第0001号.md"), []byte(voucher), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "generate", "-v", vdir, "-o", dir); err != nil {
		t.Fatalf("generate: %v", err)
	}

	// ③ gen-close（生成结转凭证，含"本年收益"，并自动预登记该科目）
	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir); err != nil {
		t.Fatalf("gen-close: %v", err)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := cfg.Tree["本年收益"]
	if !ok {
		t.Fatal("gen-close 未预登记 本年收益")
	}
	if node.Property != "贷" {
		t.Errorf("本年收益属性 = %q, want 贷", node.Property)
	}

	// ④ 重新 generate -f（并入结转凭证）——收紧闸门不得拦截自家结转凭证
	if err := runCmd(t, "generate", "-v", vdir, "-o", dir, "-f"); err != nil {
		t.Fatalf("结转后 generate 被拒（gen-close 自锁）: %v", err)
	}

	// ⑤ 损益归零 + 本年收益承接
	cfg, err = balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tree["经营收入"].Balances["2026-01"].Final; got != 0 {
		t.Errorf("经营收入期末 = %d, want 0（结转归零）", got)
	}
	if got := cfg.Tree["本年收益"].Balances["2026-01"].Final; got != -100000 {
		t.Errorf("本年收益期末 = %d, want -100000（贷余承接净收益 1000.00 元，借正贷负）", got)
	}
}
