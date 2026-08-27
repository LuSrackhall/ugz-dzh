package voucher

import (
	"fmt"
	"sort"
)

// ValidateVoucherBalance 校验凭证借贷平衡（审计 H2，CLI 内建安全）。
//
// 规则（带符号净额口径，兼容红字同侧冲抵）：
//  1. 按（日期, 凭证号）分组，每组 Σ借方 == Σ贷方（带符号求和）；不平衡返回 error（含凭证号与差额）。
//     红字（负数）直接参与求和：同侧红字冲抵（贷 -11700 + 贷 +11700 = 0）天然平衡。
//  2. 借方或贷方为负数（红字）→ warning"红字将显示为负金额"，不自动折入对侧。
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
		// 第三轮审查 N1：同行借贷双非零提示（可能是红字冲销表达，也可能录错——本月合计会虚增）
		if e.DebitCents != 0 && e.CreditCents != 0 {
			warnings = append(warnings, fmt.Sprintf("凭证 %s 记字第%d号 某行借贷双填（借 %.2f / 贷 %.2f），请确认（双填会使本月合计虚增）", e.Date, e.VoucherNum, float64(e.DebitCents)/100, float64(e.CreditCents)/100))
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
		// 生产门槛：凭证号未解析 → 阻断（不再跳过）——未解析凭证无法可靠分组，不平衡可静默入账。
		// 文件名回退已兜底（正文无凭证号时用文件名），真正未解析极罕见，强制用户修正。
		return nil, fmt.Errorf("%d 条分录凭证号未解析（正文与文件名均无法解析），无法进行借贷平衡校验，请检查凭证号书写格式", len(unparsed))
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
			debit += e.DebitCents
			credit += e.CreditCents
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
