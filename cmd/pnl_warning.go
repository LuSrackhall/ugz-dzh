package cmd

import (
	"fmt"
	"sort"
	"strings"

	"ledger/balance"
)

// unknownPnlAccounts 收集"类别未知（未分类）且期末余额≠0"的科目
// （docs/account-code-design.md v2 §2.2：此前 gen_close/year-close 对此类科目
// 静默跳过——不结转、不告警、跨年带入新账。现显式告警，不改变结转行为本身）。
// 余额取 ≤month 的最近记录（对齐 GetInitBalanceForGenerate 回退语义）——
// 当月无发生的休眠科目（此前月有发生、余额待结转带入）同样可见。
// month 为快照月（YYYY-MM）。
func unknownPnlAccounts(tree map[string]balance.AccountNode, month string) []string {
	var out []string
	for account, node := range tree {
		bal, _, has := balance.FinalAtOrBefore(node, month)
		if !has || bal == 0 {
			continue
		}
		gen := account
		if idx := strings.IndexByte(gen, '-'); idx > 0 {
			gen = gen[:idx]
		}
		if _, known := balance.AccountTypeOf(gen); !known {
			out = append(out, fmt.Sprintf("%s（余额 %.2f 元）", account, float64(bal)/100))
		}
	}
	sort.Strings(out)
	return out
}

// printUnknownPnlWarning 打印类别未知科目余额告警清单（无清单则静默）。
func printUnknownPnlWarning(unknown []string) {
	if len(unknown) == 0 {
		return
	}
	fmt.Println("⚠ 类别未知（未分类）科目余额非 0，不会参与损益结转，将跨年带入新账——请补科目属性（subjects import）或纠正科目名（map）:")
	for _, u := range unknown {
		fmt.Println("  - " + u)
	}
}
