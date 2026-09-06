package generator

import (
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"

	"ledger/balance"
)

// TestWriteSubjectBalanceSheet 科目余额表（期初+本月发生+期末一体）核心口径：
// 父级行=子树汇总（合并父级不在 initials 也成行）、贷方科目余额归一为正、
// 全 0 节点不显示、合计只按实际记账科目求和（借贷平衡时无差额提示行）。
func TestWriteSubjectBalanceSheet(t *testing.T) {
	f := excelize.NewFile()
	cfg := &balance.GlobalConfig{Tree: map[string]balance.AccountNode{
		"其他应收款":       {Property: "借"},
		"其他应收款-宏远公司1": {Property: "借"},
		"应付款":         {Property: "贷"},
		"应付款-养老金":     {Property: "贷"},
		"内部往来":        {Property: "借"}, // 合并父级场景：不在 initials，仅树键
		"内部往来-张三":     {Property: "借"},
		"经营收入":        {Property: "贷"},
		// 树里只定义叶子 → 应自动补"补助收入"一级汇总行（真实迁移账套形态）
		"补助收入-村级经费": {Property: "贷"},
	}}
	wb := &Workbook{File: f, Month: "2026-01", Config: cfg}

	initials := map[string]int64{
		"其他应收款-宏远公司1": 500000,  // 借余 5,000 元
		"应付款-养老金":     -300000, // 贷余 3,000 元（借正贷负）
		"内部往来-张三":     100000,  // 借余 1,000 元（挂合并父级下）
		"补助收入-村级经费":   -200000, // 贷余 2,000 元（叶子-only 树）
	}
	activity := map[string]Activity{
		"其他应收款-宏远公司1": {Debit: 50000}, // 本月借 500 元
		"经营收入":        {Credit: 50000},
	}

	if err := wb.writeSubjectBalanceSheet(initials, activity); err != nil {
		t.Fatalf("writeSubjectBalanceSheet: %v", err)
	}

	get := func(sheet, col string, row int) string {
		v, _ := f.GetCellValue(sheet, col+strconv.Itoa(row))
		return v
	}
	findRow := func(name string) int {
		for row := 3; row <= 60; row++ {
			if get("科目余额表", "A", row) == name {
				return row
			}
		}
		return -1
	}
	num := func(name, col string) float64 {
		row := findRow(name)
		if row < 0 {
			t.Fatalf("科目余额表缺少行 %q", name)
		}
		v, _ := strconv.ParseFloat(get("科目余额表", col, row), 64)
		return v
	}

	// 父级=子树汇总（不在 initials 的合并父级同样成行）
	if got := num("其他应收款", "C"); got != 5000 {
		t.Errorf("其他应收款 期初 = %v, want 5000（子科目汇总）", got)
	}
	if got := num("其他应收款", "F"); got != 5500 {
		t.Errorf("其他应收款 期末 = %v, want 5500（期初5000+借500）", got)
	}
	// 叶子行自身值 + 方向列
	if got := num("其他应收款-宏远公司1", "D"); got != 500 {
		t.Errorf("宏远公司1 借方发生额 = %v, want 500", got)
	}
	if dir := get("科目余额表", "B", findRow("其他应收款-宏远公司1")); dir != "借" {
		t.Errorf("方向列 = %q, want 借", dir)
	}
	// 贷方科目余额归一为正（期初 -300000 分 → 显示 3000）
	if got := num("应付款-养老金", "C"); got != 3000 {
		t.Errorf("应付款-养老金 期初 = %v, want 3000（贷方向归一）", got)
	}
	if got := num("应付款", "C"); got != 3000 {
		t.Errorf("应付款 期初 = %v, want 3000（父级汇总+归一）", got)
	}
	// 叶子-only 树的一级汇总行（旧软件"一级科目行=下级合计"）
	if got := num("补助收入", "C"); got != 2000 {
		t.Errorf("补助收入 期初 = %v, want 2000（一级汇总行自动补全）", got)
	}
	if dir := get("科目余额表", "B", findRow("补助收入")); dir != "贷" {
		t.Errorf("补助收入 方向 = %q, want 贷", dir)
	}
	// 树键补全的合并父级行
	if got := num("内部往来", "C"); got != 1000 {
		t.Errorf("内部往来 期初 = %v, want 1000（合并父级子树汇总）", got)
	}
	// 全 0 节点不显示
	if row := findRow("应付款-不存在科目"); row > 0 {
		t.Errorf("全 0 科目不应成行")
	}
	// 合计：按实际记账科目求和，借贷平衡（500/500）无差额提示行
	var totalRow = -1
	for row := 3; row <= 60; row++ {
		if get("科目余额表", "A", row) == "合计" {
			totalRow = row
		}
	}
	if totalRow < 0 {
		t.Fatal("缺少合计行")
	}
	if v, _ := strconv.ParseFloat(get("科目余额表", "D", totalRow), 64); v != 500 {
		t.Errorf("合计借方 = %v, want 500（父子不双算）", v)
	}
	if v, _ := strconv.ParseFloat(get("科目余额表", "E", totalRow), 64); v != 500 {
		t.Errorf("合计贷方 = %v, want 500", v)
	}
	if get("科目余额表", "A", totalRow+1) != "" {
		t.Errorf("借贷平衡时不应有差额提示行，实际 %q", get("科目余额表", "A", totalRow+1))
	}
}
