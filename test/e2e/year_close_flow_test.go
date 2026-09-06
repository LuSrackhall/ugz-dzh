package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestYearEndStage2TransferFlow 年末两段结转全流程（v0.9.4 二段结转）：
// 建账（银行存款 100,000 / 实收资本 100,000）→ 1~11 月每月收经营款 1,000 →
// 12 月付办公费 500 → gen-close（一段损益结转 + 二段 本年收益→收益分配-未分配收益）
// → generate -f 12 月并入 → 断言 损益全 0、本年收益=0、收益分配-未分配收益=净额 10,500（贷余）
// + 资产负债表权益列口径（收益分配行在列、无"未结转损益"行、左右平衡）
// → year-close（零"漏结转"告警）→ 2026-01 期初断言（收益分配=净额、损益/本年收益从 0 起）
// + 混合月态（2026-01 权益列 收益分配 与 本年收益（未结转损益） 两行并存）→ gen-close 幂等。
func TestYearEndStage2TransferFlow(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("找不到项目根目录: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "ledger")
	buildCmd := exec.Command("go", "build", "-o", bin, ".")
	buildCmd.Dir = root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	output := t.TempDir()
	yearDir := filepath.Join(output, "2025")
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		t.Fatalf("创建输出目录: %v", err)
	}
	voucherRoot := t.TempDir()

	makeVoucher := func(month, num, summary, debitAcct, creditAcct, amount string) string {
		return fmt.Sprintf(`红旗路办事处

记字第%s号 1/1

记帐凭证

%s年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td></td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td></td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[:4], month[5:7], summary, debitAcct, amount, summary, creditAcct, amount, amount, amount)
	}
	writeVoucherFile := func(month, num, content string) {
		dir := filepath.Join(voucherRoot, month)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("建凭证目录: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "记字第"+num+"号.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("写凭证: %v", err)
		}
	}

	// 凭证：1~11 月收经营款 1,000（借 银行存款 / 贷 经营收入）；12 月付办公费 500（借 管理费用 / 贷 银行存款）
	for i := 1; i <= 11; i++ {
		m := fmt.Sprintf("2025-%02d", i)
		num := fmt.Sprintf("%04d", i)
		writeVoucherFile(m, num, makeVoucher(m, num, "收经营款", "银行存款", "经营收入", "1,000.00"))
	}
	writeVoucherFile("2025-12", "0012", makeVoucher("2025-12", "0012", "付办公费", "管理费用", "银行存款", "500.00"))

	// JSON：建账期初 银行存款 借 100,000 / 实收资本 贷 100,000；损益科目预登记（先定义后生成）
	jsonContent := `{
  "全局设置": {
    "启动月": "2025-01",
    "科目顺序": ["银行存款", "实收资本", "经营收入", "管理费用"],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "银行存款": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 10000000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": -10000000}, "余额": {}},
    "经营收入": {"科目属性": "贷", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "管理费用": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "经营收入", "首次月份": "2025-01", "期初调整额": 0},
    {"科目": "管理费用", "首次月份": "2025-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款", "生效月份": "2025-01", "期初调整额": 100000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2025-01", "期初调整额": -100000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	jsonPath := filepath.Join(yearDir, "2025.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}

	gen := func(month string, force bool) string {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		cmd := exec.Command(bin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
		return string(out)
	}

	// 树余额读取：科目树 → 科目 → 余额 → 月 → 字段（分）
	treeBal := func(jsonPath, account, month, field string) float64 {
		raw, err := os.ReadFile(jsonPath)
		if err != nil {
			t.Fatalf("读 JSON: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("解析 JSON: %v", err)
		}
		tree, _ := doc["科目树"].(map[string]any)
		node, ok := tree[account].(map[string]any)
		if !ok {
			t.Fatalf("科目树缺少 %q", account)
		}
		balances, _ := node["余额"].(map[string]any)
		mb, ok := balances[month].(map[string]any)
		if !ok {
			return 0 // 无当月记录 = 余额 0（回写循环 2 不为零值科目写当月记录）
		}
		v, _ := mb[field].(float64)
		return v
	}

	// 阶段 1：顺序生成 1~12 月（12 月此时无结转凭证）
	for i := 1; i <= 12; i++ {
		gen(fmt.Sprintf("2025-%02d", i), false)
	}

	// 阶段 2：gen-close 两段齐发
	closeCmd := exec.Command(bin, "gen-close", "-j", jsonPath, "-o", output)
	closeOut, err := closeCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gen-close 失败: %v\n%s", err, closeOut)
	}
	if !strings.Contains(string(closeOut), "二段结转凭证") || !strings.Contains(string(closeOut), "收益分配-未分配收益") {
		t.Errorf("gen-close 未生成二段凭证，输出:\n%s", closeOut)
	}
	closingFiles, _ := filepath.Glob(filepath.Join(yearDir, "closing", "*.md"))
	if len(closingFiles) != 2 {
		t.Fatalf("closing/ 凭证数 = %d, want 2（一段+二段）", len(closingFiles))
	}

	// 阶段 3：generate -f 12 月并入两段凭证
	out := gen("2025-12", true)
	if !strings.Contains(out, "已并入 2 张自动结转凭证") {
		t.Errorf("12 月未并入 2 张结转凭证，输出:\n%s", out)
	}

	// JSON 链断言（2025-12）：损益全 0、本年收益 0、收益分配-未分配收益 = 净额 10,500 元（贷余 -1,050,000 分）
	for acct, want := range map[string]float64{
		"经营收入": 0, "管理费用": 0, "本年收益": 0,
		"收益分配-未分配收益": -1050000,
	} {
		if got := treeBal(jsonPath, acct, "2025-12", "期末"); got != want {
			t.Errorf("[2025-12] %s 期末 = %v 分, want %v", acct, got, want)
		}
	}

	// 资产负债表（2025-12）：已结转月态——权益列显示 收益分配-未分配收益，无"未结转损益"行
	{
		f, err := excelize.OpenFile(filepath.Join(yearDir, "2025-12.xlsx"))
		if err != nil {
			t.Fatalf("打开 2025-12: %v", err)
		}
		defer f.Close()
		rows, err := f.GetRows("资产负债表")
		if err != nil {
			t.Fatalf("读取资产负债表: %v", err)
		}
		foundTransfer := false
		for i, r := range rows {
			if len(r) >= 3 && strings.TrimSpace(r[2]) == "收益分配-未分配收益" {
				foundTransfer = true
				if got := yuanVal(rows, i, 3); got != 10500 {
					t.Errorf("[资产负债表 2025-12] 收益分配-未分配收益 = %v, want 10500（贷余为正）", got)
				}
			}
			if strings.Contains(strings.Join(r, "|"), "未结转损益") {
				t.Errorf("[资产负债表 2025-12] 已结转月不应出现 未结转损益 行: %v", r)
			}
		}
		if !foundTransfer {
			t.Errorf("[资产负债表 2025-12] 权益列缺少 收益分配-未分配收益 行")
		}
		for _, r := range rows {
			if strings.Contains(strings.Join(r, "|"), "差额（左-右）") {
				t.Errorf("[资产负债表 2025-12] 左右不平衡: %v", r)
			}
		}
		// GL 账页：收益分配-未分配收益 期末余额 = 10500
		glRows, err := f.GetRows("总分类账-收益分配-未分配收益")
		if err != nil {
			t.Fatalf("读取 收益分配-未分配收益 GL: %v", err)
		}
		last := -1
		for i, r := range glRows {
			for _, c := range r {
				if strings.TrimSpace(c) == "期末余额" {
					last = i
				}
			}
		}
		if last < 0 {
			t.Fatalf("收益分配-未分配收益 GL 无期末余额行")
		}
		if got := yuanVal(glRows, last, 12); got != 10500 {
			t.Errorf("[GL 2025-12] 收益分配-未分配收益 期末余额 = %v, want 10500", got)
		}
	}

	// 阶段 4：year-close —— 结转后零"漏结转"告警
	ycCmd := exec.Command(bin, "year-close", "-j", jsonPath, "-o", output)
	ycOut, err := ycCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("year-close 失败: %v\n%s", err, ycOut)
	}
	if strings.Contains(string(ycOut), "疑似漏结转") {
		t.Errorf("year-close 仍报漏结转告警，输出:\n%s", ycOut)
	}
	nextJSON := filepath.Join(output, "2026", "2026.json")
	if _, err := os.Stat(nextJSON); err != nil {
		t.Fatalf("新年 JSON 未生成: %v", err)
	}

	// 阶段 5：2026-01（混合月态）——收益分配期初=净额，损益/本年收益从 0 起
	writeVoucherFile("2026-01", "0001", makeVoucher("2026-01", "0001", "收经营款", "银行存款", "经营收入", "300.00"))
	gen("2026-01", false)
	if got := treeBal(nextJSON, "收益分配-未分配收益", "2026-01", "期初"); got != -1050000 {
		t.Errorf("[2026-01] 收益分配-未分配收益 期初 = %v 分, want -1050000（上年净额结转，借正贷负）", got)
	}
	for acct, field := range map[string]string{"经营收入": "期初", "本年收益": "期初", "管理费用": "期初"} {
		if got := treeBal(nextJSON, acct, "2026-01", field); got != 0 {
			t.Errorf("[2026-01] %s %s = %v 分, want 0", acct, field, got)
		}
	}
	{
		f, err := excelize.OpenFile(filepath.Join(output, "2026", "2026-01.xlsx"))
		if err != nil {
			t.Fatalf("打开 2026-01: %v", err)
		}
		defer f.Close()
		rows, err := f.GetRows("资产负债表")
		if err != nil {
			t.Fatalf("读取资产负债表: %v", err)
		}
		foundTransfer, foundUnclosed := false, false
		for i, r := range rows {
			if len(r) >= 3 && strings.TrimSpace(r[2]) == "收益分配-未分配收益" {
				foundTransfer = true
				if got := yuanVal(rows, i, 3); got != 10500 {
					t.Errorf("[资产负债表 2026-01] 收益分配-未分配收益 = %v, want 10500", got)
				}
			}
			if strings.Contains(strings.Join(r, "|"), "本年收益（未结转损益）") {
				foundUnclosed = true
				if got := yuanVal(rows, i, 3); got != 300 {
					t.Errorf("[资产负债表 2026-01] 本年收益（未结转损益） = %v, want 300（当年 1 月净收益）", got)
				}
			}
		}
		if !foundTransfer || !foundUnclosed {
			t.Errorf("[资产负债表 2026-01] 混合月态应两行并存：收益分配=%v 未结转损益=%v", foundTransfer, foundUnclosed)
		}
	}

	// 阶段 6：gen-close 幂等重跑（2025 年）不出新凭证
	if out, err := exec.Command(bin, "gen-close", "-j", jsonPath, "-o", output).CombinedOutput(); err != nil {
		t.Fatalf("gen-close 幂等重跑失败: %v\n%s", err, out)
	}
	closingFiles, _ = filepath.Glob(filepath.Join(yearDir, "closing", "*.md"))
	if len(closingFiles) != 2 {
		t.Errorf("幂等重跑后 closing/ 凭证数 = %d, want 2", len(closingFiles))
	}
}
