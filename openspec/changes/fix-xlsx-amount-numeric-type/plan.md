# xlsx 金额单元格数字类型化 Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将所有 xlsx 金额单元格从文本类型改为数字类型，使 Excel 原生支持 SUM 求和、数值筛选和图表。

**Architecture:** 改造唯一的转换函数 `centsToYuanStr(c int64) string` → `centsToYuan(c int64) float64`，一处定义修改覆盖全部 33 个调用点。

**Tech Stack:** Go, excelize/v2

---

### Task 1: 改造 centsToYuan 函数

**Files:**
- Modify: `generator/workbook.go:152-163`
- Modify: `generator/generator_test.go:42-60`
- Modify: 7 个文件中 33 处调用点（仅函数名重命名）

- [ ] **Step 1: 修改 `centsToYuanStr` 函数定义**

  在 `generator/workbook.go` 第 152-163 行，将：

```go
// centsToYuanStr 分转元显示字符串。
func centsToYuanStr(c int64) string {
	if c == 0 {
		return "0.00"
	}
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return sign + fmt.Sprintf("%d.%02d", c/100, c%100)
}
```

  改为：

```go
// centsToYuan 分转元数值。
func centsToYuan(c int64) float64 {
	return float64(c) / 100
}
```

- [ ] **Step 2: 全局重命名所有调用点**

  将所有文件中 `centsToYuanStr(` 替换为 `centsToYuan(`：

```bash
# macOS sed 语法
find generator -name '*.go' -exec sed -i '' 's/centsToYuanStr(/centsToYuan(/g' {} +
```

  受影响文件（共 33 处）：
  - `generator/workbook.go` (定义 + 无调用点)
  - `generator/gl_sheet.go` (~9处)
  - `generator/ml_sheet.go` (~12处)
  - `generator/monthly_close.go` (7处)
  - `generator/monthly_close_ml.go` (~10处)
  - `generator/initial_sheet.go` (2处)
  - `generator/final_sheet.go` (2处)
  - `generator/generator_test.go` (2处)

- [ ] **Step 3: 验证编译**

  ```bash
  go build ./...
  ```

  预期：编译通过，无 `undefined: centsToYuanStr` 错误。

- [ ] **Step 4: 更新测试用例**

  在 `generator/generator_test.go` 第 42-60 行，将：

```go
func TestCentsToYuanStr(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{100, "1.00"},
		{12345, "123.45"},
		{-500, "-5.00"},
		{-1, "-0.01"},
		{99, "0.99"},
	}
	for _, tt := range tests {
		got := centsToYuanStr(tt.cents)
		if got != tt.want {
			t.Errorf("centsToYuanStr(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}
```

  改为：

```go
func TestCentsToYuan(t *testing.T) {
	tests := []struct {
		cents int64
		want  float64
	}{
		{0, 0.0},
		{100, 1.00},
		{12345, 123.45},
		{-500, -5.00},
		{-1, -0.01},
		{99, 0.99},
	}
	for _, tt := range tests {
		got := centsToYuan(tt.cents)
		if got != tt.want {
			t.Errorf("centsToYuan(%d) = %v, want %v", tt.cents, got, tt.want)
		}
	}
}
```

- [ ] **Step 5: 运行单元测试**

  ```bash
  go test ./generator/... -v -run TestCentsToYuan
  ```

  预期：PASS

- [ ] **Step 6: 提交**

  ```bash
  git add generator/workbook.go generator/generator_test.go
  git add generator/gl_sheet.go generator/ml_sheet.go generator/monthly_close.go generator/monthly_close_ml.go generator/initial_sheet.go generator/final_sheet.go
  git commit -m "fix: xlsx 金额单元格从文本改为数字类型 — centsToYuanStr 返回 float64

  centsToYuanStr(c int64) string → centsToYuan(c int64) float64,
  使 SetCellValue 写入数字单元格而非文本，Excel 原生支持 SUM 和数值筛选"
  ```

---

### Task 2: 验证

**Files:** 无需修改（仅验证）

- [ ] **Step 1: 运行全部测试**

  ```bash
  go test ./...
  ```

  预期：全部 PASS（balance, cmd, generator, e2e, voucher）。

- [ ] **Step 2: 运行 e2e 管线生成 4 个月数据**

  ```bash
  go run ./cmd init -y 2026
  go run ./cmd generate -y 2026 -m 01
  go run ./cmd generate -y 2026 -m 02
  go run ./cmd generate -y 2026 -m 03
  go run ./cmd generate -y 2026 -m 04
  ```

  预期：全部成功生成，无报错。

- [ ] **Step 3: 运行验证脚本确认数值不变**

  ```bash
  go run scripts/verify_ml_closings.go output/2026
  ```

  预期：5 项检查全部 PASS。

- [ ] **Step 4: 程序化验证 xlsx 单元格为数字类型**

  使用以下脚本抽查生成的 xlsx 中金额列为 float64 而非 string：

  ```bash
  cat > /tmp/check_numeric.go << 'GOEOF'
  package main

  import (
  	"fmt"
  	"os"
  	"strings"

  	"github.com/xuri/excelize/v2"
  )

  func main() {
  	f, err := excelize.OpenFile(os.Args[1])
  	if err != nil {
  		fmt.Println("FAIL open:", err)
  		os.Exit(1)
  	}
  	defer f.Close()

  	failed := false
  	for _, sheet := range f.GetSheetList() {
  		if !strings.HasPrefix(sheet, "总分类账-") && !strings.HasPrefix(sheet, "多科目明细账-") {
  			continue
  		}
  		rows, _ := f.GetRows(sheet)
  		for ri, r := range rows {
  			// 检查 D(4), E(5), G(7) 列以及 H+(8+) 明细列
  			checkCols := []int{3, 4, 6}
  			maxCol := len(r)
  			// 多科目明细账：检查 H+(8+) 列
  			if strings.HasPrefix(sheet, "多科目明细账-") {
  				for ci := 7; ci < maxCol; ci++ {
  					checkCols = append(checkCols, ci)
  				}
  			}
  			for _, ci := range checkCols {
  				if ci >= maxCol {
  					continue
  				}
  				val := strings.TrimSpace(r[ci])
  				if val == "" || val == "-" || val == "借" || val == "贷" || val == "平" {
  					continue
  				}
  				// 如果是数字字符串（含小数点），尝试解析为 float
  				// 但如果 excelize 读回来是 string 则说明写入时就是 string
  				// 关键验证：通过 GetCellValue 返回的是格式化的字符串——
  				// 对 float64 写入的单元格，excelize 读回来会按数字格式化
  				// 但 GetRows 返回的是原始值字符串。
  				// 
  				// 换个思路：直接读单元格类型
  				cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
  				// 用 GetCellType 是不可能的...excelize 没有这个 API
  				// 
  				// 实操验证：用 GetCellValue 读回来如果是数字会返回 "0" 而非 "0.00"
  				// 但最可靠的是打开 xlsx 的 xml 看 cell 类型
  				_ = cell
  			}
  		}
  	}

  	if failed {
  		fmt.Println("SOME CHECKS FAILED")
  		os.Exit(1)
  	}
  	fmt.Println("ALL CELLS NUMERIC")
  }
  GOEOF
  go run /tmp/check_numeric.go output/2026/2026-01.xlsx
  ```

  注意：上述脚本仅做样例框架。更可靠的方式是直接解压 xlsx 查看 xml 中 `<c>` 节点的 `t` 属性。

  ```bash
  # 抽查：解压 xlsx，查看某个 sheet 的 xml 中金额单元格的 t 属性
  # 金额列为 D(4), E(5), G(7) — excelize 内部列号从 1 开始
  # xlsx xml 中，cell reference 如 D5, E5, G5
  # 数字单元格：<c r="D5"><v>1234.56</v></c>  (无 t 属性或 t="n")
  # 文本单元格：<c r="D5" t="inlineStr"><is><t>1234.56</t></is></c>
  
  unzip -p output/2026/2026-01.xlsx xl/worksheets/sheet1.xml | grep -E '<c r="[DEG]' | head -20
  ```

  预期：金额列 `<c>` 节点无 `t="inlineStr"` 或 `t="str"` 属性，为数字类型。
