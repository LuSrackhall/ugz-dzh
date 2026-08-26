package voucher

import (
	"fmt"
	"sort"
)

// ValidateVoucherBalance 校验凭证借贷平衡（审计 H2，CLI 内建安全）。
//
// 规则（绝对值口径，兼容红字）：
//  1. 按（日期, 凭证号）分组，每组 Σ|借方| == Σ|贷方|；不平衡返回 error（含凭证号与差额）。
//  2. 借方或贷方为负数（红字）→ warning"红字将显示为负金额"，不自动折入对侧（折入会破坏绝对值平衡）。
//  3. VoucherNum<=0 的条目无法可靠分组，跳过分组校验并 warning。
//
// entries 不会被修改。
func ValidateVoucherBalance(entries []Entry) (warnings []string, err error) {
	// 红字提示
	for _, e := range entries {
		if e.DebitCents < 0 {
			warnings = append(warnings, fmt.Sprintf("凭证 %s 记字第%d号 借方为负数（红字）%.2f 元，将显示为负金额", e.Date, e.VoucherNum, float64(-e.DebitCents)/100))
		}
		if e.CreditCents < 0 {
			warnings = append(warnings, fmt.Sprintf("凭证 %s 记字第%d号 贷方为负数（红字）%.2f 元，将显示为负金额", e.Date, e.VoucherNum, float64(-e.CreditCents)/100))
		}
	}

	// 按（日期, 凭证号）分组
	type groupKey struct {
		date string
		num  int
	}
	groups := make(map[groupKey][]Entry)
	var unparsed []Entry
	for _, e := range entries {
		if e.VoucherNum <= 0 {
			unparsed = append(unparsed, e)
			continue
		}
		key := groupKey{date: e.Date, num: e.VoucherNum}
		groups[key] = append(groups[key], e)
	}

	if len(unparsed) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d 条分录凭证号未解析，跳过借贷平衡校验（请检查凭证号书写格式）", len(unparsed)))
	}

	// 逐组校验（凭证号排序输出，便于定位）
	keys := make([]groupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].date != keys[j].date {
			return keys[i].date < keys[j].date
		}
		return keys[i].num < keys[j].num
	})

	for _, k := range keys {
		var debit, credit int64
		for _, e := range groups[k] {
			d, c := e.DebitCents, e.CreditCents
			if d < 0 {
				d = -d
			}
			if c < 0 {
				c = -c
			}
			debit += d
			credit += c
		}
		if debit != credit {
			diff := debit - credit
			if diff < 0 {
				diff = -diff
			}
			return nil, fmt.Errorf("凭证 %s 记字第%d号 借贷不平衡：借方合计 %.2f 元，贷方合计 %.2f 元，差额 %.2f 元",
				k.date, k.num, float64(debit)/100, float64(credit)/100, float64(diff)/100)
		}
	}

	return warnings, nil
}
