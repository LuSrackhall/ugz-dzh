package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestMergeGLNoDoubleClose 固化"连续月结行"bug 修复（2026-08-30）：
// 合并总账父级科目的 sheet（总分类账-{父级}）只能有一套月结（由 WriteMergeGLClosings 写），
// WriteMonthClosings 的 M4 补月结逻辑不得对合并父级写第二套（否则连续两套月结行）。
// 触发条件：合并父级期初≠0（来自科目树 Tree[父级].Balances 历史余额，D1a 未覆盖此路径）。
func TestMergeGLNoDoubleClose(t *testing.T) {
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

	// 1. 最小凭证：库存现金借1000，内部往来-张三贷1000（合并父级"内部往来"有子科目分录）
	voucherDir := filepath.Join(t.TempDir(), "vouchers")
	if err := os.MkdirAll(voucherDir, 0o755); err != nil {
		t.Fatalf("创建凭证目录: %v", err)
	}
	voucher := `红旗路办事处

记字第0001号 1/1

记帐凭证

2026年02月02日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>收还款</td><td>库存现金</td><td></td><td>1,000.00</td><td></td></tr><tr><td>收还款</td><td>内部往来</td><td>张三</td><td></td><td>1,000.00</td></tr><tr><td>合计</td><td></td><td></td><td>1,000.00</td><td>1,000.00</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`
	if err := os.WriteFile(filepath.Join(voucherDir, "记字第0001号.md"), []byte(voucher), 0o644); err != nil {
		t.Fatalf("写凭证: %v", err)
	}

	// 2. JSON 配置：合并总账科目=["内部往来"]，科目树"内部往来"父级有历史余额 5000 元
	//    （模拟旧版本回写/year-close 跨年结转写入父级的余额，让 initials[父级]≠0 触发 M4）
	jsonContent := `{
  "全局设置": {
    "启动月": "2026-02",
    "科目顺序": ["库存现金", "内部往来"],
    "科目映射表": {},
    "合并总账科目": ["内部往来"],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-02", "金额": 0}, "余额": {}},
    "内部往来": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-02", "金额": 0}, "余额": {"2026-01": {"期初": 500000, "借方": 0, "贷方": 0, "期末": 500000}}},
    "内部往来-张三": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-02", "金额": 0}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2026-02", "期初调整额": 0},
    {"科目": "内部往来", "首次月份": "2026-02", "期初调整额": 0},
    {"科目": "内部往来-张三", "首次月份": "2026-02", "期初调整额": 0}
  ],
  "手动调整科目": [],
  "明细列顺序": {}
}`
	if err := os.WriteFile(filepath.Join(yearDir, "2026.json"), []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("写配置: %v", err)
	}

	// 3. 生成
	cmd := exec.Command(bin, "generate", "-v", voucherDir, "-o", output, "-f")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate 失败: %v\n%s", err, out)
	}

	// 4. 断言合并 sheet 只有一套月结
	f, err := excelize.OpenFile(filepath.Join(yearDir, "2026-02.xlsx"))
	if err != nil {
		t.Fatalf("打开 xlsx: %v", err)
	}
	defer f.Close()

	sheet := "总分类账-内部往来"
	idx, err := f.GetSheetIndex(sheet)
	if err != nil || idx < 0 {
		t.Fatalf("合并 sheet %q 不存在", sheet)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 sheet: %v", err)
	}
	// 统计月结四件套标签出现次数（合并 sheet 月结在摘要列，遍历所有单元格）
	counts := map[string]int{}
	for _, r := range rows {
		for _, c := range r {
			c = strings.TrimSpace(c)
			switch c {
			case "本月合计", "本季合计", "本年累计", "期末余额":
				counts[c]++
			}
		}
	}
	if got := counts["本月合计"]; got != 1 {
		t.Errorf("合并 sheet 本月合计应出现 1 次（由 WriteMergeGLClosings 写），实际 %d 次 —— 连续月结行 bug 未修复（M4 与 WriteMergeGLClosings 双写）", got)
	}
	if got := counts["期末余额"]; got != 1 {
		t.Errorf("合并 sheet 期末余额应出现 1 次，实际 %d 次", got)
	}
	// 合并父级期初≠0 时，M4 那套期末=期初（5000）、未计入当月发生额（错误）；修复后仅 WriteMergeGLClosings 一套（含发生额）
	// 期末余额应为 期初5000 + 当月净发生（1000贷→内部往来借方科目余额减少？此处仅断言唯一性，数值正确性由会计审查保证）
	t.Logf("合并 sheet 月结标签次数: %v（应各 1 次）", counts)
}
