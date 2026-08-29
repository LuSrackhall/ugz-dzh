package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	// 2. JSON 配置：合并总账科目=["内部往来"]，子科目"内部往来-张三"有历史余额 5000 元
	//    （余额挂在子科目上，与真实账本一致；父级是汇总视图不落余额）。
	//    覆盖点：① 同月只允许一套月结（WriteMonthClosings 不得对合并父级双写）；
	//    ② 新建合并页必须写期初行，分录余额从期初起算（链不断）；
	//    ③ 月结行自带完整边框（无分录月无账页预置框可依赖）。
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
    "内部往来": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-12", "金额": 0}, "余额": {}},
    "内部往来-张三": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-12", "金额": 0}, "余额": {"2026-01": {"期初": 500000, "借方": 0, "贷方": 0, "期末": 500000}}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2026-02", "期初调整额": 0},
    {"科目": "内部往来", "首次月份": "2025-12", "期初调整额": 0},
    {"科目": "内部往来-张三", "首次月份": "2025-12", "期初调整额": 0}
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

	// 4. 断言合并 sheet 只有一套月结 + 期初行 + 余额链连续
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
	initRowIdx, endRowIdx := -1, -1
	for i, r := range rows {
		for j, c := range r {
			c = strings.TrimSpace(c)
			switch c {
			case "本月合计", "本季合计", "本年累计", "期末余额":
				counts[c]++
			case "期初余额", "上年结转":
				counts[c]++
				if j == 6 { // 摘要列（正面区 GetRows 索引）
					initRowIdx = i
				}
			}
			if c == "期末余额" && j == 6 {
				endRowIdx = i
			}
		}
	}
	if got := counts["本月合计"]; got != 1 {
		t.Errorf("合并 sheet 本月合计应出现 1 次（由 WriteMergeGLClosings 写），实际 %d 次 —— 连续月结行 bug 未修复（M4 与 WriteMergeGLClosings 双写）", got)
	}
	if got := counts["期末余额"]; got != 1 {
		t.Errorf("合并 sheet 期末余额应出现 1 次，实际 %d 次", got)
	}
	// D2 固化：新建合并页必须写期初行（余额挂子科目，汇总=5000 借）
	if initRowIdx < 0 {
		t.Fatalf("新建合并页缺少期初/结转行 —— isNew 误判导致期初丢失，分录从 0 起算链断")
	}
	if bal := yuanVal(rows, initRowIdx, 12); bal != 5000 {
		t.Errorf("期初行余额应为 5000 元，实际 %v", bal)
	}
	// 余额链：期末 = 期初 5000 + 本月借 0 - 本月贷 1000 = 4000
	if endRowIdx < 0 {
		t.Fatalf("缺少期末余额行")
	}
	if bal := yuanVal(rows, endRowIdx, 12); bal != 4000 {
		t.Errorf("期末余额应为 4000 元（期初5000-贷1000），实际 %v —— 余额链断裂", bal)
	}
	// D3 固化：月结行自带完整四边框（无分录月无账页预置框可依赖）
	assertCellFramed(t, f, sheet, endRowIdx, 6)  // 摘要列
	assertCellFramed(t, f, sheet, endRowIdx, 12) // 余额列
	t.Logf("合并 sheet 月结标签次数: %v（应各 1 次）；期初行+期末余额链连续", counts)
}

// TestMergeGLInitialOnlyPage 固化"有余额无分录也须建页"（D1，2026-08-30 第二次修复）：
// 合并父级的子科目有期初（Tree.Balances）但当月（跨年 1 月）无分录时，合并账页必须存在：
// 上年结转行 + 一套月结（期末=期初）。4b696b4 将合并父级排除出普通路径后此场景账页整体消失。
func TestMergeGLInitialOnlyPage(t *testing.T) {
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

	// 1. 最小凭证：仅库存现金/经营收入，不涉及合并父级"内部往来"（当月无分录）
	voucherDir := filepath.Join(t.TempDir(), "vouchers")
	if err := os.MkdirAll(voucherDir, 0o755); err != nil {
		t.Fatalf("创建凭证目录: %v", err)
	}
	voucher := `红旗路办事处

记字第0001号 1/1

记帐凭证

2026年01月15日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>收经营款</td><td>库存现金</td><td></td><td>800.00</td><td></td></tr><tr><td>收经营款</td><td>经营收入</td><td></td><td></td><td>800.00</td></tr><tr><td>合计</td><td></td><td></td><td>800.00</td><td>800.00</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`
	if err := os.WriteFile(filepath.Join(voucherDir, "记字第0001号.md"), []byte(voucher), 0o644); err != nil {
		t.Fatalf("写凭证: %v", err)
	}

	// 2. JSON：启动月 2026-01；子科目"内部往来-张三"有 2025-12 期末 5000 元（跨年结转）
	jsonContent := `{
  "全局设置": {
    "启动月": "2026-01",
    "科目顺序": ["库存现金", "内部往来", "经营收入"],
    "科目映射表": {},
    "合并总账科目": ["内部往来"],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": [],
    "结账月": ""
  },
  "科目树": {
    "库存现金": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-01", "金额": 0}, "余额": {}},
    "内部往来": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-12", "金额": 0}, "余额": {}},
    "内部往来-张三": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2025-12", "金额": 0}, "余额": {"2025-12": {"期初": 500000, "借方": 0, "贷方": 0, "期末": 500000}}},
    "经营收入": {"科目属性": "贷", "首次记录": {"方式": "自动识别", "月份": "2026-01", "金额": 0}, "余额": {}}
  },
  "自动识别科目": [
    {"科目": "库存现金", "首次月份": "2026-01", "期初调整额": 0},
    {"科目": "内部往来", "首次月份": "2025-12", "期初调整额": 0},
    {"科目": "内部往来-张三", "首次月份": "2025-12", "期初调整额": 0}
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

	// 4. 断言合并账页存在：上年结转 5000 + 一套月结（期末 5000，当月无发生）
	f, err := excelize.OpenFile(filepath.Join(yearDir, "2026-01.xlsx"))
	if err != nil {
		t.Fatalf("打开 xlsx: %v", err)
	}
	defer f.Close()

	sheet := "总分类账-内部往来"
	if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
		t.Fatalf("合并 sheet %q 不存在 —— 有余额无分录的合并账页被整体删除（D1 回归）", sheet)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 sheet: %v", err)
	}
	initRowIdx, closingCount := -1, 0
	for i, r := range rows {
		for j, c := range r {
			c = strings.TrimSpace(c)
			if j != 6 { // 摘要列
				continue
			}
			switch c {
			case "上年结转", "期初余额":
				initRowIdx = i
			case "本月合计":
				closingCount++
			}
		}
	}
	if initRowIdx < 0 {
		t.Fatalf("合并账页缺少上年结转/期初行（子科目期初合计 5000 应入页）")
	}
	if bal := yuanVal(rows, initRowIdx, 12); bal != 5000 {
		t.Errorf("上年结转行余额应为 5000 元，实际 %v", bal)
	}
	// 无发生额月份不写月结（移除 M4 后的行为）：连续空月结是噪音，已移除
	if closingCount != 0 {
		t.Errorf("无发生额月份不应写月结行，实际本月合计 %d 次", closingCount)
	}
	// 期初/结转行边框必须自包含完整（该页无账页预置框可依赖）
	assertCellFramed(t, f, sheet, initRowIdx, 6)
	assertCellFramed(t, f, sheet, initRowIdx, 12)
	t.Logf("有余额无分录月：合并账页存在，仅上年结转 5000，无空月结")
}

// yuanVal 读 GetRows 指定单元格的金额并解析为元（容逗号/空；解析失败返回 -1）。
func yuanVal(rows [][]string, rowIdx, colIdx int) float64 {
	raw := strings.ReplaceAll(cellVal(rows, rowIdx, colIdx), ",", "")
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return -1
	}
	return v
}

// cellVal 读 GetRows 行的指定索引（越界返回空串）。
func cellVal(rows [][]string, rowIdx, colIdx int) string {
	if rowIdx < 0 || rowIdx >= len(rows) || colIdx >= len(rows[rowIdx]) {
		return ""
	}
	return strings.TrimSpace(rows[rowIdx][colIdx])
}

// assertCellFramed 断言单元格样式含四边边框（GetRows 索引转 Excel 列号 = 索引+1）。
func assertCellFramed(t *testing.T, f *excelize.File, sheet string, rowIdx, colIdx int) {
	t.Helper()
	col, _ := excelize.ColumnNumberToName(colIdx + 1)
	cell := col + itoa(rowIdx+1)
	st, err := f.GetCellStyle(sheet, cell)
	if err != nil || st == 0 {
		t.Fatalf("单元格 %s!%s 无样式", sheet, cell)
	}
	s, err := f.GetStyle(st)
	if err != nil {
		t.Fatalf("读取 %s!%s 样式: %v", sheet, cell, err)
	}
	has := map[string]bool{}
	for _, b := range s.Border {
		if b.Style != 0 {
			has[b.Type] = true
		}
	}
	for _, side := range []string{"left", "right", "top", "bottom"} {
		if !has[side] {
			t.Errorf("%s!%s 缺少 %s 边框 —— 月结行边框不自包含，打印版列线断裂", sheet, cell, side)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestMergeGLPageCrossing 固化"合并账页翻页列区错位"修复（2026-08-30 第七轮）：
// 合并账页分录跨页时，翻页分支此前缺三步（页码不更新/不写新页头/不跳数据首行），
// 导致承前页与后续分录带着旧页码写进旧列区——反面页一部分被写入左侧正面页列数中，
// 且新页无表头，逐月月结视觉上连成一片。
// 构造 25 条分录（首页 20 行满 + 翻页）断言：第 2 页带正面区无任何内容、
// 反面区有页头与承前页、承前页承接过次页余额。
func TestMergeGLPageCrossing(t *testing.T) {
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

	// 1. 25 张凭证：库存现金 借100 / 内部往来-张三 贷100（合并页 25 行分录，必翻页）
	voucherDir := filepath.Join(t.TempDir(), "vouchers")
	if err := os.MkdirAll(voucherDir, 0o755); err != nil {
		t.Fatalf("创建凭证目录: %v", err)
	}
	tpl := `红旗路办事处

记字第%04d号 1/1

记帐凭证

2026年02月%02d日

附件 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody><tr><td>收还款%02d</td><td>库存现金</td><td></td><td>100.00</td><td></td></tr><tr><td>收还款%02d</td><td>内部往来</td><td>张三</td><td></td><td>100.00</td></tr><tr><td>合计</td><td></td><td></td><td>100.00</td><td>100.00</td></tr></tbody></table>

1

会计主管

记帐

审核

制单`
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("记字第%04d号.md", i)
		if err := os.WriteFile(filepath.Join(voucherDir, name), []byte(fmt.Sprintf(tpl, i, i%28+1, i, i)), 0o644); err != nil {
			t.Fatalf("写凭证 %s: %v", name, err)
		}
	}

	// 2. JSON：合并总账科目=["内部往来"]，无历史余额（链从 0 起算）
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
    "内部往来": {"科目属性": "借", "首次记录": {"方式": "自动识别", "月份": "2026-02", "金额": 0}, "余额": {}},
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

	// 4. 结构断言
	f, err := excelize.OpenFile(filepath.Join(yearDir, "2026-02.xlsx"))
	if err != nil {
		t.Fatalf("打开 xlsx: %v", err)
	}
	defer f.Close()

	sheet := "总分类账-内部往来"
	if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
		t.Fatalf("合并 sheet %q 不存在", sheet)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 sheet: %v", err)
	}
	if len(rows) <= 28 {
		t.Fatalf("25 条分录应触发翻页（总行数 %d ≤ 28，未翻页）", len(rows))
	}
	// 第 2 页带（R29-R56）：正面列区（GetRows 索引 0..13）必须无任何内容
	frontContent := 0
	sample := ""
	for i := 28; i < 56 && i < len(rows); i++ {
		for j := 0; j <= 13 && j < len(rows[i]); j++ {
			if strings.TrimSpace(rows[i][j]) != "" {
				frontContent++
				if sample == "" {
					sample = fmt.Sprintf("R%d [%d]%s", i+1, j, strings.TrimSpace(rows[i][j]))
				}
			}
		}
	}
	if frontContent > 0 {
		t.Errorf("第 2 页带正面列区出现 %d 处内容（首处 %s）—— 翻页未更新页码，反面页内容写进正面列", frontContent, sample)
	}
	// 反面区：新页头标题（R30，索引16）+ 承前页（数据区，索引20）
	if cellVal(rows, 29, 16) == "" || !strings.Contains(cellVal(rows, 29, 16), "总") {
		t.Errorf("第 2 页 R30 反面区缺少页头标题（翻页未写新页头）: %q", cellVal(rows, 29, 16))
	}
	carryIdx := -1
	for i := 28; i < len(rows) && i < 56; i++ {
		if cellVal(rows, i, 20) == "承前页" {
			carryIdx = i
			break
		}
	}
	if carryIdx < 0 {
		t.Fatalf("第 2 页反面区（索引20）未找到承前页")
	}
	// 承前页承接过次页余额：20 条 × 100 贷 = 2000（借方向列应为 贷）
	if bal := yuanVal(rows, carryIdx, 26); bal != 2000 {
		t.Errorf("承前页余额应为 2000 元（20×100 贷），实际 %v", bal)
	}
	t.Logf("翻页结构正确：第 2 页反面区页头+承前页 2000，正面列区无错位内容")
}
