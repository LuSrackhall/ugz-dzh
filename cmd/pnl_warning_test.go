package cmd

import (
	"reflect"
	"testing"

	"ledger/balance"
)

// TestUnknownPnlAccounts v2 §2.2：仅列出"类别未知且快照月余额≠0"的科目；
// 已知损益/其他类别、非快照月余额均不列。
// 专项应付款-日间照料中心 在官方 42 科目表补全后已可识别（不再未分类）。
func TestUnknownPnlAccounts(t *testing.T) {
	tree := map[string]balance.AccountNode{
		"库存现金":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 100000}}}, // 已知资产→不列
		"管理费用":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 30000}}},  // 已知费用→不列
		"管埋费用":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: -5000}}},  // 未分类→列出
		"专项应付款-日间照料中心": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 80000}}},  // 官方表补全后已知→不列
		"旧错名科目":        {Balances: map[string]balance.MonthBalance{"2025-11": {Final: 999}}},    // 非快照月→不列
	}
	want := []string{"管埋费用（余额 -50.00 元）"}
	if got := unknownPnlAccounts(tree, "2025-12"); !reflect.DeepEqual(got, want) {
		t.Errorf("unknownPnlAccounts = %v, want %v", got, want)
	}
	if got := unknownPnlAccounts(tree, "2025-11"); len(got) != 1 || got[0] != "旧错名科目（余额 9.99 元）" {
		t.Errorf("unknownPnlAccounts(2025-11) = %v, want 仅旧错名科目", got)
	}
}
