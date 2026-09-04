package cmd

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ledger/balance"
)

// scanTestVoucher 最小凭证夹具（宽容解析：凭证号可有可无）。
const scanTestVoucher = `记字第0001号
2025年12月20日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
%s
</table>`

func writeScanFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f1 := filepath.Join(dir, "记字第0001号.md")
	content1 := strings.Replace(scanTestVoucher,
		"%s",
		"<tr><td>提现</td><td>库存现金</td><td></td><td>1,000.00</td><td></td></tr>"+
			"<tr><td>提现</td><td>银行存款</td><td>工商银行</td><td></td><td>1,000.00</td></tr>", 1)
	if err := os.WriteFile(f1, []byte(content1), 0o644); err != nil {
		t.Fatal(err)
	}
	f2 := filepath.Join(dir, "无号凭证.md")
	// 无凭证号正文 + 文件名亦无号（宽容解析仍提取科目；凭证号缺失仅告警不阻断）；
	// 管理费用-办公费 重复出现计频
	content2 := `2025年12月20日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>购办公用品</td><td>管理费用</td><td>办公费</td><td>200.00</td><td></td></tr>
<tr><td>提现</td><td>库存现金</td><td></td><td>200.00</td><td></td></tr>
<tr><td>OCR错名</td><td>管埋费用</td><td></td><td></td><td>50.00</td></tr>
</table>`
	if err := os.WriteFile(f2, []byte(content2), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func scanTestConfig(t *testing.T) *balance.GlobalConfig {
	t.Helper()
	return &balance.GlobalConfig{
		Settings: balance.GlobalSettings{StartMonth: "2025-12"},
		Tree: map[string]balance.AccountNode{
			"管理费用-办公费": {Property: "借"},
			"库存现金":     {Property: "借"},
		},
	}
}

// TestScanVoucherSubjects 宽容扫描：提取科目对与频次；凭证号缺失不阻断。
func TestScanVoucherSubjects(t *testing.T) {
	dir := writeScanFixtures(t)
	cands, err := scanVoucherSubjects(dir)
	if err != nil {
		t.Fatalf("scanVoucherSubjects: %v", err)
	}
	got := map[string]scanCandidate{}
	for _, c := range cands {
		got[c.Account] = c
	}
	wantAccounts := []string{"管理费用-办公费", "库存现金", "银行存款-工商银行", "管埋费用"}
	if len(cands) != len(wantAccounts) {
		t.Fatalf("候选数 = %d (%v), want %d", len(cands), cands, len(wantAccounts))
	}
	for _, a := range wantAccounts {
		if _, ok := got[a]; !ok {
			t.Errorf("缺少候选科目 %s", a)
		}
	}
	if got["管理费用-办公费"].Count != 1 {
		t.Errorf("管理费用-办公费 出现次数 = %d, want 1", got["管理费用-办公费"].Count)
	}
	if got["库存现金"].Count != 2 {
		t.Errorf("库存现金 出现次数 = %d, want 2", got["库存现金"].Count)
	}
	if got["管埋费用"].Count != 1 {
		t.Errorf("管埋费用 出现次数 = %d, want 1", got["管埋费用"].Count)
	}
}

func TestScanVoucherSubjectsEmptyDir(t *testing.T) {
	if _, err := scanVoucherSubjects(t.TempDir()); err == nil {
		t.Error("空目录应报错")
	}
	if _, err := scanVoucherSubjects(filepath.Join(t.TempDir(), "不存在")); err == nil {
		t.Error("不存在的目录应报错")
	}
}

// TestScanSuggestedDirection 方向建议：已登记沿用 / 官方表收录按官方属性 /
// 未知留空（确认闸门）。
func TestScanSuggestedDirection(t *testing.T) {
	cfg := scanTestConfig(t)
	cases := []struct {
		account, general, want string
	}{
		{"管理费用-办公费", "管理费用", "借"},      // 已登记沿用
		{"库存现金", "库存现金", "借"},          // 已登记沿用
		{"专项应付款-日间照料中心", "专项应付款", "贷"}, // 官方表收录
		{"管埋费用", "管埋费用", ""},           // 未知 → 留空（subjects import 会拒绝，构成确认闸门）
	}
	for _, c := range cases {
		got := scanSuggestedDirection(cfg, scanCandidate{Account: c.account, General: c.general})
		if got != c.want {
			t.Errorf("scanSuggestedDirection(%s) = %q, want %q", c.account, got, c.want)
		}
	}
}

// TestScanCSVRowsFeedableToImport scan 产物补全方向后可直接回喂 parseReviewCSV
// （列序无关、多余列忽略的闭环验证）。
func TestScanCSVRowsFeedableToImport(t *testing.T) {
	cfg := scanTestConfig(t)
	dir := writeScanFixtures(t)
	cands, err := scanVoucherSubjects(dir)
	if err != nil {
		t.Fatal(err)
	}
	rows := scanCSVRows(cfg, cands)
	if rows[0][0] != "科目" || rows[0][1] != "方向" || rows[0][2] != "期初余额" || rows[0][3] != "备注" {
		t.Fatalf("表头与建账审核表不兼容: %v", rows[0])
	}

	// 写成 CSV（模拟 agent/人工把空方向补为 借），再 parseReviewCSV 应通过
	path := filepath.Join(t.TempDir(), "候选.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(f)
	for _, r := range rows {
		if r[1] == "" {
			r[1] = "借" // 确认动作
		}
		_ = w.Write(r)
	}
	w.Flush()
	f.Close()

	parsed, err := parseReviewCSV(path)
	if err != nil {
		t.Fatalf("scan 产物应可直接回喂 subjects import: %v", err)
	}
	if len(parsed) != len(rows)-1 {
		t.Errorf("解析行数 = %d, want %d", len(parsed), len(rows)-1)
	}
}

// TestSubjectsImportAppliesDirectionForNewSubjects S3：新登记科目同样应用
// 审核表「方向」列（此前只有已存在科目才设置属性，自创名新科目的方向被静默
// 丢弃，需二次 import）。
func TestSubjectsImportAppliesDirectionForNewSubjects(t *testing.T) {
	dir := t.TempDir()
	if err := runCmd(t, "init", "-s", "2026-01", "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	configPath := filepath.Join(dir, "2026", "2026.json")

	csvPath := filepath.Join(dir, "审核表.csv")
	content := "科目,方向,期初余额,备注\n" +
		"官司支出-律师费,贷,,自创科目名（非官方 42 表）\n" + // 官方表查无此名 → 属性不能靠推断
		"累计折旧,贷,,备抵科目（官方表 Property=贷）\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "subjects", "import", "-f", csvPath, "-j", configPath); err != nil {
		t.Fatalf("import: %v", err)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tree["官司支出-律师费"].Property; got != "贷" {
		t.Errorf("新登记自创科目属性 = %q, want 贷（方向列必须生效）", got)
	}
	if got := cfg.Tree["累计折旧"].Property; got != "贷" {
		t.Errorf("累计折旧属性 = %q, want 贷（备抵科目）", got)
	}
}
