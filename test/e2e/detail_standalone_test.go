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

// TestDetailStandaloneLedgerFlow 明细科目独立账页全流程（明细账独立科目）：
// 建账（银行存款-工行 100,000 / 实收资本 100,000）→ 1 月 名单明细 办公费 300 +
// 非名单明细 差旅费 200 → generate → 断言：
//   - 独立页 明细账-管理费用-办公费 存在且分录/月结/余额链正确；
//   - GL 叶子页 照常（互不影响，加法路由）；
//   - 合并 ML 页 办公费列收缩、差旅费列保留；
//   - JSON 余额链照常回写（拆的是页不是账）；
//   - sheet 排序：明细账页在 多科目明细账 区块之前；
//   - 打印版同步生成独立页（GL 变换路径）。
//
// → 2 月续写（期初 300 + 100 = 400，跨月链连续）。
func TestDetailStandaloneLedgerFlow(t *testing.T) {
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
	yearDir := filepath.Join(output, "2026")
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		t.Fatalf("创建输出目录: %v", err)
	}
	voucherRoot := t.TempDir()

	makeVoucher := func(month, num, summary, debitAcct, debitDetail, creditAcct, creditDetail, amount string) string {
		return fmt.Sprintf(`红旗路办事处

记字第%s号 1/1

记帐凭证

%s年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[:4], month[5:7], summary, debitAcct, debitDetail, amount, summary, creditAcct, creditDetail, amount, amount, amount)
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

	writeVoucherFile("2026-01", "0001", makeVoucher("2026-01", "0001", "买办公耗材", "管理费用", "办公费", "银行存款", "工行", "300.00"))
	writeVoucherFile("2026-01", "0002", makeVoucher("2026-01", "0002", "出差报销", "管理费用", "差旅费", "银行存款", "工行", "200.00"))
	writeVoucherFile("2026-02", "0003", makeVoucher("2026-02", "0003", "再买耗材", "管理费用", "办公费", "银行存款", "工行", "100.00"))

	jsonContent := `{
  "全局设置": {
    "启动月": "2026-01",
    "科目顺序": ["银行存款", "实收资本", "管理费用"],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "明细账独立科目": ["管理费用-办公费"],
    "结账月": ""
  },
  "科目树": {
    "银行存款-工行": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2026-01", "金额": 10000000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2026-01", "金额": -10000000}, "余额": {}},
    "管理费用-办公费": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-01", "金额": 0}, "余额": {}},
    "管理费用-差旅费": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-01", "金额": 0}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "管理费用-办公费", "首次月份": "2026-01", "期初调整额": 0},
    {"科目": "管理费用-差旅费", "首次月份": "2026-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款-工行", "生效月份": "2026-01", "期初调整额": 100000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2026-01", "期初调整额": -100000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	jsonPath := filepath.Join(yearDir, "2026.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}

	gen := func(month string, force bool) string {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
		return string(out)
	}

	// ── 1 月生成 ──
	gen("2026-01", false)

	f, err := excelize.OpenFile(filepath.Join(yearDir, "2026-01.xlsx"))
	if err != nil {
		t.Fatalf("打开 xlsx: %v", err)
	}
	defer f.Close()
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

	// 1) 独立页在；GL 叶子页照常（互不影响）
	const detailSheet = "明细账-管理费用-办公费"
	if !has(detailSheet) {
		t.Fatalf("独立明细账页 %s 缺失, sheets=%v", detailSheet, sheets)
	}
	if !has("总分类账-管理费用-办公费") {
		t.Errorf("GL 叶子页必须照常存在（加法路由）, sheets=%v", sheets)
	}

	// 2) 合并 ML 列收缩：办公费列不在、差旅费列在
	mlSheet := "多科目明细账-管理费用"
	if !has(mlSheet) {
		t.Fatalf("合并 ML 页缺失, sheets=%v", sheets)
	}
	mlRows, _ := f.GetRows(mlSheet)
	joined := ""
	for _, r := range mlRows {
		joined += strings.Join(r, "|") + "\n"
	}
	if strings.Contains(joined, "\t办公费") || hasMLColumnLabel(mlRows, "办公费") {
		t.Errorf("合并 ML 页不应含名单明细列 办公费")
	}
	if !hasMLColumnLabel(mlRows, "差旅费") {
		t.Errorf("合并 ML 页应保留非名单列 差旅费")
	}

	// 3) 独立页分录 + 月结：分录 300；本月合计 300；期末余额 300（借）
	// ML 列位（GetRows 0-based）：BindingLeftCols(1)+mlOffSummary(4)=5、+mlOffBalance(8)=9
	dRows, _ := f.GetRows(detailSheet)
	labels := []string{}
	for _, r := range dRows {
		if len(r) > 5 && r[5] != "" {
			labels = append(labels, strings.TrimSpace(r[5]))
		}
	}
	joinLabels := strings.Join(labels, "|")
	if !strings.Contains(joinLabels, "买办公耗材") {
		t.Errorf("独立页缺分录摘要, labels=%v", labels)
	}
	if !strings.Contains(joinLabels, "本月合计") || !strings.Contains(joinLabels, "期末余额") {
		t.Errorf("独立页缺月结行, labels=%v", labels)
	}
	endBal := lastSignedBalance(f, detailSheet, "期末余额")
	if parseYuan(endBal) != 300 {
		t.Errorf("独立页期末余额 = %q, want 300", endBal)
	}

	// 4) sheet 排序：明细账独立页在 ML 区块之后（ML 家族分离形态殿后）
	if !(idx(detailSheet) > idx(mlSheet)) {
		t.Errorf("排序错误: 独立页(%d) 应在合并 ML 页(%d) 之后", idx(detailSheet), idx(mlSheet))
	}

	// 5) 打印版同步生成独立页
	pf, err := excelize.OpenFile(filepath.Join(yearDir, "print", "2026-01.xlsx"))
	if err != nil {
		t.Fatalf("打开打印版: %v", err)
	}
	defer pf.Close()
	if pidx, err := pf.GetSheetIndex(detailSheet); err != nil || pidx < 0 {
		t.Errorf("打印版缺独立明细账页 %s", detailSheet)
	}
	pf.Close()

	// 6) JSON 余额链照常（拆的是页不是账）：办公费 2026-01 期末 = 30,000 分（300 元）
	assertTreeField(t, jsonPath, "管理费用-办公费", "2026-01", "期末", 30000)

	// ── 2 月续写：期初 300 + 100 = 400（跨月链连续）──
	gen("2026-02", false)

	f2, err := excelize.OpenFile(filepath.Join(yearDir, "2026-02.xlsx"))
	if err != nil {
		t.Fatalf("打开 2 月 xlsx: %v", err)
	}
	defer f2.Close()
	if !f2HasSheet(f2, detailSheet) {
		t.Fatalf("2 月复制工作簿后独立页缺失")
	}
	endBal2 := lastSignedBalance(f2, detailSheet, "期末余额")
	if parseYuan(endBal2) != 400 {
		t.Errorf("2 月独立页期末余额 = %q, want 400（期初 300 + 当月 100）", endBal2)
	}
	assertTreeField(t, filepath.Join(yearDir, "2026.json"), "管理费用-办公费", "2026-02", "期末", 40000)
}

// ── 测试助手 ──

func hasMLColumnLabel(rows [][]string, label string) bool {
	for _, r := range rows {
		for _, c := range r {
			if strings.TrimSpace(c) == label {
				return true
			}
		}
	}
	return false
}

// lastSignedBalance 从 sheet 最后一个 labelRow 行读余额列（Back 区，ML 列位）。
func lastSignedBalance(f *excelize.File, sheet, label string) string {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return ""
	}
	// ML 布局：BindingLeftCols=1，摘要 offset 4 → GetRows idx 5；余额 offset 8 → idx 9
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 5 && strings.TrimSpace(rows[i][5]) == label {
			if len(rows[i]) > 9 {
				return strings.TrimSpace(rows[i][9])
			}
			return ""
		}
	}
	return ""
}

func f2HasSheet(f *excelize.File, name string) bool {
	idx, err := f.GetSheetIndex(name)
	return err == nil && idx >= 0
}

// parseYuan 宽松解析金额（"300" / "400.00" / "1,234.56" 均可；excelize 保存前后
// GetCellValue 对数字格式的返回值不稳定，比较须走数值）。
func parseYuan(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return -1e18 // 解析失败 → 必然不等
	}
	return v
}

func assertTreeField(t *testing.T, jsonPath, account, month, field string, want float64) {
	t.Helper()
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
		t.Fatalf("%s 缺少 %s 余额记录", account, month)
	}
	got, _ := mb[field].(float64)
	if got != want {
		t.Errorf("%s %s %s = %v, want %v", account, month, field, got, want)
	}
}
