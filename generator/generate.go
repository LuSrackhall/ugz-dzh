package generator

import (
	"fmt"
	"os"
	"strings"

	"ledger/balance"
	"ledger/voucher"
)

// GenerateWorkbook 是 xlsx 生成的唯一入口，按序执行完整生成流程。
// entries 应已经过同年同月校验和科目映射替换。
func GenerateWorkbook(configPath, month, outputDir string, entries []voucher.Entry) error {
	if len(entries) == 0 {
		return fmt.Errorf("月份 %s 没有匹配的凭证分录", month)
	}

	// 创建或复制上月工作薄（内部加载配置到 wb.Config）
	wb, err := NewWorkbook(configPath, month, outputDir)
	if err != nil {
		return fmt.Errorf("创建工作薄: %w", err)
	}
	cfg := wb.Config

	// 期初机制修复：清理历史幻影期初（幂等，自动科目 FirstRecord 置 0 + 删除回填记录）
	balance.PurgePhantomInitials(cfg)

	// 第三轮审查 D1a：合并总账科目禁止直接记账（无明细分录）——
	// 否则 GL 与 MergeGL 共用同名 sheet 月结两遍、期初被合并视图污染。
	for _, general := range cfg.Settings.MergeGLAccounts {
		for _, e := range entries {
			if e.GeneralAccount == general && e.DetailAccount == "" {
				return fmt.Errorf("科目 %s 配置为合并总账科目，不能直接记账，请使用子科目（如 %s-明细）", general, general)
			}
		}
		// D1a 扩展（子 agent 验收发现）：合并父级禁止设置期初调整额——
		// 否则 appendCarryForwardOnly 会为父级创建同名 sheet（结转行），与合并月结写两个月结块。
		for _, m := range cfg.ManualItems {
			if m.Account == general && m.Adjustment != 0 {
				return fmt.Errorf("科目 %s 配置为合并总账科目，不能设置期初调整额，请设置到子科目（如 %s-明细）", general, general)
			}
		}
		for _, a := range cfg.AutoItems {
			if a.Account == general && a.Adjustment != 0 {
				return fmt.Errorf("科目 %s 配置为合并总账科目，不能设置期初调整额，请设置到子科目（如 %s-明细）", general, general)
			}
		}
	}

	// 4. 提取上月期末作为本月期初
	prevFinals, err := wb.ExtractLastMonthFinals()
	if err != nil {
		return fmt.Errorf("提取上月期末: %w", err)
	}

	// 生产门槛：跳月检测——非首月、非跨年首月（1 月期初走 JSON 结转，12 月 xlsx 在上一级目录）且
	// 上月账本 xlsx 不存在 → 告警（不阻断，余额链靠 JSON 回退仍连续）
	if month > cfg.Settings.StartMonth && !strings.HasSuffix(month, "-01") {
		prevXlsx := wb.prevMonthPath()
		if prevXlsx != "" {
			if _, err := os.Stat(prevXlsx); err != nil {
				fmt.Printf("警告: 上月账本 %s 不存在，疑似跳月/漏月，请确认（本年累计可能缺月）\n", prevXlsx)
			}
		}
	}

	// 构建期初映射
	initials := make(map[string]int64)
	allAccounts := balance.GetLeafAccounts(entries)
	for _, account := range allAccounts {
		initials[account] = balance.GetInitBalanceForGenerate(cfg, account, month, prevFinals)
	}
	// 补充：科目树中有非零余额但当月无分录的科目，也加入期初映射
	for k := range cfg.Tree {
		if _, exists := initials[k]; !exists {
			initials[k] = balance.GetInitBalanceForGenerate(cfg, k, month, prevFinals)
		}
	}

	// 记录当月期初来自调整额的科目（账页期初行摘要用：调整额→"期初余额"）
	wb.InitialAdjust = make(map[string]bool)
	for account := range initials {
		if balance.HasInitialAdjustment(cfg, account, month) {
			wb.InitialAdjust[account] = true
		}
	}

	// 期初试算平衡校验（借正贷负求和，不平告警不阻断，避免历史数据卡死）
	if diff := balance.InitialBalanceDiff(initials); diff != 0 {
		fmt.Printf("⚠ 期初借贷不平衡（%s），差额 %.2f 元（借正贷负）。请核对期初设置\n", month, float64(diff)/100)
	}

	// 5. 生成本月期初表
	if err := wb.WriteInitialSheet(initials); err != nil {
		return fmt.Errorf("生成期初表: %w", err)
	}

	// 6. 追加分录到总分类账 Sheet
	// 对仅有期初余额但无当月分录的科目，追加 上年结转
	if err := wb.AppendEntries(entries, initials); err != nil {
		return fmt.Errorf("追加总分类账: %w", err)
	}
	// 对仅有期初余额但无当月分录的科目，写入 上年结转
	if err := wb.appendCarryForwardOnly(entries, initials); err != nil {
		return fmt.Errorf("追加上年结转: %w", err)
	}

	// 6.1 追加分录到合并总分类账 Sheet（纯增量，不影响原有 GL）
	if err := wb.AppendMergeEntries(entries, initials); err != nil {
		return fmt.Errorf("追加合并总分类账: %w", err)
	}

	// 7. 追加分录到多科目明细账 Sheet
	if err := wb.AppendMLEntries(entries, initials); err != nil {
		return fmt.Errorf("追加多科目明细账: %w", err)
	}

	// 8. 计算当月活动量
	activity := ComputeActivity(entries)
	changedSheets := CollectChangedSheets(entries)

	// 同时收集多科目明细账 Sheet（排除已忽略科目）
	mlSuppress := make(map[string]bool)
	for _, a := range cfg.Settings.MLSuppressAccounts {
		mlSuppress[a] = true
	}
	for general := range getMLGenerals(entries) {
		if !mlSuppress[general] {
			changedSheets[sheetNameML(general)] = true
		}
	}

	// 提取本年累计 — 需要所有 Config.Tree 中的科目，而非仅当月有分录的科目，
	// 否则无当月分录但有历史余额的明细科目的累计值会丢失。
	allAccountsWithHistory := make([]string, len(allAccounts))
	copy(allAccountsWithHistory, allAccounts)
	seen := make(map[string]bool, len(allAccounts))
	for _, a := range allAccounts {
		seen[a] = true
	}
	for k := range cfg.Tree {
		if !seen[k] {
			allAccountsWithHistory = append(allAccountsWithHistory, k)
		}
	}
	ytdDebit, ytdCredit := wb.ExtractYtdTotals(allAccountsWithHistory)

	// 提取本季累计（截至上月）
	qtdDebit, qtdCredit := wb.ExtractQuarterlyTotals(allAccountsWithHistory)

	// 9. 月末结账（总分类账）
	if err := wb.WriteMonthClosings(activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit, initials, changedSheets); err != nil {
		return fmt.Errorf("月结: %w", err)
	}

	// 9.02 现金/银行日记账（设计专家审查 Change 9；ytd 已就绪）
	if err := wb.WriteJournals(entries, initials, ytdDebit, ytdCredit); err != nil {
		return fmt.Errorf("生成日记账: %w", err)
	}

	// 9.05 月末结账（合并总分类账）
	if err := wb.WriteMergeGLClosings(activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit, initials); err != nil {
		return fmt.Errorf("合并总分类账月结: %w", err)
	}

	// 9.1 月末结账（多科目明细账）
	if err := wb.WriteMLMonthClosings(entries, ytdDebit, ytdCredit, qtdDebit, qtdCredit, changedSheets); err != nil {
		return fmt.Errorf("多科目明细账月结: %w", err)
	}

	// 9.5. 生成独立期末余额汇总 Sheet
	if err := wb.WriteFinalSheet(initials, activity); err != nil {
		return fmt.Errorf("生成期末表: %w", err)
	}

	// 10. 回写余额 — 转换为 balance.Activity
	balActivity := make(map[string]balance.Activity)
	for k, v := range activity {
		balActivity[k] = balance.Activity{Debit: v.Debit, Credit: v.Credit}
	}
	if err := balance.UpdateBalancesAfterGenerate(cfg, month, balActivity, initials); err != nil {
		return fmt.Errorf("回写余额: %w", err)
	}

	// 11. 页末写红色"过次页"标签
	if err := wb.finalizeAllGLSheets(); err != nil {
		return fmt.Errorf("页末补齐: %w", err)
	}

	// 12. 保存 xlsx（生产门槛：先落盘 xlsx、成功后回写 JSON——失败原子性）
	if err := wb.Save(); err != nil {
		return fmt.Errorf("保存 xlsx: %w", err)
	}
	if err := balance.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("保存配置: %w", err)
	}

	return nil
}

// appendCarryForwardOnly 对仅有期初余额但无当月分录的科目写入上年结转行。
func (wb *Workbook) appendCarryForwardOnly(entries []voucher.Entry, initials map[string]int64) error {
	// 收集当月有分录的科目
	hasEntries := make(map[string]bool)
	for _, e := range entries {
		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		hasEntries[path] = true
	}
	// 对仅期初非零但无分录的科目，写入期初行（1 月跨年延续 或 当月调整额生效）
	for account, initial := range initials {
		if initial != 0 && !hasEntries[account] && (strings.HasSuffix(wb.Month, "-01") || wb.InitialAdjust[account]) {
			if err := wb.appendToGLSheet(account, nil, initial); err != nil {
				return fmt.Errorf("追加上年结转 %s: %w", account, err)
			}
		}
	}
	return nil
}

// getMLGenerals 返回需要有明细科目的总账科目集合。
func getMLGenerals(entries []voucher.Entry) map[string]bool {
	generals := make(map[string]bool)
	for _, e := range entries {
		if e.DetailAccount != "" {
			generals[e.GeneralAccount] = true
		}
	}
	return generals
}
