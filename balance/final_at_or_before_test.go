package balance

import "testing"

// TestFinalAtOrBefore ≤month 最近记录回退（S2：休眠科目告警/统计口径，
// 与 GetInitBalanceForGenerate 的"最近月份期末"语义对齐）。
func TestFinalAtOrBefore(t *testing.T) {
	node := AccountNode{Balances: map[string]MonthBalance{
		"2025-10": {Final: 111},
		"2025-12": {Final: 333},
	}}
	if v, m, ok := FinalAtOrBefore(node, "2026-01"); !ok || v != 333 || m != "2025-12" {
		t.Errorf("FinalAtOrBefore(2026-01) = %d/%s/%v, want 333/2025-12/true", v, m, ok)
	}
	if v, m, ok := FinalAtOrBefore(node, "2025-11"); !ok || v != 111 || m != "2025-10" {
		t.Errorf("FinalAtOrBefore(2025-11) = %d/%s/%v, want 111/2025-10/true", v, m, ok)
	}
	if v, m, ok := FinalAtOrBefore(node, "2025-10"); !ok || v != 111 || m != "2025-10" {
		t.Errorf("FinalAtOrBefore(2025-10) = %d/%s/%v, want 111/2025-10/true", v, m, ok)
	}
	if _, _, ok := FinalAtOrBefore(node, "2025-09"); ok {
		t.Error("无 ≤month 记录应返回 false")
	}
	if _, _, ok := FinalAtOrBefore(AccountNode{}, "2026-01"); ok {
		t.Error("空余额表应返回 false")
	}
}
