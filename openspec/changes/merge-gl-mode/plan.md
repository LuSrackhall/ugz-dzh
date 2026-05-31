# 合并总分类账模式 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为可配置的父级科目生成合并 GL Sheet，子科目分录归入同一帐页，同时支持忽略叶子 GL / ML 的独立配置。

**Architecture:** 新增 `generator/merge_gl_sheet.go` 作为独立模块，在 `GenerateWorkbook` 流程中 `AppendEntries` 之后插入。`GlobalSettings` 新增三个 `[]string` 字段，三个忽略/合并逻辑完全解耦。

**Tech Stack:** Go, excelize/v2

**Spec:** `docs/superpowers/specs/2026-05-30-merge-gl-design.md`

---

### Task 1: 在 GlobalSettings 中新增三个配置字段

**Files:**
- Modify: `balance/balance.go:26-30`

- [ ] **Step 1: 修改 GlobalSettings 结构体**

在 `GlobalSettings` 结构体中追加三个字段，带 `omitempty` tag：

```go
type GlobalSettings struct {
	StartMonth            string            `json:"启动月"`
	Order                 []string          `json:"科目顺序"`
	AccountMap            map[string]string `json:"科目映射表"`
	MergeGLAccounts       []string          `json:"合并总账科目,omitempty"`
	GLSuppressAccounts    []string          `json:"总分类账忽略科目,omitempty"`
	MLSuppressAccounts    []string          `json:"多科目明细账忽略科目,omitempty"`
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

- [ ] **Step 3: 运行现有测试确保零影响**

```bash
go test ./...
```
Expected: 全部 PASS（无代码引用新字段，不可能引入失败）

- [ ] **Step 4: Commit**

```bash
git add balance/balance.go
git commit -m "feat: add MergeGLAccounts, GLSuppressAccounts, MLSuppressAccounts to GlobalSettings"
```

---

### Task 2: 实现合并 GL 的核心生成方法 AppendMergeEntries

**Files:**
- Create: `generator/merge_gl_sheet.go`

- [ ] **Step 1: 创建文件并实现 AppendMergeEntries**

```go
package generator

import (
	"fmt"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// AppendMergeEntries 为配置中"合并总账科目"指定的父级科目生成合并 GL Sheet。
// 该父级科目下所有子科目分录按发生时间序归入同一帐页，摘要前缀 [子科目名]。
// 此方法纯增量，不影响原有叶子 GL 的生成。
func (wb *Workbook) AppendMergeEntries(entries []voucher.Entry, initials map[string]int64) error {
	if len(wb.Config.Settings.MergeGLAccounts) == 0 {
		return nil
	}

	// 构建合并科目集合
	mergeSet := make(map[string]bool)
	for _, a := range wb.Config.Settings.MergeGLAccounts {
		mergeSet[a] = true
	}

	// 按父级科目分组：只收集有明细科目且 GeneralAccount 在 mergeSet 中的分录
	type mergeGroup struct {
		entries []voucher.Entry
	}
	groups := make(map[string]*mergeGroup)

	for _, e := range entries {
		if !mergeSet[e.GeneralAccount] {
			continue
		}
		// 若分录无明细科目但父级在合并集合中，也纳入（作为直接分录）
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mergeGroup{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
	}

	for general, g := range groups {
		if len(g.entries) == 0 {
			continue
		}
		if err := wb.appendToMergeGLSheet(general, g.entries, initials); err != nil {
			return fmt.Errorf("合并总分类账 %s: %w", general, err)
		}
	}

	return nil
}
```

- [ ] **Step 2: 实现 ensureMergeGLSheet — 确保 Sheet 存在并初始化**

追加在 `AppendMergeEntries` 之后：

```go
// ensureMergeGLSheet 确保合并 GL Sheet 存在并已初始化标题。
func (wb *Workbook) ensureMergeGLSheet(general string) (string, error) {
	name := sheetNameGL(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writeGLTitle(name); err != nil {
		return "", err
	}
	return name, nil
}
```

注意：`sheetNameGL(general)` 返回 `"总分类账-" + general`，而叶子 GL 是 `"总分类账-" + general + "-" + detail`（由 `AppendEntries` 中 `fullPath(general, detail)` 拼出），命名天然不冲突。`writeGLTitle` 复用现有方法。

- [ ] **Step 3: 实现 appendToMergeGLSheet — 数据行追加核心逻辑**

追加在 `ensureMergeGLSheet` 之后：

```go
// appendToMergeGLSheet 将分录追加到指定父级科目的合并 GL Sheet。
// 摘要列格式: [子科目名] 原摘要；余额按父级汇总累计。
func (wb *Workbook) appendToMergeGLSheet(general string, entries []voucher.Entry, initials map[string]int64) error {
	sheet, err := wb.ensureMergeGLSheet(general)
	if err != nil {
		return err
	}

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	// 计算父级期初余额 = 各子科目期初之和
	var parentInitial int64
	for k, v := range initials {
		if isChildOf(k, general) {
			parentInitial += v
		}
	}

	if isNew && parentInitial != 0 {
		if err := wb.insertCarryForward(sheet, parentInitial); err != nil {
			return err
		}
	}

	// 按日期+凭证号排序
	sortEntries(entries)

	balance := parentInitial
	var pageDebit, pageCredit int64
	if !isNew {
		balance = wb.lastPageBalance(sheet)
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		// 补承前页
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		// 页满 → 过次页 + 承前页
		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			wb.writeCarryForwardRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		// 摘要: [子科目] 原摘要
		summary := e.Summary
		if e.DetailAccount != "" {
			summary = fmt.Sprintf("[%s] %s", e.DetailAccount, e.Summary)
		}

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)

		wb.markRowForPrint(sheet, row)
		row++
	}

	return nil
}
```

- [ ] **Step 4: 实现辅助函数 isChildOf 和 sortEntries**

追加在文件末尾：

```go
// isChildOf 判断 account 是否为 parent 的子科目（account 以 "parent-" 开头）。
func isChildOf(account, parent string) bool {
	return len(account) > len(parent) && account[:len(parent)] == parent && account[len(parent)] == '-'
}

// sortEntries 按日期、凭证号排序分录。
func sortEntries(entries []voucher.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})
}
```

注意：`sortEntries` 需要 import `"sort"`。

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add generator/merge_gl_sheet.go
git commit -m "feat: add AppendMergeEntries for merge GL sheet generation"
```

---

### Task 3: 在 AppendEntries 和 AppendMLEntries 中添加过滤逻辑

**Files:**
- Modify: `generator/gl_sheet.go:122-149` (AppendEntries)
- Modify: `generator/ml_sheet.go:342-425` (AppendMLEntries)

- [ ] **Step 1: 在 AppendEntries 中添加 GLSuppressAccounts 过滤**

在 `AppendEntries` 中，分组循环里跳过被忽略父科目的子科目。修改 `gl_sheet.go` 的 `AppendEntries` 函数：

```go
func (wb *Workbook) AppendEntries(entries []voucher.Entry, initials map[string]int64) error {
	type entryGroup struct {
		entries []voucher.Entry
		initial int64
	}
	groups := make(map[string]*entryGroup)

	// 构建忽略集合
	glSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.GLSuppressAccounts {
		glSuppress[a] = true
	}

	for _, e := range entries {
		// 若分录所属父级在总分类账忽略列表中，跳过（不生成叶子 GL）
		if glSuppress[e.GeneralAccount] {
			continue
		}

		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		g, ok := groups[path]
		if !ok {
			g = &entryGroup{initial: initials[path]}
			groups[path] = g
		}
		g.entries = append(g.entries, e)
	}

	for account, g := range groups {
		if err := wb.appendToGLSheet(account, g.entries, g.initial); err != nil {
			return fmt.Errorf("追加科目 %s: %w", account, err)
		}
	}

	return nil
}
```

- [ ] **Step 2: 在 AppendMLEntries 中添加 MLSuppressAccounts 过滤**

修改 `ml_sheet.go` 的 `AppendMLEntries` 函数。在分组循环开头添加过滤：

```go
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	type mlGroup struct {
		entries []voucher.Entry
		details []string
	}
	groups := make(map[string]*mlGroup)

	// 构建忽略集合
	mlSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.MLSuppressAccounts {
		mlSuppress[a] = true
	}

	for _, e := range entries {
		// 若分录所属父级在多科目明细账忽略列表中，跳过
		if mlSuppress[e.GeneralAccount] {
			continue
		}

		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlGroup{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		if e.DetailAccount != "" {
			found := false
			for _, d := range g.details {
				if d == e.DetailAccount {
					found = true
					break
				}
			}
			if !found {
				g.details = append(g.details, e.DetailAccount)
			}
		}
	}

	// ... 后续不变
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add generator/gl_sheet.go generator/ml_sheet.go
git commit -m "feat: add GLSuppressAccounts and MLSuppressAccounts filtering in AppendEntries and AppendMLEntries"
```

---

### Task 4: 为合并 GL 实现月末关账

**Files:**
- Modify: `generator/merge_gl_sheet.go` (追加方法)
- Modify: `generator/generate.go` (接入关账流程)

合并 GL 的关账与现有 GL 月结一致：本月合计、本季合计（仅季末）、本年累计、期末余额。复用现有的 `WriteMonthClosings` 逻辑，但针对合并科目的数据源调整。

- [ ] **Step 1: 在 merge_gl_sheet.go 中实现 WriteMergeGLClosings**

```go
// WriteMergeGLClosings 为所有合并 GL Sheet 追加月结行。
// activity 包含当月各叶子科目的借/贷合计。
// 合并科目的 activity 由其子科目汇总得出。
func (wb *Workbook) WriteMergeGLClosings(activity map[string]Activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit map[string]int64, initials map[string]int64) error {
	if len(wb.Config.Settings.MergeGLAccounts) == 0 {
		return nil
	}

	for _, general := range wb.Config.Settings.MergeGLAccounts {
		sheet := sheetNameGL(general)
		// 若 Sheet 不存在（无分录），跳过
		if idx, err := wb.File.GetSheetIndex(sheet); err != nil || idx < 0 {
			continue
		}

		// 汇总该父级下所有子科目的月度活动
		var mtdDebit, mtdCredit int64
		for k, a := range activity {
			if isChildOf(k, general) {
				mtdDebit += a.Debit
				mtdCredit += a.Credit
			}
		}

		// 也包含父级自身的直接分录（无明细科目的）
		if a, ok := activity[general]; ok {
			mtdDebit += a.Debit
			mtdCredit += a.Credit
		}

		// 汇总期初
		var parentInitial int64
		for k, v := range initials {
			if isChildOf(k, general) {
				parentInitial += v
			}
		}
		parentInitial += initials[general]

		// 汇总本年累计
		var cumDebit, cumCredit int64
		for k := range activity {
			if isChildOf(k, general) {
				cumDebit += ytdDebit[k] + activity[k].Debit
				cumCredit += ytdCredit[k] + activity[k].Credit
			}
		}

		// 汇总本季累计
		var qtDebit, qtCredit int64
		if isQuarterEnd(wb.Month) {
			for k := range activity {
				if isChildOf(k, general) {
					qtDebit += qtdDebit[k] + activity[k].Debit
					qtCredit += qtdCredit[k] + activity[k].Credit
				}
			}
		}

		if err := wb.writeMergeGLClosingRows(sheet, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit, parentInitial); err != nil {
			return fmt.Errorf("合并总分类账 %s 月结: %w", general, err)
		}
	}

	return nil
}

// writeMergeGLClosingRows 写入合并 GL 的四行月结。
func (wb *Workbook) writeMergeGLClosingRows(sheet string, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit int64, parentInitial int64) error {
	row, err := wb.nextDataRowAfterBreak(sheet)
	if err != nil {
		return err
	}

	// 本月合计
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "本月合计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(mtdDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(mtdCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), "")
	wb.File.SetCellValue(sheet, cellName(7, row), "")

	monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), monthlyStyle)
	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	row++

	// 本季合计（仅季末）
	if isQuarterEnd(wb.Month) {
		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本季合计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(qtDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(qtCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")

		qtStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), qtStyle)
		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)
		row++
	}

	// 本年累计
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(cumDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(cumCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), "")
	wb.File.SetCellValue(sheet, cellName(7, row), "")

	cumStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), cumStyle)
	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	row++

	// 期末余额
	endBalance := parentInitial + mtdDebit - mtdCredit
	endDir, endDisp := directionFor(endBalance, 0)

	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), periodEndLabel)
	wb.File.SetCellValue(sheet, cellName(4, row), "")
	wb.File.SetCellValue(sheet, cellName(5, row), "")
	wb.File.SetCellValue(sheet, cellName(6, row), endDir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(endDisp))

	endStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#000000", Style: 2},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), endStyle)
	wb.setMoneyStyle(sheet, row, 7)

	return nil
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add generator/merge_gl_sheet.go
git commit -m "feat: add merge GL month closing (WriteMergeGLClosings)"
```

---

### Task 5: 在 GenerateWorkbook 中接入合并 GL 流程

**Files:**
- Modify: `generator/generate.go:42-44`（在 AppendEntries 和 AppendMLEntries 之间插入）

- [ ] **Step 1: 插入 AppendMergeEntries 调用**

在 `AppendEntries` 之后、`AppendMLEntries` 之前插入：

```go
	// 6. 追加分录到总分类账 Sheet
	if err := wb.AppendEntries(entries, initials); err != nil {
		return fmt.Errorf("追加总分类账: %w", err)
	}

	// 6.1 追加分录到合并总分类账 Sheet（纯增量，不影响原有 GL）
	if err := wb.AppendMergeEntries(entries, initials); err != nil {
		return fmt.Errorf("追加合并总分类账: %w", err)
	}

	// 7. 追加分录到多科目明细账 Sheet
	if err := wb.AppendMLEntries(entries, initials); err != nil {
		return fmt.Errorf("追加多科目明细账: %w", err)
	}
```

- [ ] **Step 2: 插入 WriteMergeGLClosings 调用**

在 `WriteMonthClosings` 之后插入：

```go
	// 9. 月末结账（总分类账）
	if err := wb.WriteMonthClosings(activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit, initials, changedSheets); err != nil {
		return fmt.Errorf("月结: %w", err)
	}

	// 9.05 月末结账（合并总分类账）
	if err := wb.WriteMergeGLClosings(activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit, initials); err != nil {
		return fmt.Errorf("合并总分类账月结: %w", err)
	}

	// 9.1 月末结账（多科目明细账）
	if err := wb.WriteMLMonthClosings(entries, initials, ytdDebit, ytdCredit, qtdDebit, qtdCredit, changedSheets); err != nil {
		return fmt.Errorf("多科目明细账月结: %w", err)
	}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add generator/generate.go
git commit -m "feat: wire AppendMergeEntries and WriteMergeGLClosings into GenerateWorkbook"
```

---

### Task 6: 编写测试

**Files:**
- Create: `generator/merge_gl_sheet_test.go`
- Modify: `generator/generator_test.go`（扩展现有测试）

- [ ] **Step 1: 编写合并 GL 基础测试**

```go
package generator

import (
	"strings"
	"testing"

	"ledger/balance"
	"ledger/voucher"
)

func TestAppendMergeEntries_Basic(t *testing.T) {
	cfg := &balance.GlobalConfig{
		Settings: balance.GlobalSettings{
			StartMonth:      "2026-01",
			MergeGLAccounts: []string{"固定资产"},
		},
	}
	wb := &Workbook{Config: cfg}
	// ... 使用 excelize 新建文件
	// ... 构造 entries
	// ... 调用 AppendMergeEntries
	// ... 验证 Sheet "总分类账-固定资产" 存在
	// ... 验证数据行数和摘要格式
}

func TestAppendMergeEntries_SummaryFormat(t *testing.T) {
	// 验证 [子科目] 原摘要 格式
}

func TestAppendMergeEntries_MultipleDetails(t *testing.T) {
	// 验证多子科目分录，余额跨子科目累计
}

func TestAppendMergeEntries_PageBreak(t *testing.T) {
	// 验证超 20 行后自动分页
}
```

- [ ] **Step 2: 编写过滤测试**

```go
func TestAppendEntries_GLSuppress(t *testing.T) {
	// 配置 GLSuppressAccounts: ["固定资产"]
	// 验证固定资产的叶子 GL sheet 不存在
}

func TestAppendMLEntries_MLSuppress(t *testing.T) {
	// 配置 MLSuppressAccounts: ["固定资产"]
	// 验证固定资产的 ML sheet 不存在
}
```

- [ ] **Step 3: 运行全部测试**

```bash
go test ./generator/... -v
```

- [ ] **Step 4: Commit**

```bash
git add generator/merge_gl_sheet_test.go generator/generator_test.go
git commit -m "test: add merge GL and filtering tests"
```

---

### Task 7: 端到端验证

- [ ] **Step 1: 运行完整测试套件**

```bash
go test ./... -v
```
Expected: 所有测试 PASS，包括原有 14 个 generator 测试不受影响。

- [ ] **Step 2: 实际生成验证**

使用测试配置（含 `合并总账科目`），运行 `go run . generate ...` 生成 xlsx，手动打开验证：
- `总分类账-固定资产` Sheet 存在
- 摘要格式为 `[子科目名] 原摘要`
- 金额格式为 `#,##0.00`
- 有关账行（本月合计等）
- 被忽略的叶子 GL / ML 不存在

- [ ] **Step 3: 回归确认**

不带任何新配置项，运行 `go run . generate ...` 确保输出与改动前一模一样。

---

### Task 8: 更新 tasks.md 标记完成

- [ ] **Step 1: 创建 tasks.md**

创建 `openspec/changes/merge-gl-mode/tasks.md`：

```markdown
## 1. 配置数据结构

- [ ] 1.1 在 GlobalSettings 中新增 MergeGLAccounts、GLSuppressAccounts、MLSuppressAccounts 字段

## 2. 合并 GL 生成

- [ ] 2.1 实现 AppendMergeEntries 和 appendToMergeGLSheet
- [ ] 2.2 实现 ensureMergeGLSheet
- [ ] 2.3 实现 isChildOf 和 sortEntries 辅助函数

## 3. 过滤逻辑

- [ ] 3.1 AppendEntries 中实现 GLSuppressAccounts 过滤
- [ ] 3.2 AppendMLEntries 中实现 MLSuppressAccounts 过滤

## 4. 月结

- [ ] 4.1 实现 WriteMergeGLClosings 和 writeMergeGLClosingRows

## 5. 流程接入

- [ ] 5.1 GenerateWorkbook 中插入 AppendMergeEntries 调用
- [ ] 5.2 GenerateWorkbook 中插入 WriteMergeGLClosings 调用

## 6. 测试

- [ ] 6.1 合并 GL 基础测试
- [ ] 6.2 过滤测试
- [ ] 6.3 回归测试

## 7. 验证

- [ ] 7.1 端到端生成验证
- [ ] 7.2 回归确认
```

- [ ] **Step 2: Commit**

```bash
git add openspec/changes/merge-gl-mode/tasks.md
git commit -m "docs: add merge GL mode tasks.md"
```
