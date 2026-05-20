package generator

import (
	"fmt"

	"ledger/balance"
	"ledger/voucher"
)

// GenerateWorkbook 是 xlsx 生成的唯一入口，按序执行完整生成流程。
// entries 应已经过同年同月校验和科目映射替换。
func GenerateWorkbook(configPath, month, outputDir string, entries []voucher.Entry) error {
	// 1. 加载配置
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("月份 %s 没有匹配的凭证分录", month)
	}

	// 3. 创建或复制上月工作薄
	wb, err := NewWorkbook(configPath, month, outputDir)
	if err != nil {
		return fmt.Errorf("创建工作薄: %w", err)
	}

	// 4. 提取上月期末作为本月期初
	prevFinals, err := wb.ExtractLastMonthFinals()
	if err != nil {
		return fmt.Errorf("提取上月期末: %w", err)
	}

	// 构建期初映射
	initials := make(map[string]int64)
	allAccounts := balance.GetLeafAccounts(entries)
	for _, account := range allAccounts {
		initials[account] = balance.GetInitBalanceForGenerate(cfg, account, month, prevFinals)
	}

	// 5. 生成本月期初表
	if err := wb.WriteInitialSheet(initials); err != nil {
		return fmt.Errorf("生成期初表: %w", err)
	}

	// 6. 追加分录到总分类账 Sheet
	if err := wb.AppendEntries(entries, initials); err != nil {
		return fmt.Errorf("追加总分类账: %w", err)
	}

	// 7. 追加分录到多科目明细账 Sheet
	if err := wb.AppendMLEntries(entries, initials); err != nil {
		return fmt.Errorf("追加多科目明细账: %w", err)
	}

	// 8. 计算当月活动量
	activity := ComputeActivity(entries)
	changedSheets := CollectChangedSheets(entries)

	// 同时收集多科目明细账 Sheet
	for general := range getMLGenerals(entries) {
		changedSheets[sheetNameML(general)] = true
	}

	// 提取本年累计
	ytdDebit, ytdCredit := wb.ExtractYtdTotals(allAccounts)

	// 提取本季累计（截至上月）
	qtdDebit, qtdCredit := wb.ExtractQuarterlyTotals(allAccounts)

	// 9. 月末结账（总分类账）
	if err := wb.WriteMonthClosings(activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit, initials, changedSheets); err != nil {
		return fmt.Errorf("月结: %w", err)
	}

	// 9.1 月末结账（多科目明细账）
	if err := wb.WriteMLMonthClosings(entries, initials, ytdDebit, ytdCredit, qtdDebit, qtdCredit, changedSheets); err != nil {
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
	if err := balance.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("保存配置: %w", err)
	}

	// 11. 保存 xlsx
	if err := wb.Save(); err != nil {
		return fmt.Errorf("保存 xlsx: %w", err)
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
