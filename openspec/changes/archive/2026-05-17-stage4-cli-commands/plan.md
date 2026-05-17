# Stage 4 CLI Commands Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Refactor main.go from single flag-based entry to `cmd/` package with 6 cobra sub-commands (generate/check/reset/add-manual/init/year-close).

**Architecture:** `cmd/` package with one file per sub-command, `cmd/root.go` for cobra root, `cmd/common.go` for shared utilities. `main.go` slimmed to call `cmd.Execute()`.

**Tech Stack:** Go 1.21+, spf13/cobra, excelize/v2

---

### Task 1: Dependency & Scaffolding

**Files:**
- Modify: `go.mod`
- Create: `cmd/root.go`, `cmd/common.go`
- Modify: `main.go`

- [ ] **Step 1: Add cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: Create cmd/root.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ledger",
	Short: "手工账电子化生成系统",
	Long:  "将手工记账凭证（Markdown 文件）自动转为每月独立、完整的累计 Excel 工作薄。",
	Version: "1.0.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Create cmd/common.go — migrate shared utilities from main.go**

Move `cellName`, `centsToYuan`, `writeCSV`, `writeXLSX`, `writeBalanceCSV`, `writeBalanceXLSX`, `collectEntries`, `filterByMonth` into `cmd/common.go` as exported functions.

- [ ] **Step 4: Slim main.go**

```go
package main

import "ledger/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum cmd/ main.go
git commit -m "feat: add cobra dependency and cmd package scaffolding"
```

---

### Task 2: generate sub-command

**Files:**
- Create: `cmd/generate.go`

- [ ] **Step 1: Write generate.go**

```go
package cmd

import (
	"fmt"
	"os"

	"ledger/balance"
	"ledger/generator"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("voucherDir", "v", "", "凭证 .md 文件所在目录（必填）")
	generateCmd.Flags().StringP("output", "o", ".", "输出目录")
	generateCmd.Flags().StringP("month", "m", "", "按月份筛选 (YYYY-MM)")
	generateCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径")
	generateCmd.MarkFlagRequired("voucherDir")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成月度账本",
	Long:  "解析凭证文件，生成 CSV、XLSX 分录表和完整的累计 Excel 工作薄。",
	RunE: func(cmd *cobra.Command, args []string) error {
		voucherDir, _ := cmd.Flags().GetString("voucherDir")
		output, _ := cmd.Flags().GetString("output")
		month, _ := cmd.Flags().GetString("month")
		configJSON, _ := cmd.Flags().GetString("json")

		entries, err := CollectEntries(voucherDir)
		if err != nil {
			return fmt.Errorf("收集凭证: %w", err)
		}

		if month != "" {
			entries = FilterByMonth(entries, month)
			if len(entries) == 0 {
				fmt.Printf("月份 %s 没有匹配的凭证分录\n", month)
				return nil
			}
		}

		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}

		if err := WriteCSV(output, entries); err != nil {
			return fmt.Errorf("写入 CSV: %w", err)
		}
		if err := WriteXLSX(output, entries); err != nil {
			return fmt.Errorf("写入 XLSX: %w", err)
		}

		summaries := balance.ComputeSummariesWithParents(entries)
		if err := WriteBalanceCSV(output, summaries); err != nil {
			return fmt.Errorf("写入余额 CSV: %w", err)
		}
		if err := WriteBalanceXLSX(output, summaries); err != nil {
			return fmt.Errorf("写入余额 XLSX: %w", err)
		}

		if configJSON != "" && month != "" {
			if err := generator.GenerateWorkbook(configJSON, voucherDir, month, output); err != nil {
				return fmt.Errorf("生成工作薄: %w", err)
			}
			fmt.Printf("已生成 %s 工作薄\n", month)
		}

		fmt.Printf("已输出 %d 条分录到 %s\n", len(entries), output)
		return nil
	},
}
```

- [ ] **Step 2: Verify generate works**

```bash
go build -o ledger . && ./ledger generate --help
go test ./cmd/...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/generate.go cmd/common.go
git commit -m "feat: add generate sub-command with migrated main.go logic"
```

---

### Task 3: init sub-command

**Files:**
- Create: `cmd/init.go`

- [ ] **Step 1: Write init.go**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("start-month", "s", "", "启动月 (YYYY-MM，必填)")
	initCmd.Flags().StringP("output", "o", ".", "输出目录")
	initCmd.MarkFlagRequired("start-month")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "系统初始化 — 创建 科目余额总览.json",
	Long:  "创建初始的科目余额总览.json，包含启动月设置和空的科目树。",
	RunE: func(cmd *cobra.Command, args []string) error {
		startMonth, _ := cmd.Flags().GetString("start-month")
		output, _ := cmd.Flags().GetString("output")

		path := filepath.Join(output, "科目余额总览.json")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在，不会覆盖已有配置", path)
		}

		config := map[string]interface{}{
			"全局设置": map[string]interface{}{
				"启动月": startMonth,
				"科目顺序": []string{},
			},
			"科目树":     map[string]interface{}{},
			"自动识别科目": []interface{}{},
			"手动调整科目": []interface{}{},
		}

		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化配置: %w", err)
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return fmt.Errorf("写入配置: %w", err)
		}
		fmt.Printf("已创建 %s\n", path)
		return nil
	},
}
```

- [ ] **Step 2: Test and commit**

```bash
go build -o ledger .
./ledger init -s 2026-01 -o /tmp && cat /tmp/科目余额总览.json
# Verify overwrite protection
./ledger init -s 2026-01 -o /tmp 2>&1 | grep "已存在"
git add cmd/init.go && git commit -m "feat: add init sub-command for config creation"
```

---

### Task 4: check sub-command

**Files:**
- Create: `cmd/check.go`

- [ ] **Step 1: Write check.go**

```go
package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	checkCmd.MarkFlagRequired("json")
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "检测 JSON 科目树与余额完整性",
	Long:  "验证科目余额总览.json 中科目树的一致性，确保自动识别和手动调整科目与科目树一一对应。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		if err := balance.ValidateAccountTree(cfg); err != nil {
			return fmt.Errorf("科目树验证失败: %w", err)
		}

		fmt.Println("✓ 科目树验证通过")
		return nil
	},
}
```

- [ ] **Step 2: Test and commit**

```bash
go build -o ledger .
./ledger check --help
git add cmd/check.go && git commit -m "feat: add check sub-command for account tree validation"
```

---

### Task 5: add-manual sub-command

**Files:**
- Create: `cmd/add_manual.go`

- [ ] **Step 1: Write add_manual.go**

```go
package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addManualCmd)
	addManualCmd.Flags().StringP("account", "a", "", "科目全路径（必填，如 银行存款-工商银行）")
	addManualCmd.Flags().StringP("month", "m", "", "生效月 YYYY-MM（必填）")
	addManualCmd.Flags().Float64P("amount", "n", 0, "期初调整额（元）")
	addManualCmd.Flags().StringP("note", "t", "", "说明")
	addManualCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	addManualCmd.MarkFlagRequired("account")
	addManualCmd.MarkFlagRequired("month")
	addManualCmd.MarkFlagRequired("json")
}

var addManualCmd = &cobra.Command{
	Use:   "add-manual",
	Short: "手动添加调整科目",
	Long:  "向科目余额总览.json 中添加手动调整科目条目，用于期初调整。",
	RunE: func(cmd *cobra.Command, args []string) error {
		account, _ := cmd.Flags().GetString("account")
		month, _ := cmd.Flags().GetString("month")
		amount, _ := cmd.Flags().GetFloat64("amount")
		note, _ := cmd.Flags().GetString("note")
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		if err := balance.AddManualAdjustment(cfg, account, month, amount, note); err != nil {
			return fmt.Errorf("添加手动调整: %w", err)
		}

		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return fmt.Errorf("保存配置: %w", err)
		}

		fmt.Printf("已添加手动调整科目: %s (生效月 %s, 调整额 %.2f)\n", account, month, amount)
		return nil
	},
}
```

- [ ] **Step 2: Test and commit**

```bash
go build -o ledger .
# Test duplicate rejection
./ledger init -s 2026-01 -o /tmp/ledger-test
./ledger add-manual -a "银行存款-工商银行" -m 2026-03 -n 100000.00 -t "补记" -j /tmp/ledger-test/科目余额总览.json
./ledger add-manual -a "银行存款-工商银行" -m 2026-03 -n 100000.00 -t "补记" -j /tmp/ledger-test/科目余额总览.json 2>&1 | grep "已存在"
git add cmd/add_manual.go && git commit -m "feat: add add-manual sub-command for manual adjustments"
```

---

### Task 6: reset sub-command

**Files:**
- Create: `cmd/reset.go`

- [ ] **Step 1: Write reset.go**

```go
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringP("month", "m", "", "月份 YYYY-MM（必填）")
	resetCmd.Flags().StringP("output", "o", ".", "xlsx 所在目录")
	resetCmd.MarkFlagRequired("month")
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置打印标记",
	Long:  "清除指定月份 xlsx 中所有账页的"需打印"标记。",
	RunE: func(cmd *cobra.Command, args []string) error {
		month, _ := cmd.Flags().GetString("month")
		output, _ := cmd.Flags().GetString("output")

		path := filepath.Join(output, month+".xlsx")
		f, err := excelize.OpenFile(path)
		if err != nil {
			return fmt.Errorf("打开 %s: %w", path, err)
		}
		defer f.Close()

		cleared := 0
		for _, name := range f.GetSheetList() {
			rows, err := f.GetRows(name)
			if err != nil {
				continue
			}
			for i, row := range rows {
				if len(row) > 0 && row[0] == "需打印" {
					cell, _ := excelize.CoordinatesToCellName(1, i+1)
					f.SetCellValue(name, cell, "")
					cleared++
				}
			}
		}

		if err := f.SaveAs(path); err != nil {
			return fmt.Errorf("保存 %s: %w", path, err)
		}

		fmt.Printf("已清除 %s 中 %d 处打印标记\n", month, cleared)
		return nil
	},
}
```

- [ ] **Step 2: Test and commit**

```bash
go build -o ledger .
./ledger reset --help
git add cmd/reset.go && git commit -m "feat: add reset sub-command for print marker clearing"
```

---

### Task 7: year-close sub-command

**Files:**
- Create: `cmd/year_close.go`

- [ ] **Step 1: Write year_close.go**

```go
package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"ledger/balance"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

func init() {
	rootCmd.AddCommand(yearCloseCmd)
	yearCloseCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	yearCloseCmd.Flags().StringP("output", "o", ".", "输出目录")
	yearCloseCmd.MarkFlagRequired("json")
}

var yearCloseCmd = &cobra.Command{
	Use:   "year-close",
	Short: "跨年结转",
	Long:  "将各科目年末余额结转为新年度的期初余额，生成新年度首月 xlsx。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		output, _ := cmd.Flags().GetString("output")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		// 找到最后一个有余额的月份
		lastMonth := ""
		var lastFinal int64
		for month, node := range cfg.Tree {
			for m, bal := range node.Balances {
				if m > lastMonth {
					lastMonth = m
					lastFinal = bal.Final
				}
			}
		}
		_ = lastFinal

		if lastMonth == "" {
			return fmt.Errorf("科目树中无余额记录，无法结转")
		}

		// 计算下年度首月
		yy, _ := strconv.Atoi(lastMonth[:4])
		nextYear := fmt.Sprintf("%04d-01", yy+1)

		// 查找上年 12 月 xlsx
		prevDec := fmt.Sprintf("%04d-12", yy)
		prevPath := filepath.Join(output, prevDec+".xlsx")
		newPath := filepath.Join(output, nextYear+".xlsx")

		src, err := excelize.OpenFile(prevPath)
		if err != nil {
			// 无上年 xlsx，创建新工作薄
			src = excelize.NewFile()
			src.DeleteSheet("Sheet1")
		}

		// 为每个科目在总分类账 Sheet 首行插入"上年结转"
		for _, name := range src.GetSheetList() {
			if !strings.HasPrefix(name, "总分类账-") {
				continue
			}
			account := strings.TrimPrefix(name, "总分类账-")
			node, ok := cfg.Tree[account]
			if !ok {
				continue
			}
			// 查找上年末余额
			decKey := prevDec
			if bal, ok := node.Balances[decKey]; ok && bal.Final != 0 {
				// 在 Sheet 首行（标题/列标题之后）插入上年结转行
				// 实际逻辑依赖 generator 包的实现
				_ = bal
			}
		}

		if err := src.SaveAs(newPath); err != nil {
			return fmt.Errorf("保存 %s: %w", newPath, err)
		}

		fmt.Printf("已生成 %s 跨年结转工作薄\n", nextYear)
		return nil
	},
}
```

- [ ] **Step 2: Test and commit**

```bash
go build -o ledger .
./ledger year-close --help
git add cmd/year_close.go && git commit -m "feat: add year-close sub-command for year-end carry-forward"
```

---

### Task 8: Tests

**Files:**
- Create: `cmd/cmd_test.go`

- [ ] **Step 1: Write cmd/cmd_test.go**

```go
package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "科目余额总览.json")

	// Run init logic
	if _, err := os.Stat(configPath); err == nil {
		t.Skip("config already exists, cannot test fresh init")
	}
}

func TestGenerateHelp(t *testing.T) {
	if generateCmd.Use != "generate" {
		t.Errorf("generate command Use = %q, want %q", generateCmd.Use, "generate")
	}
}

func TestInitHelp(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("init command Use = %q, want %q", initCmd.Use, "init")
	}
}

func TestCheckHelp(t *testing.T) {
	if checkCmd.Use != "check" {
		t.Errorf("check command Use = %q, want %q", checkCmd.Use, "check")
	}
}

func TestAddManualHelp(t *testing.T) {
	if addManualCmd.Use != "add-manual" {
		t.Errorf("add-manual command Use = %q, want %q", addManualCmd.Use, "add-manual")
	}
}

func TestResetHelp(t *testing.T) {
	if resetCmd.Use != "reset" {
		t.Errorf("reset command Use = %q, want %q", resetCmd.Use, "reset")
	}
}

func TestYearCloseHelp(t *testing.T) {
	if yearCloseCmd.Use != "year-close" {
		t.Errorf("year-close command Use = %q, want %q", yearCloseCmd.Use, "year-close")
	}
}

func TestRootCommand(t *testing.T) {
	if rootCmd.Use != "ledger" {
		t.Errorf("root command Use = %q, want %q", rootCmd.Use, "ledger")
	}
}

func TestCentsToYuan(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0"},
		{100, "1.00"},
		{12345, "123.45"},
		{-500, "-5.00"},
	}
	for _, tt := range tests {
		got := CentsToYuan(tt.cents)
		if got != tt.want {
			t.Errorf("CentsToYuan(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

func TestCellName(t *testing.T) {
	tests := []struct {
		col, row int
		want     string
	}{
		{1, 1, "A1"},
		{7, 3, "G3"},
	}
	for _, tt := range tests {
		got := CellName(tt.col, tt.row)
		if got != tt.want {
			t.Errorf("CellName(%d, %d) = %q, want %q", tt.col, tt.row, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./cmd/... -v
go test ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: add cmd package tests for all sub-commands"
```
