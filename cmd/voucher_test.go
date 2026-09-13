package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"ledger/balance"
	"ledger/voucher"
)

// setupLedger 建一个临时账套并登记指定科目（科目名 → 借/贷属性）。
//
// 必须同时写科目树与「自动识别科目」列表：balance.ValidateAccountTree 要求两者一致
// （科目树节点必须出现在自动识别/手动调整列表中），只写树会让 ledger check 报
// "科目树中存在多余科目"。照此镜像 subjects import 的真实登记结果。
func setupLedger(t *testing.T, startMonth string, subjects map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := runCmd(t, "init", "-s", startMonth, "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	year := startMonth[:4]
	cfgPath := filepath.Join(dir, year, year+".json")
	cfg, err := balance.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Tree == nil {
		cfg.Tree = map[string]balance.AccountNode{}
	}
	names := make([]string, 0, len(subjects))
	for name := range subjects {
		names = append(names, name)
	}
	sort.Strings(names) // 确定性：AutoItems 顺序可复现
	for _, name := range names {
		cfg.Tree[name] = balance.AccountNode{Property: subjects[name], Balances: map[string]balance.MonthBalance{}}
		cfg.AutoItems = append(cfg.AutoItems, balance.AutoItem{Account: name, FirstMonth: startMonth})
	}
	if err := balance.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	return dir
}

func standardSubjects() map[string]string {
	return map[string]string{
		"银行存款":      "借",
		"公益支出-补助费用": "借",
		"应付款-养老金":   "贷",
	}
}

func TestVoucherCommandsRegistered(t *testing.T) {
	for _, path := range [][]string{
		{"voucher"}, {"voucher", "add"}, {"voucher", "list"}, {"voucher", "check"},
	} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil || cmd == nil {
			t.Fatalf("未注册命令 %v: %v", path, err)
		}
	}
}

func TestVoucherAddFlagForm(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())

	if err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=9990.00",
		"--credit", "应付款-养老金=9990.00"); err != nil {
		t.Fatalf("voucher add: %v", err)
	}

	path := filepath.Join(dir, "vouchers", "2026_03", "记字第0001号.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("凭证文件未创建: %v", err)
	}
	parsed, err := voucher.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("分录条数 = %d, want 2", len(parsed))
	}
	if parsed[0].VoucherNum != 1 || parsed[0].Date != "2026-03-05" {
		t.Errorf("凭证号/日期 = %d/%s", parsed[0].VoucherNum, parsed[0].Date)
	}
	if parsed[0].GeneralAccount != "公益支出" || parsed[0].DetailAccount != "补助费用" || parsed[0].DebitCents != 999000 {
		t.Errorf("首条分录 = %+v", parsed[0])
	}
	if parsed[1].CreditCents != 999000 {
		t.Errorf("次条分录贷方 = %d", parsed[1].CreditCents)
	}
}

// TestVoucherAddDoesNotGenerate 记账不等于入账：add 不得生成账本、不得动 JSON。
func TestVoucherAddDoesNotGenerate(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	cfgPath := filepath.Join(dir, "2026", "2026.json")
	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=9990.00",
		"--credit", "应付款-养老金=9990.00"); err != nil {
		t.Fatalf("voucher add: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "2026", "2026-03.xlsx")); err == nil {
		t.Error("voucher add 不应生成账本 xlsx")
	}
	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("voucher add 不应修改账套 JSON")
	}
}

func TestVoucherAddAutoNumberIncrements(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	for i := 0; i < 3; i++ {
		if err := runCmd(t, "voucher", "add", "-o", dir,
			"--date", "2026-03-0"+string(rune('5'+i)), "--summary", "付养老金",
			"--debit", "公益支出-补助费用=100.00",
			"--credit", "应付款-养老金=100.00"); err != nil {
			t.Fatalf("第 %d 次 add: %v", i+1, err)
		}
	}
	for _, name := range []string{"记字第0001号.md", "记字第0002号.md", "记字第0003号.md"} {
		if _, err := os.Stat(filepath.Join(dir, "vouchers", "2026_03", name)); err != nil {
			t.Errorf("缺少 %s: %v", name, err)
		}
	}
}

func TestVoucherAddRejectsUnbalancedAndWritesNothing(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "不平衡",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=90.00")
	if err == nil || !strings.Contains(err.Error(), "平衡") {
		t.Fatalf("应因不平衡拒绝，得到: %v", err)
	}
	monthDir := filepath.Join(dir, "vouchers", "2026_03")
	if entries, _ := os.ReadDir(monthDir); len(entries) != 0 {
		t.Errorf("不平衡时不应写盘，实际剩 %d 个文件", len(entries))
	}
}

func TestVoucherAddRejectsUndefinedSubject(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "错名科目",
		"--debit", "管埋费用=100.00",
		"--credit", "应付款-养老金=100.00")
	if err == nil || !strings.Contains(err.Error(), "未定义科目") {
		t.Fatalf("应因未定义科目拒绝，得到: %v", err)
	}
	monthDir := filepath.Join(dir, "vouchers", "2026_03")
	if entries, _ := os.ReadDir(monthDir); len(entries) != 0 {
		t.Errorf("未定义科目时不应写盘，实际剩 %d 个文件", len(entries))
	}
}

func TestVoucherAddRejectsZeroAmount(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "零金额",
		"--debit", "公益支出-补助费用=0.00",
		"--credit", "应付款-养老金=0.00")
	if err == nil {
		t.Fatal("零金额应被拒绝")
	}
}

// TestVoucherAddRedInkNet 红字按净额参与平衡并写为括号。
func TestVoucherAddRedInkNet(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	if err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "红字冲减",
		"--debit", "公益支出-补助费用=100.00",
		"--debit", "公益支出-补助费用=-20.00",
		"--credit", "应付款-养老金=80.00"); err != nil {
		t.Fatalf("红字净额应平衡: %v", err)
	}
	path := filepath.Join(dir, "vouchers", "2026_03", "记字第0001号.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "(20.00)") {
		t.Errorf("红字应写为括号形式：\n%s", string(b))
	}
	parsed, err := voucher.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if parsed[1].DebitCents != -2000 {
		t.Errorf("红字解析 = %d, want -2000", parsed[1].DebitCents)
	}
}

func TestVoucherAddIdempotency(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	args := []string{"voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=100.00",
		"--idempotency-key", "k1"}
	if err := runCmd(t, args...); err != nil {
		t.Fatalf("首次: %v", err)
	}
	if err := runCmd(t, args...); err != nil {
		t.Fatalf("重复调用应幂等返回而非报错: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "vouchers", "2026_03"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("幂等键相同应只产生一张凭证，实际 %d 张", len(entries))
	}
	// 幂等索引在月目录之外
	if _, err := os.Stat(filepath.Join(dir, "vouchers", "2026_03", ".voucher-keys")); err == nil {
		t.Error("幂等索引不应落在月目录内")
	}
	if _, err := os.Stat(filepath.Join(dir, "vouchers", ".voucher-keys", "2026_03.json")); err != nil {
		t.Errorf("幂等索引应落在 vouchers/.voucher-keys/: %v", err)
	}
}

// TestVoucherAddIdempotencyRebuildsWhenFileDeleted 索引存在但凭证被人工删除 → 重新创建。
func TestVoucherAddIdempotencyRebuildsWhenFileDeleted(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	args := []string{"voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=100.00",
		"--idempotency-key", "k1"}
	if err := runCmd(t, args...); err != nil {
		t.Fatalf("首次: %v", err)
	}
	path := filepath.Join(dir, "vouchers", "2026_03", "记字第0001号.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, args...); err != nil {
		t.Fatalf("文件被删后应重建: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("应重新创建 %s: %v", path, err)
	}
}

func TestVoucherAddJSONFormAndMutualExclusion(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	jsonPath := filepath.Join(t.TempDir(), "v.json")
	payload := `{"date":"2026-03-06","summary":"付养老金","unit":"某居委会",
	  "entries":[{"account":"公益支出-补助费用","side":"借","amount":"100.00"},
	             {"account":"应付款-养老金","side":"贷","amount":100.00}]}`
	if err := os.WriteFile(jsonPath, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "voucher", "add", "-o", dir, "--json", jsonPath); err != nil {
		t.Fatalf("JSON 形态: %v", err)
	}
	path := filepath.Join(dir, "vouchers", "2026_03", "记字第0001号.md")
	parsed, err := voucher.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[0].DebitCents != 10000 || parsed[1].CreditCents != 10000 {
		t.Fatalf("JSON 形态解析异常: %+v", parsed)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "某居委会") {
		t.Error("单位名应写入抬头")
	}

	// 互斥
	if err := runCmd(t, "voucher", "add", "-o", dir, "--json", jsonPath,
		"--debit", "公益支出-补助费用=1.00"); err == nil || !strings.Contains(err.Error(), "互斥") {
		t.Fatalf("--json 与 --debit 应互斥，得到: %v", err)
	}
}

func TestVoucherAddExplicitNumConflict(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	base := []string{"voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=100.00"}
	if err := runCmd(t, append(append([]string{}, base...), "--num", "7")...); err != nil {
		t.Fatalf("显式号 7: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "vouchers", "2026_03", "记字第0007号.md")); err != nil {
		t.Fatalf("应按显式号落盘: %v", err)
	}
	err := runCmd(t, append(append([]string{}, base...), "--num", "7")...)
	if err == nil || !strings.Contains(err.Error(), "冲突") {
		t.Fatalf("显式号冲突应拒绝，得到: %v", err)
	}
}

// TestVoucherAddWarnsButDoesNotBlockOnDirtyDir 目录卫生问题默认只告警（兼容铁律一）。
func TestVoucherAddWarnsButDoesNotBlockOnDirtyDir(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	monthDir := filepath.Join(dir, "vouchers", "2026_03")
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monthDir, "草稿.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=100.00"); err != nil {
		t.Fatalf("存在非正式文件时默认不应阻断: %v", err)
	}
	if _, err := os.Stat(filepath.Join(monthDir, "记字第0001号.md")); err != nil {
		t.Errorf("应照常落盘: %v", err)
	}
}

func TestVoucherCheckDefaultWarnsStrictBlocks(t *testing.T) {
	dir := t.TempDir()
	monthDir := filepath.Join(dir, "2026_03")
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"记字第0001号.md", "记字第0003号.md"} { // 缺 0002
		if err := os.WriteFile(filepath.Join(monthDir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runCmd(t, "voucher", "check", "-v", monthDir); err != nil {
		t.Fatalf("默认模式应告警但零退出: %v", err)
	}
	if err := runCmd(t, "voucher", "check", "-v", monthDir, "--strict"); err == nil {
		t.Fatal("--strict 应非零退出")
	}
	// JSON 输出可消费
	if err := runCmd(t, "voucher", "check", "-v", monthDir, "--json"); err != nil {
		t.Fatalf("--json 模式: %v", err)
	}
}

func TestCollectVoucherList(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	add := func(day, amount string) {
		if err := runCmd(t, "voucher", "add", "-o", dir,
			"--date", "2026-03-"+day, "--summary", "付养老金",
			"--debit", "公益支出-补助费用="+amount,
			"--credit", "应付款-养老金="+amount); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	add("05", "100.00")
	add("06", "200.00")
	monthDir := filepath.Join(dir, "vouchers", "2026_03")
	rows, err := collectVoucherList(monthDir)
	if err != nil {
		t.Fatalf("collectVoucherList: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("行数 = %d, want 2", len(rows))
	}
	if rows[0].Num != 1 || rows[1].Num != 2 {
		t.Errorf("应按凭证号升序: %+v", rows)
	}
	if rows[0].DebitCents != 10000 || rows[1].DebitCents != 20000 {
		t.Errorf("借贷合计异常: %+v", rows)
	}
	if !strings.Contains(rows[0].VoucherNo, "0001") {
		t.Errorf("凭证号写法异常: %s", rows[0].VoucherNo)
	}
}

// TestVoucherAddThenGenerate 端到端：voucher add 产出的凭证必须能被 generate 消费，
// 且生成后的账本与 JSON 余额链一致（写入器与解析器、生成器三段打通）。
func TestVoucherAddThenGenerate(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	add := func(day, summary, debitAcct, creditAcct, amount string) {
		t.Helper()
		if err := runCmd(t, "voucher", "add", "-o", dir,
			"--date", "2026-03-"+day, "--summary", summary,
			"--debit", debitAcct+"="+amount,
			"--credit", creditAcct+"="+amount); err != nil {
			t.Fatalf("voucher add (%s): %v", summary, err)
		}
	}
	add("05", "收街道拨款", "银行存款", "应付款-养老金", "63194.00")
	add("08", "付养老金", "公益支出-补助费用", "应付款-养老金", "9990.00")

	monthDir := filepath.Join(dir, "vouchers", "2026_03")
	if err := runCmd(t, "generate", "-v", monthDir, "-o", dir); err != nil {
		t.Fatalf("generate 应能消费 voucher add 产出的凭证: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026", "2026-03.xlsx")); err != nil {
		t.Fatalf("账本未生成: %v", err)
	}
	// check 校验 xlsx 期末与 JSON 余额链一致
	if err := runCmd(t, "check", "-j", filepath.Join(dir, "2026", "2026.json")); err != nil {
		t.Fatalf("check: %v", err)
	}
}

// TestVoucherAddRejectsNonPositiveNum 显式给了非法 --num 必须报错，不得静默改为自动发号。
func TestVoucherAddRejectsNonPositiveNum(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	for _, bad := range []string{"0", "-3"} {
		err := runCmd(t, "voucher", "add", "-o", dir,
			"--date", "2026-03-05", "--summary", "付养老金",
			"--debit", "公益支出-补助费用=100.00",
			"--credit", "应付款-养老金=100.00",
			"--num", bad)
		if err == nil || !strings.Contains(err.Error(), "--num") {
			t.Fatalf("--num %s 应被拒绝，得到: %v", bad, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "vouchers", "2026_03")); len(entries) != 0 {
		t.Errorf("非法 --num 不应写盘，实际剩 %d 个文件", len(entries))
	}
}

// TestVoucherAddWarnsOnAlternativeLayout 检测到 vouchers/YYYY/MM 布局时提示但不阻断。
func TestVoucherAddWarnsOnAlternativeLayout(t *testing.T) {
	dir := setupLedger(t, "2026-03", standardSubjects())
	altDir := filepath.Join(dir, "vouchers", "2026", "03")
	if err := os.MkdirAll(altDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(altDir, "记字第0001号.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "voucher", "add", "-o", dir,
		"--date", "2026-03-05", "--summary", "付养老金",
		"--debit", "公益支出-补助费用=100.00",
		"--credit", "应付款-养老金=100.00"); err != nil {
		t.Fatalf("布局歧义应只提示不阻断: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "vouchers", "2026_03", "记字第0001号.md")); err != nil {
		t.Errorf("仍应写入 YYYY_MM 布局: %v", err)
	}
}
