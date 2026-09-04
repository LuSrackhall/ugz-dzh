package cmd

import (
	"reflect"
	"testing"

	"ledger/balance"
)

// TestUnknownPnlAccounts v2 §2.2：仅列出"类别未知且快照月余额≠0"的科目；
// 余额取 ≤快照月的最近记录——当月无发生的休眠科目（此前月有发生、余额待
// 跨年带入）同样必须被告警（S2 修复：此前按快照月当月记录查，休眠科目漏告警）。
// 专项应付款-日间照料中心 在官方 42 科目表补全后已可识别（不再未分类）。
func TestUnknownPnlAccounts(t *testing.T) {
	tree := map[string]balance.AccountNode{
		"库存现金":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 100000}}}, // 已知资产→不列
		"管理费用":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 30000}}},  // 已知费用→不列
		"管埋费用":         {Balances: map[string]balance.MonthBalance{"2025-12": {Final: -5000}}},  // 未分类→列出
		"专项应付款-日间照料中心": {Balances: map[string]balance.MonthBalance{"2025-12": {Final: 80000}}},  // 官方表补全后已知→不列
		"旧错名科目":        {Balances: map[string]balance.MonthBalance{"2025-11": {Final: 999}}},    // 休眠（此后无发生）→仍列出
		"零余额错名":        {Balances: map[string]balance.MonthBalance{"2025-10": {Final: 0}}},      // 余额 0→不列
	}
	// 排序按字节序：旧(U+65E7) < 管(U+7BA1)
	want := []string{"旧错名科目（余额 9.99 元）", "管埋费用（余额 -50.00 元）"}
	if got := unknownPnlAccounts(tree, "2025-12"); !reflect.DeepEqual(got, want) {
		t.Errorf("unknownPnlAccounts(2025-12) = %v, want %v", got, want)
	}
	want11 := []string{"旧错名科目（余额 9.99 元）"}
	if got := unknownPnlAccounts(tree, "2025-11"); !reflect.DeepEqual(got, want11) {
		t.Errorf("unknownPnlAccounts(2025-11) = %v, want %v", got, want11)
	}
}
