package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// 复现下游用户报告的 0.9.0 bug：科目被凭证冲平后静置一个月再重现，
// 期初被错误重置为建账期初（应为上月期末 0）——宏远公司1 六月冲平、八月重现期初又变回 1,003 万。
// 场景：建账期初 10,030,000 元 → 2~5 月静置（M4 补月结链）→ 6 月凭证冲平（期末=0）
// → 7 月静置 → 8 月重现（新分录）。断言 8 月期初=0（余额链：6月期末0 → 7月0 → 8月期初0）。
// 再用 -f 重跑 6/7/8 月（下游报告的复发路径），断言不复发。
func TestFlushFlatSubjectOpeningChain(t *testing.T) {
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

	// 凭证：每月一个目录。1~5 月、7 月为无关分录（保住"当月必须有分录"）；
	// 6 月冲平宏远公司1；8 月重现。
	voucherRoot := t.TempDir()

	// 统一用一借一贷凭证构造器
	makeVoucher := func(month, num, summary, debitAcct, creditAcct, amount string) string {
		return fmt.Sprintf(`红旗路办事处

记字第%s号 1/1

记帐凭证

2025年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td></td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td></td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[5:7], summary, debitAcct, amount, summary, creditAcct, amount, amount, amount)
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

	// 建账月 dummy + 2~5 月 dummy（不动宏远公司1）
	dummy := func(month, num string) string {
		return makeVoucher(month, num, "零星收支", "库存现金", "银行存款", "100.00")
	}
	writeVoucherFile("2025-01", "0001", dummy("2025-01", "0001"))
	writeVoucherFile("2025-02", "0002", dummy("2025-02", "0002"))
	writeVoucherFile("2025-03", "0003", dummy("2025-03", "0003"))
	writeVoucherFile("2025-04", "0004", dummy("2025-04", "0004"))
	writeVoucherFile("2025-05", "0005", dummy("2025-05", "0005"))
	// 6 月：银行存款收回 → 冲平宏远公司1
	writeVoucherFile("2025-06", "0006", makeVoucher("2025-06", "0006", "收回往来款", "银行存款", "其他应收款-宏远公司1", "10,030,000.00"))
	// 7 月：dummy（宏远公司1 静置）
	writeVoucherFile("2025-07", "0007", dummy("2025-07", "0007"))
	// 8 月：宏远公司1 重现
	writeVoucherFile("2025-08", "0008", makeVoucher("2025-08", "0008", "垫付款", "其他应收款-宏远公司1", "银行存款", "500.00"))

	// JSON：建账期初 借 10,030,000（宏远公司1）+ 5,000（银行存款）= 贷 10,035,000（实收资本）
	jsonContent := `{
  "全局设置": {
    "启动月": "2025-01",
    "科目顺序": ["库存现金", "银行存款", "其他应收款", "实收资本"],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "银行存款": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 500000}, "余额": {}},
    "其他应收款": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "其他应收款-宏远公司1": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 1003000000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": -1003500000}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2025-01", "期初调整额": 0},
    {"科目": "其他应收款", "首次月份": "2025-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款", "生效月份": "2025-01", "期初调整额": 5000.00, "说明": "建账"},
    {"科目": "其他应收款-宏远公司1", "生效月份": "2025-01", "期初调整额": 10030000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2025-01", "期初调整额": -10035000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	if err := os.WriteFile(filepath.Join(yearDir, "2025.json"), []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}

	gen := func(month string, force bool) {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		cmd := exec.Command(bin, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
	}

	// 断言：宏远公司1 GL sheet 最后一个"期末余额"行的余额（即最近月末运行余额）
	lastFinal := func(month string) float64 {
		f, err := excelize.OpenFile(filepath.Join(yearDir, month+".xlsx"))
		if err != nil {
			t.Fatalf("打开 %s: %v", month, err)
		}
		defer f.Close()
		const sheet = "总分类账-其他应收款-宏远公司1"
		idx, err := f.GetSheetIndex(sheet)
		if err != nil || idx < 0 {
			t.Fatalf("%s 缺少 sheet %q", month, sheet)
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatalf("读取 sheet: %v", err)
		}
		last := -1
		for i, r := range rows {
			for _, c := range r {
				if strings.TrimSpace(c) == "期末余额" {
					last = i
				}
			}
		}
		if last < 0 {
			t.Fatalf("%s 的 %s 无期末余额行", month, sheet)
		}
		return yuanVal(rows, last, 12)
	}

	// 阶段 1：顺序生成 1~8 月
	for _, m := range []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06", "2025-07", "2025-08"} {
		gen(m, false)
	}
	if got := lastFinal("2025-06"); got != 0 {
		t.Errorf("[顺序生成] 6 月冲平后期末应为 0，实际 %v", got)
	}
	if got := lastFinal("2025-08"); got != 500 {
		t.Errorf("[顺序生成] 8 月期末应为 500（期初0+借500），实际 %v —— 期初被重置为建账期初的 bug 复现", got)
	}

	// 科目余额表（期初+发生+期末一体）：冲平科目 8 月行应为 期初空(0)/借500/期末500，父级汇总一致
	subjBalance := func(month, account string) map[string]float64 {
		f, err := excelize.OpenFile(filepath.Join(yearDir, month+".xlsx"))
		if err != nil {
			t.Fatalf("打开 %s: %v", month, err)
		}
		defer f.Close()
		rows, err := f.GetRows("科目余额表")
		if err != nil {
			t.Fatalf("读取科目余额表: %v", err)
		}
		for _, r := range rows {
			if len(r) > 0 && strings.TrimSpace(r[0]) == account {
				val := func(i int) float64 {
					if i >= len(r) || r[i] == "" {
						return 0
					}
					v, err := strconv.ParseFloat(strings.ReplaceAll(r[i], ",", ""), 64)
					if err != nil {
						t.Fatalf("科目余额表 %s %q 列解析失败: %v", account, r[i], err)
					}
					return v
				}
				return map[string]float64{"init": val(2), "debit": val(3), "credit": val(4), "final": val(5)}
			}
		}
		t.Fatalf("%s 科目余额表缺少行 %q", month, account)
		return nil
	}
	for _, acct := range []string{"其他应收款-宏远公司1", "其他应收款"} {
		b := subjBalance("2025-08", acct)
		if b["debit"] != 500 || b["final"] != 500 || b["init"] != 0 {
			t.Errorf("[科目余额表] %s 8月 = %v, want 期初0/借500/期末500", acct, b)
		}
	}

	// 阶段 2：-f 重跑历史月（下游报告的复发路径）——从 6 月级联
	gen("2025-06", true)
	gen("2025-07", true)
	gen("2025-08", true)
	if got := lastFinal("2025-08"); got != 500 {
		t.Errorf("[-f 6→7→8] 8 月期末应为 500，实际 %v —— -f 重跑后复发", got)
	}

	// 阶段 3：单月 -f 8 月（6/7 月 xlsx 不动）
	gen("2025-08", true)
	if got := lastFinal("2025-08"); got != 500 {
		t.Errorf("[-f 仅 8 月] 8 月期末应为 500，实际 %v", got)
	}

	// 阶段 4：-f 重跑静置月 7 月（级联 8 月）
	gen("2025-07", true)
	gen("2025-08", true)
	if got := lastFinal("2025-08"); got != 500 {
		t.Errorf("[-f 7→8] 8 月期末应为 500，实际 %v", got)
	}
}

func fmtYuan(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	intPart := s[:len(s)-3]
	frac := s[len(s)-3:]
	if len(intPart) > 3 {
		var groups []string
		for len(intPart) > 3 {
			groups = append([]string{intPart[len(intPart)-3:]}, groups...)
			intPart = intPart[:len(intPart)-3]
		}
		groups = append([]string{intPart}, groups...)
		intPart = strings.Join(groups, ",")
	}
	return intPart + frac
}

// TestStaleWorkbookJSONAuthority 固化双源冲突修复（2026-09-06，下游 0.9.0 实测 bug）：
// 账本 xlsx 链陈旧（6 月冲平从未进 xlsx，账页最后期末余额停在 5 月的 1,003 万），
// 用户手工把 JSON 权威链修成 0 后，-f 单月重跑 8 月 —— 修复前路径 2（上月 xlsx 链）
// 压过路径 3（JSON 权威链，铁律三），期初被重置回 1,003 万并重新污染 Balances（"复发"）；
// 修复后双源冲突采信 JSON 并告警，-f 重建自愈。
func TestStaleWorkbookJSONAuthority(t *testing.T) {
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

2025年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td></td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td></td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[5:7], summary, debitAcct, amount, summary, creditAcct, amount, amount, amount)
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
	dummy := func(month, num string) string {
		return makeVoucher(month, num, "零星收支", "库存现金", "银行存款", "100.00")
	}
	for i, m := range []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06", "2025-07"} {
		writeVoucherFile(m, fmt.Sprintf("%04d", i+1), dummy(m, fmt.Sprintf("%04d", i+1)))
	}
	// 8 月重现凭证（借 宏远公司1 500）
	writeVoucherFile("2025-08", "0008", makeVoucher("2025-08", "0008", "垫付款", "其他应收款-宏远公司1", "银行存款", "500.00"))

	jsonContent := `{
  "全局设置": {
    "启动月": "2025-01",
    "科目顺序": ["库存现金", "银行存款", "其他应收款", "实收资本"],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "银行存款": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 500000}, "余额": {}},
    "其他应收款": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "其他应收款-宏远公司1": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 1003000000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": -1003500000}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2025-01", "期初调整额": 0},
    {"科目": "其他应收款", "首次月份": "2025-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款", "生效月份": "2025-01", "期初调整额": 5000.00, "说明": "建账"},
    {"科目": "其他应收款-宏远公司1", "生效月份": "2025-01", "期初调整额": 10030000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2025-01", "期初调整额": -10035000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	jsonPath := filepath.Join(yearDir, "2025.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}

	gen := func(month string, force bool) {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		cmd := exec.Command(bin, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
	}

	// 阶段 1：顺序生成 1~7 月（无冲平凭证——xlsx 链停在 1,003 万，与下游"xlsx 从未吃进冲平"一致）
	for _, m := range []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06", "2025-07"} {
		gen(m, false)
	}

	// 阶段 2：模拟下游"修复脚本"——只修 JSON 权威链（179 处修复的等价物），xlsx 不动：
	// 6 月冲平（期末 0）、7 月静置（期末 0）。
	// [期初, 借方, 贷方, 期末]（分）
	handFixJSON(t, jsonPath, map[string][4]int64{
		"2025-06": {1003000000, 0, 1003000000, 0},
		"2025-07": {0, 0, 0, 0},
	})

	// 阶段 3：-f 单月重跑 8 月（下游报告的复发路径）
	gen("2025-08", true)

	f, err := excelize.OpenFile(filepath.Join(yearDir, "2025-08.xlsx"))
	if err != nil {
		t.Fatalf("打开 2025-08: %v", err)
	}
	defer f.Close()
	const sheet = "总分类账-其他应收款-宏远公司1"
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 sheet: %v", err)
	}
	last := -1
	for i, r := range rows {
		for _, c := range r {
			if strings.TrimSpace(c) == "期末余额" {
				last = i
			}
		}
	}
	if last < 0 {
		t.Fatalf("8 月 sheet 无期末余额行")
	}
	if got := yuanVal(rows, last, 12); got != 500 {
		t.Errorf("8 月期末应为 500（JSON 权威链 7 月期末 0 + 借 500），实际 %v —— 双源冲突未采信 JSON（铁律三被违反）", got)
	}
}

// handFixJSON 把指定月份的科目余额记录写进 JSON 科目树（模拟手工修复 Balances 链）。
func handFixJSON(t *testing.T, path string, months map[string][4]int64) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 JSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("解析 JSON: %v", err)
	}
	tree := doc["科目树"].(map[string]any)
	node := tree["其他应收款-宏远公司1"].(map[string]any)
	balances := node["余额"].(map[string]any)
	for month, v := range months {
		balances[month] = map[string]any{
			"期初": v[0], "借方": v[1], "贷方": v[2], "期末": v[3],
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("序列化 JSON: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("写 JSON: %v", err)
	}
}
