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

// TestGLSuppressLeafSheets 固化 总分类账忽略科目 语义（json-schema.md：父级下子科目不生成叶子 GL Sheet）。
// 下游 v0.9.1 实测：配置父级名或叶子全路径，"总分类账-固定资产-×××" 叶子 sheet 仍全部生成。
// 根因（修复前）：① AppendEntries 只匹配凭证总帐科目段（叶子全路径名单永不命中）；
// ② appendCarryForwardOnly 期初行无抑制（建账月给被忽略科目建 sheet）；
// ③ WriteMonthClosings 月结无抑制（分录被跳过但月结照写，sheet 缺失时硬报错）；
// ④ 存量 sheet 随复制链永久存活，-f 也不清除。
func TestGLSuppressLeafSheets(t *testing.T) {
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

	voucherRoot := t.TempDir()
	// 双行凭证：借行/贷行各带（总帐科目, 明细科目）——真实凭证结构（固定资产=总帐科目，电脑=明细）
	makeVoucher := func(month, num, summary, dGeneral, dDetail, cGeneral, cDetail, amount string) string {
		return fmt.Sprintf(`红旗路办事处

记字第%s号 1/1

记帐凭证

2025年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[5:7], summary, dGeneral, dDetail, amount, summary, cGeneral, cDetail, amount, amount, amount)
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
	// 1 月：被忽略科目有发生（固定资产-电脑 借 500）；2 月：与固定资产无关的 dummy
	writeVoucherFile("2025-01", "0001", makeVoucher("2025-01", "0001", "购电脑", "固定资产", "电脑", "银行存款", "", "500.00"))
	writeVoucherFile("2025-02", "0002", makeVoucher("2025-02", "0002", "零星收支", "库存现金", "", "银行存款", "", "100.00"))

	jsonContent := func(suppress []string) string {
		suppressJSON := "[]"
		if len(suppress) > 0 {
			quoted := make([]string, len(suppress))
			for i, s := range suppress {
				quoted[i] = "\"" + s + "\""
			}
			suppressJSON = "[" + strings.Join(quoted, ", ") + "]"
		}
		return `{
  "全局设置": {
    "启动月": "2025-01",
    "科目顺序": ["库存现金", "银行存款", "固定资产", "实收资本"],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": ` + suppressJSON + `,
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "银行存款": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 500000}, "余额": {}},
    "固定资产-电脑": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 300000}, "余额": {}},
    "固定资产-桌椅": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 100000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": -900000}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2025-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款", "生效月份": "2025-01", "期初调整额": 5000.00, "说明": "建账"},
    {"科目": "固定资产-电脑", "生效月份": "2025-01", "期初调整额": 3000.00, "说明": "建账"},
    {"科目": "固定资产-桌椅", "生效月份": "2025-01", "期初调整额": 1000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2025-01", "期初调整额": -9000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	}

	newCase := func(t *testing.T) (string, string) {
		output := t.TempDir()
		yearDir := filepath.Join(output, "2025")
		if err := os.MkdirAll(yearDir, 0o755); err != nil {
			t.Fatalf("建目录: %v", err)
		}
		return output, filepath.Join(yearDir, "2025.json")
	}
	gen := func(t *testing.T, bin, output, month string, force bool) {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
	}
	sheetExists := func(t *testing.T, path, sheet string) bool {
		f, err := excelize.OpenFile(path)
		if err != nil {
			t.Fatalf("打开 %s: %v", path, err)
		}
		defer f.Close()
		idx, _ := f.GetSheetIndex(sheet)
		return idx >= 0
	}

	// 场景 A：父级名 ["固定资产"] → 两叶子的 GL sheet 均不生成；ML sheet 照常；报表数据仍可见
	t.Run("父级名忽略整棵子树", func(t *testing.T) {
		output, jsonPath := newCase(t)
		if err := os.WriteFile(jsonPath, []byte(jsonContent([]string{"固定资产"})), 0o644); err != nil {
			t.Fatalf("写配置: %v", err)
		}
		gen(t, bin, output, "2025-01", false)
		gen(t, bin, output, "2025-02", false)

		for _, m := range []string{"2025-01", "2025-02"} {
			path := filepath.Join(output, "2025", m+".xlsx")
			for _, leaf := range []string{"总分类账-固定资产-电脑", "总分类账-固定资产-桌椅"} {
				if sheetExists(t, path, leaf) {
					t.Errorf("[%s] 被忽略科目的叶子 sheet %q 不应生成", m, leaf)
				}
			}
			if !sheetExists(t, path, "多科目明细账-固定资产") {
				t.Errorf("[%s] 未配置 ML 忽略，多科目明细账 sheet 应照常生成", m)
			}
		}
		// 忽略的是 sheet 生成，不是数据：期末表仍应列出被忽略科目
		f, err := excelize.OpenFile(filepath.Join(output, "2025", "2025-02.xlsx"))
		if err != nil {
			t.Fatalf("打开期末表: %v", err)
		}
		defer f.Close()
		found := false
		for _, r := range mustRows(t, f, "2025-02期末") {
			if len(r) > 0 && strings.TrimSpace(r[0]) == "固定资产-电脑" {
				found = true
			}
		}
		if !found {
			t.Errorf("期末表应仍包含被忽略科目的余额（忽略的是 sheet 生成，不是数据）")
		}
	})

	// 场景 B：叶子全路径 ["固定资产-电脑"] → 只忽略该叶，桌椅照常（下游"112 项全路径不生效"的靶点）
	t.Run("叶子全路径精确忽略", func(t *testing.T) {
		output, jsonPath := newCase(t)
		if err := os.WriteFile(jsonPath, []byte(jsonContent([]string{"固定资产-电脑"})), 0o644); err != nil {
			t.Fatalf("写配置: %v", err)
		}
		gen(t, bin, output, "2025-01", false)
		path := filepath.Join(output, "2025", "2025-01.xlsx")
		if sheetExists(t, path, "总分类账-固定资产-电脑") {
			t.Errorf("叶子全路径忽略后 sheet 不应生成")
		}
		if !sheetExists(t, path, "总分类账-固定资产-桌椅") {
			t.Errorf("未列入忽略的叶子 sheet 应正常生成")
		}
	})

	// 场景 C：存量收敛——先无忽略生成（sheet 已存在），后加忽略配置 -f 重建，sheet 应被清除
	t.Run("存量sheet随generate清除", func(t *testing.T) {
		output, jsonPath := newCase(t)
		if err := os.WriteFile(jsonPath, []byte(jsonContent(nil)), 0o644); err != nil {
			t.Fatalf("写配置: %v", err)
		}
		gen(t, bin, output, "2025-01", false)
		path := filepath.Join(output, "2025", "2025-01.xlsx")
		if !sheetExists(t, path, "总分类账-固定资产-电脑") {
			t.Fatalf("前置：未配置忽略时 sheet 应存在")
		}
		// 模拟下游：事后补配忽略 → -f 重建当月
		if err := os.WriteFile(jsonPath, []byte(jsonContent([]string{"固定资产"})), 0o644); err != nil {
			t.Fatalf("改配置: %v", err)
		}
		gen(t, bin, output, "2025-01", true)
		if sheetExists(t, path, "总分类账-固定资产-电脑") {
			t.Errorf("-f 重建后存量被忽略 sheet 应被清除（下游主诉场景）")
		}
	})
}

// mustRows 读取 sheet 全部行（测试辅助）。
func mustRows(t *testing.T, f *excelize.File, sheet string) [][]string {
	t.Helper()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 %s: %v", sheet, err)
	}
	return rows
}

// TestGLSuppressEdgeScenarios 红队复核补测（minor-2/5）：
// D 合并父级×忽略科目并存（最高危组合：叶子 GL 抑制、合并视图存活且聚合子科目分录）；
// E 非 -f 自然收敛（生成下月即清除复制链存量）；
// F 重新包含（移出名单后 sheet 从当月重建，期初行=JSON 链余额、链正确起算）；
// G 极端配置（忽略全部叶子，工作簿仍有期初/期末/报表 sheet，不出现零 sheet）。
func TestGLSuppressEdgeScenarios(t *testing.T) {
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

	voucherRoot := t.TempDir()
	makeVoucher := func(month, num, summary, dGeneral, dDetail, cGeneral, cDetail, amount string) string {
		return fmt.Sprintf(`红旗路办事处

记字第%s号 1/1

记帐凭证

2025年%s月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td></tr><tr><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr><tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`,
			num, month[5:7], summary, dGeneral, dDetail, amount, summary, cGeneral, cDetail, amount, amount, amount)
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
	writeVoucherFile("2025-01", "0001", makeVoucher("2025-01", "0001", "购电脑", "固定资产", "电脑", "银行存款", "", "500.00"))
	writeVoucherFile("2025-02", "0002", makeVoucher("2025-02", "0002", "零星收支", "库存现金", "", "银行存款", "", "100.00"))
	writeVoucherFile("2025-03", "0003", makeVoucher("2025-03", "0003", "购桌椅", "固定资产", "电脑", "银行存款", "", "200.00"))

	jsonContent := func(merge []string, suppress []string) string {
		toJSON := func(list []string) string {
			if len(list) == 0 {
				return "[]"
			}
			quoted := make([]string, len(list))
			for i, s := range list {
				quoted[i] = "\"" + s + "\""
			}
			return "[" + strings.Join(quoted, ", ") + "]"
		}
		return `{
  "全局设置": {
    "启动月": "2025-01",
    "科目顺序": ["库存现金", "银行存款", "固定资产", "实收资本"],
    "科目映射表": {},
    "合并总账科目": ` + toJSON(merge) + `,
    "总分类账忽略科目": ` + toJSON(suppress) + `,
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-01", "金额": 0}, "余额": {}},
    "银行存款": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 500000}, "余额": {}},
    "固定资产-电脑": {"科目属性": "借", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": 300000}, "余额": {}},
    "实收资本": {"科目属性": "贷", "首次记录": {"方式": "手动调整", "月份": "2025-01", "金额": -800000}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2025-01", "期初调整额": 0}
  ],
  "手动调整科目": [
    {"科目": "银行存款", "生效月份": "2025-01", "期初调整额": 5000.00, "说明": "建账"},
    {"科目": "固定资产-电脑", "生效月份": "2025-01", "期初调整额": 3000.00, "说明": "建账"},
    {"科目": "实收资本", "生效月份": "2025-01", "期初调整额": -8000.00, "说明": "建账"}
  ],
  "明细列顺序": {}
}`
	}

	gen := func(t *testing.T, output, month string, force bool) {
		args := []string{"generate", "-v", filepath.Join(voucherRoot, month), "-o", output}
		if force {
			args = append(args, "-f")
		}
		if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
			t.Fatalf("generate %s 失败: %v\n%s", month, err, out)
		}
	}
	sheetExists := func(t *testing.T, path, sheet string) bool {
		f, err := excelize.OpenFile(path)
		if err != nil {
			t.Fatalf("打开 %s: %v", path, err)
		}
		defer f.Close()
		idx, _ := f.GetSheetIndex(sheet)
		return idx >= 0
	}
	writeJSON := func(t *testing.T, path string, merge, suppress []string) {
		if err := os.WriteFile(path, []byte(jsonContent(merge, suppress)), 0o644); err != nil {
			t.Fatalf("写配置: %v", err)
		}
	}
	// editSuppress 只改忽略名单字段（读-改-写）——整文件覆写会抹掉 generate 回写的余额链
	// （与 flush_flat 场景 handFixJSON 同一教训）
	editSuppress := func(t *testing.T, path string, suppress []string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读配置: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("解析配置: %v", err)
		}
		gs := doc["全局设置"].(map[string]any)
		if suppress == nil {
			gs["总分类账忽略科目"] = []any{}
		} else {
			items := make([]any, len(suppress))
			for i, s := range suppress {
				items[i] = s
			}
			gs["总分类账忽略科目"] = items
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatalf("序列化: %v", err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Fatalf("写配置: %v", err)
		}
	}

	// 场景 D：合并父级与忽略科目并存
	t.Run("合并父级视图存活并聚合被忽略叶子", func(t *testing.T) {
		output := t.TempDir()
		yearDir := filepath.Join(output, "2025")
		os.MkdirAll(yearDir, 0o755)
		jsonPath := filepath.Join(yearDir, "2025.json")
		writeJSON(t, jsonPath, []string{"固定资产"}, []string{"固定资产"})
		gen(t, output, "2025-01", false)

		path := filepath.Join(yearDir, "2025-01.xlsx")
		for _, leaf := range []string{"总分类账-固定资产-电脑"} {
			if sheetExists(t, path, leaf) {
				t.Errorf("叶子 GL sheet %q 不应生成", leaf)
			}
		}
		if !sheetExists(t, path, "总分类账-固定资产") {
			t.Fatalf("合并父级视图 sheet 应存活（不受忽略约束）")
		}
		// 合并视图应聚合被忽略叶子的分录
		f, err := excelize.OpenFile(path)
		if err != nil {
			t.Fatalf("打开: %v", err)
		}
		defer f.Close()
		foundChild := false
		for _, r := range mustRows(t, f, "总分类账-固定资产") {
			for _, c := range r {
				if strings.Contains(c, "电脑") {
					foundChild = true
				}
			}
		}
		if !foundChild {
			t.Errorf("合并视图应聚合被忽略叶子的分录（未找到子科目'电脑'）")
		}
	})

	// 场景 E：非 -f 自然收敛（生成下月即清除复制链存量）
	t.Run("非f自然收敛", func(t *testing.T) {
		output := t.TempDir()
		yearDir := filepath.Join(output, "2025")
		os.MkdirAll(yearDir, 0o755)
		jsonPath := filepath.Join(yearDir, "2025.json")
		writeJSON(t, jsonPath, nil, nil)
		gen(t, output, "2025-01", false)
		if !sheetExists(t, filepath.Join(yearDir, "2025-01.xlsx"), "总分类账-固定资产-电脑") {
			t.Fatalf("前置：未配置忽略时 sheet 应存在")
		}
		editSuppress(t, jsonPath, []string{"固定资产"})
		gen(t, output, "2025-02", false) // 不带 -f，正常生成下月
		if sheetExists(t, filepath.Join(yearDir, "2025-02.xlsx"), "总分类账-固定资产-电脑") {
			t.Errorf("生成下月即应清除复制链中的存量被忽略 sheet")
		}
	})

	// 场景 F：重新包含——移出名单后 sheet 从当月重建，期初行=JSON 链余额（3000+500=3500），链正确起算
	t.Run("重新包含后链从期初行起算", func(t *testing.T) {
		output := t.TempDir()
		yearDir := filepath.Join(output, "2025")
		os.MkdirAll(yearDir, 0o755)
		jsonPath := filepath.Join(yearDir, "2025.json")
		writeJSON(t, jsonPath, nil, []string{"固定资产"})
		gen(t, output, "2025-01", false)
		gen(t, output, "2025-02", false)
		editSuppress(t, jsonPath, nil) // 移出忽略名单（保住 generate 回写的余额链）
		gen(t, output, "2025-03", false)

		f, err := excelize.OpenFile(filepath.Join(yearDir, "2025-03.xlsx"))
		if err != nil {
			t.Fatalf("打开: %v", err)
		}
		defer f.Close()
		rows := mustRows(t, f, "总分类账-固定资产-电脑")
		var initBal, finalBal float64
		for _, r := range rows {
			for j, c := range r {
				c = strings.TrimSpace(c)
				if c == "期初余额" && len(r) > 12 {
					initBal, _ = strconv.ParseFloat(strings.ReplaceAll(r[12], ",", ""), 64)
				}
				if c == "期末余额" && len(r) > 12 {
					finalBal, _ = strconv.ParseFloat(strings.ReplaceAll(r[12], ",", ""), 64)
				}
				_ = j
			}
		}
		if initBal != 3500 {
			t.Errorf("重建 sheet 期初行 = %v, want 3500（JSON 链：建账 3000 + 1 月借 500）", initBal)
		}
		if finalBal != 3700 {
			t.Errorf("重建 sheet 期末 = %v, want 3700（期初 3500 + 3 月借 200）", finalBal)
		}
	})

	// 场景 G：极端配置——忽略全部叶子，工作簿仍正常（期初/期末/报表 sheet 在）
	t.Run("忽略全部叶子不出现零sheet", func(t *testing.T) {
		output := t.TempDir()
		yearDir := filepath.Join(output, "2025")
		os.MkdirAll(yearDir, 0o755)
		jsonPath := filepath.Join(yearDir, "2025.json")
		writeJSON(t, jsonPath, nil, []string{"库存现金", "银行存款", "固定资产-电脑", "实收资本"})
		gen(t, output, "2025-01", false)

		f, err := excelize.OpenFile(filepath.Join(yearDir, "2025-01.xlsx"))
		if err != nil {
			t.Fatalf("工作簿应可正常打开: %v", err)
		}
		defer f.Close()
		for _, must := range []string{"2025-01期初", "2025-01期末", "科目余额表", "资产负债表"} {
			if idx, _ := f.GetSheetIndex(must); idx < 0 {
				t.Errorf("极端配置下 %q sheet 应仍存在", must)
			}
		}
	})
}
