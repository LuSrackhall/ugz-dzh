package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ledger/balance"
	"ledger/voucher"
)

// ---------- 纯函数：方向两态 / 求和口径 / 防重扫描 ----------

// TestStage2TransferDirection 二段方向两态（借正贷负）：
// 贷余（净收益）→ 借 本年收益 / 贷 收益分配-未分配收益；借余（净亏损）方向相反。
func TestStage2TransferDirection(t *testing.T) {
	// 净收益：本年收益贷余 -263358207（e2e 演示账套 2025 实测值）
	debit, credit, amount, ok := stage2Transfer(-263358207)
	if !ok || debit != "本年收益" || credit != transferTargetAccount || amount != 263358207 {
		t.Errorf("净收益方向 = (%q,%q,%d,%v), want (本年收益,%s,263358207,true)", debit, credit, amount, ok, transferTargetAccount)
	}
	// 净亏损：借余 +15000
	debit, credit, amount, ok = stage2Transfer(15000)
	if !ok || debit != transferTargetAccount || credit != "本年收益" || amount != 15000 {
		t.Errorf("净亏损方向 = (%q,%q,%d,%v), want (%s,本年收益,15000,true)", debit, credit, amount, ok, transferTargetAccount)
	}
	// 收支平衡：不生成二段
	if _, _, _, ok := stage2Transfer(0); ok {
		t.Errorf("postFinal=0 不应生成二段凭证")
	}
}

// TestStage2PostFinalThreeStates D2 核心等式：三种运行态 postFinal 同值。
// ① 干净首跑（本年收益无记录、损益带余额）
// ② 一段已生成未合并（closing/ 有凭证、JSON 仍损益非零）
// ③ 一段已合并（老版本升级账套：本年收益=净额、损益=0）
func TestStage2PostFinalThreeStates(t *testing.T) {
	mkCfg := func(benFinal, income, expense int64) *balance.GlobalConfig {
		cfg := &balance.GlobalConfig{Tree: map[string]balance.AccountNode{
			"经营收入": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: income}}},
			"管理费用": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: expense}}},
		}}
		if benFinal != 0 { // 干净账套本年收益无记录（态①）；已合并才有节点（态③）
			cfg.Tree["本年收益"] = balance.AccountNode{Balances: map[string]balance.MonthBalance{"2025-12": {Final: benFinal}}}
		}
		return cfg
	}
	// 净收益 1000 元：收入贷余 -150000，费用借余 +50000
	s := mkCfg(0, -150000, 50000)
	state1 := benClosingFinal(s, "2025-12") + pnlClosingSum(s, "2025-12")
	// 态②同态①（JSON 未变）
	// 态③：一段已合并 → 损益 0，本年收益 = -100000
	s3 := mkCfg(-100000, 0, 0)
	state3 := benClosingFinal(s3, "2025-12") + pnlClosingSum(s3, "2025-12")
	if state1 != -100000 {
		t.Errorf("干净首跑 postFinal = %d, want -100000", state1)
	}
	if state3 != -100000 {
		t.Errorf("一段已合并 postFinal = %d, want -100000（自愈等值）", state3)
	}
	// 未分类科目不计入求和
	u := &balance.GlobalConfig{Tree: map[string]balance.AccountNode{
		"自创科目": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 999}}},
		"经营收入": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: -100000}}},
	}}
	if got := pnlClosingSum(u, "2025-12"); got != -100000 {
		t.Errorf("未分类科目被计入 pnlClosingSum = %d, want -100000", got)
	}
}

// TestScanClosingVouchers 防重扫描：closed 全路径集合 + hasTransfer 独立口径
// （一段凭证含 本年收益 不触发；含 收益分配 才触发）。
func TestScanClosingVouchers(t *testing.T) {
	dir := t.TempDir()
	stage1 := "记字第0001号 1/1\n\n记帐凭证\n\n2025年12月31日\n\n附件 张\n\n" +
		"<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody>" +
		"<tr><td>年末损益结转</td><td>经营收入</td><td>分包工程</td><td>100.00</td><td></td></tr>" +
		"<tr><td>年末损益结转</td><td>本年收益</td><td></td><td></td><td>100.00</td></tr>" +
		"</tbody></table>\n"
	f1 := filepath.Join(dir, "记字第0001号 年末损益结转.md")
	if err := os.WriteFile(f1, []byte(stage1), 0o644); err != nil {
		t.Fatal(err)
	}
	closed, hasTransfer := scanClosingVouchers([]string{f1})
	if !closed["经营收入-分包工程"] || !closed["本年收益"] {
		t.Errorf("closed = %v, want 含 经营收入-分包工程 与 本年收益", closed)
	}
	if hasTransfer {
		t.Errorf("一段凭证不应触发 hasTransfer")
	}
	// 追加二段凭证
	stage2 := buildTransferVoucher(2, 2025, 12, 31, "本年收益", transferTargetAccount, 10000)
	f2 := filepath.Join(dir, "记字第0002号 年末收益结转.md")
	if err := os.WriteFile(f2, []byte(stage2), 0o644); err != nil {
		t.Fatal(err)
	}
	closed, hasTransfer = scanClosingVouchers([]string{f1, f2})
	if !hasTransfer {
		t.Errorf("二段凭证应触发 hasTransfer")
	}
	if !closed[transferTargetAccount] {
		t.Errorf("closed 应含 %s（全路径组合）", transferTargetAccount)
	}
	// 解析自检：二段凭证可被标准解析器识别且借贷平衡
	es, err := voucher.ParseFile(f2)
	if err != nil || len(es) != 2 {
		t.Fatalf("二段凭证解析 = %v 条, err=%v, want 2 条", len(es), err)
	}
	if es[0].DebitCents != 10000 || es[1].CreditCents != 10000 {
		t.Errorf("二段分录借贷金额错误: %+v", es)
	}
}

// TestBuildTransferVoucher 明细层级：总帐科目=收益分配、明细科目=未分配收益 分列两格。
func TestBuildTransferVoucher(t *testing.T) {
	content := buildTransferVoucher(7, 2025, 12, 31, "本年收益", transferTargetAccount, 263358207)
	for _, want := range []string{
		"记字第0007号 1/1", "2025年12月31日", "年末收益结转",
		"<td>本年收益</td><td></td><td>2633582.07</td>",
		"<td>收益分配</td><td>未分配收益</td><td></td><td>2633582.07</td>",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("二段凭证缺少 %q", want)
		}
	}
}

// ---------- 集成链路（进程内 runCmd，参照 gen_close_strict_test.go） ----------

// stage2ChainFixture 建账+登记科目+单笔凭证+生成首月。
func stage2ChainFixture(t *testing.T, dir, csv, voucher string) string {
	t.Helper()
	if err := runCmd(t, "init", "-s", "2026-01", "-o", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	configPath := filepath.Join(dir, "2026", "2026.json")
	csvPath := filepath.Join(dir, "审核表.csv")
	if err := os.WriteFile(csvPath, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "subjects", "import", "-f", csvPath, "-j", configPath); err != nil {
		t.Fatalf("import: %v", err)
	}
	vdir := filepath.Join(dir, "vouchers-tmp")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, "记字第0001号.md"), []byte(voucher), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "generate", "-v", vdir, "-o", dir); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return configPath
}

func closingFileCount(t *testing.T, dir string) int {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "2026", "closing", "*.md"))
	return len(files)
}

// TestGenCloseStage2ChainProfit 净收益年：两段齐发 → generate -f 并入 →
// 损益=0、本年收益=0、收益分配-未分配收益=净额（贷余）；重跑幂等。
func TestGenCloseStage2ChainProfit(t *testing.T) {
	dir := t.TempDir()
	configPath := stage2ChainFixture(t, dir,
		"科目,方向,期初余额,备注\n银行存款,借,,\n经营收入,贷,,\n",
		"记字第0001号\n2026年01月20日\n<table>\n"+
			"<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>\n"+
			"<tr><td>收经营款</td><td>银行存款</td><td></td><td>1,000.00</td><td></td></tr>\n"+
			"<tr><td>收经营款</td><td>经营收入</td><td></td><td></td><td>1,000.00</td></tr>\n"+
			"</table>")

	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir); err != nil {
		t.Fatalf("gen-close: %v", err)
	}
	if n := closingFileCount(t, dir); n != 2 {
		t.Fatalf("closing/ 凭证数 = %d, want 2（一段+二段）", n)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := cfg.Tree[transferTargetAccount]
	if !ok {
		t.Fatal("gen-close 未预登记 " + transferTargetAccount)
	}
	if node.Property != "贷" {
		t.Errorf("%s 属性 = %q, want 贷", transferTargetAccount, node.Property)
	}

	if err := runCmd(t, "generate", "-v", filepath.Join(dir, "vouchers-tmp"), "-o", dir, "-f"); err != nil {
		t.Fatalf("结转后 generate 被拒: %v", err)
	}
	cfg, err = balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tree["经营收入"].Balances["2026-01"].Final; got != 0 {
		t.Errorf("经营收入期末 = %d, want 0", got)
	}
	if got := cfg.Tree["本年收益"].Balances["2026-01"].Final; got != 0 {
		t.Errorf("本年收益期末 = %d, want 0（二段结转归零）", got)
	}
	if got := cfg.Tree[transferTargetAccount].Balances["2026-01"].Final; got != -100000 {
		t.Errorf("%s 期末 = %d, want -100000（贷余承接净收益 1000.00 元）", transferTargetAccount, got)
	}

	// 幂等：重跑 gen-close 不出新凭证
	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir); err != nil {
		t.Fatalf("gen-close 幂等重跑: %v", err)
	}
	if n := closingFileCount(t, dir); n != 2 {
		t.Errorf("幂等重跑后 closing/ 凭证数 = %d, want 2", n)
	}
}

// TestGenCloseStage2ChainLoss 净亏损年：二段方向相反（借 收益分配-未分配收益 / 贷 本年收益）。
func TestGenCloseStage2ChainLoss(t *testing.T) {
	dir := t.TempDir()
	configPath := stage2ChainFixture(t, dir,
		"科目,方向,期初余额,备注\n库存现金,借,,\n管理费用,借,,\n",
		"记字第0001号\n2026年01月20日\n<table>\n"+
			"<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>\n"+
			"<tr><td>付办公费</td><td>管理费用</td><td></td><td>400.00</td><td></td></tr>\n"+
			"<tr><td>付办公费</td><td>库存现金</td><td></td><td></td><td>400.00</td></tr>\n"+
			"</table>")

	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir); err != nil {
		t.Fatalf("gen-close: %v", err)
	}
	if err := runCmd(t, "generate", "-v", filepath.Join(dir, "vouchers-tmp"), "-o", dir, "-f"); err != nil {
		t.Fatalf("结转后 generate: %v", err)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tree[transferTargetAccount].Balances["2026-01"].Final; got != 40000 {
		t.Errorf("%s 期末 = %d, want 40000（借余=未弥补亏损 400.00 元）", transferTargetAccount, got)
	}
	// 二段凭证方向：借 收益分配-未分配收益 / 贷 本年收益
	files, _ := filepath.Glob(filepath.Join(dir, "2026", "closing", "*年末收益结转.md"))
	if len(files) != 1 {
		t.Fatalf("二段凭证数 = %d, want 1", len(files))
	}
	es, err := voucher.ParseFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 || es[0].GeneralAccount != "收益分配" || es[0].DetailAccount != "未分配收益" || es[0].DebitCents != 40000 {
		t.Errorf("净亏损二段分录错误: %+v", es)
	}
}

// TestGenCloseNoTransferFlag --no-transfer：仅一段，二段目标科目不登记。
func TestGenCloseNoTransferFlag(t *testing.T) {
	dir := t.TempDir()
	configPath := stage2ChainFixture(t, dir,
		"科目,方向,期初余额,备注\n银行存款,借,,\n经营收入,贷,,\n",
		"记字第0001号\n2026年01月20日\n<table>\n"+
			"<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>\n"+
			"<tr><td>收经营款</td><td>银行存款</td><td></td><td>1,000.00</td><td></td></tr>\n"+
			"<tr><td>收经营款</td><td>经营收入</td><td></td><td></td><td>1,000.00</td></tr>\n"+
			"</table>")

	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir, "--no-transfer"); err != nil {
		t.Fatalf("gen-close --no-transfer: %v", err)
	}
	if n := closingFileCount(t, dir); n != 1 {
		t.Fatalf("closing/ 凭证数 = %d, want 1（仅一段）", n)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Tree[transferTargetAccount]; ok {
		t.Errorf("--no-transfer 不应登记 %s", transferTargetAccount)
	}
}

// TestGenCloseStage2SelfHeal 老账套/中断自愈（design.md D2 态②）：
// 一段已生成未合并时重跑 gen-close → 二段补发；合并后终态正确。
func TestGenCloseStage2SelfHeal(t *testing.T) {
	dir := t.TempDir()
	configPath := stage2ChainFixture(t, dir,
		"科目,方向,期初余额,备注\n银行存款,借,,\n经营收入,贷,,\n",
		"记字第0001号\n2026年01月20日\n<table>\n"+
			"<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>\n"+
			"<tr><td>收经营款</td><td>银行存款</td><td></td><td>1,000.00</td><td></td></tr>\n"+
			"<tr><td>收经营款</td><td>经营收入</td><td></td><td></td><td>1,000.00</td></tr>\n"+
			"</table>")

	// 第一跑：--no-transfer 仅出一段
	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir, "--no-transfer"); err != nil {
		t.Fatalf("gen-close --no-transfer: %v", err)
	}
	if n := closingFileCount(t, dir); n != 1 {
		t.Fatalf("closing/ 凭证数 = %d, want 1", n)
	}
	// 未合并直接重跑（无 flag）：一段被 closed 跳过，二段按 Σ损益 重算补发
	if err := runCmd(t, "gen-close", "-j", configPath, "-o", dir); err != nil {
		t.Fatalf("gen-close 补发二段: %v", err)
	}
	if n := closingFileCount(t, dir); n != 2 {
		t.Fatalf("补发后 closing/ 凭证数 = %d, want 2", n)
	}
	// 一次合并两段 → 终态正确
	if err := runCmd(t, "generate", "-v", filepath.Join(dir, "vouchers-tmp"), "-o", dir, "-f"); err != nil {
		t.Fatalf("generate: %v", err)
	}
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tree["本年收益"].Balances["2026-01"].Final; got != 0 {
		t.Errorf("本年收益期末 = %d, want 0", got)
	}
	if got := cfg.Tree[transferTargetAccount].Balances["2026-01"].Final; got != -100000 {
		t.Errorf("%s 期末 = %d, want -100000", transferTargetAccount, got)
	}
}
