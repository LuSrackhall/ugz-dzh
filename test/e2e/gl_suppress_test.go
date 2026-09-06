package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
