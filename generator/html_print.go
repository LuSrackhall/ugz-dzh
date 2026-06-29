package generator

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"ledger/voucher"
)

//go:embed templates/*.html
var templateFS embed.FS

// PrintEntry 打印版条目数据
type PrintEntry struct {
	Date            string
	VoucherNum      string
	Summary         string
	DebitDisplay    string
	CreditDisplay   string
	Direction       string
	BalanceDisplay  string
	AmountDigits    []string
}

// PrintData 打印版页面数据
type PrintData struct {
	Title               string
	Entries             []PrintEntry
	AmountHeaders       []string
	ShowMonthlyTotal    bool
	MonthlyDebitDisplay string
	MonthlyCreditDisplay string
	MonthlyBalanceDisplay string
	MonthlyAmountDigits []string
	ShowYearlyTotal     bool
	YearlyDebitDisplay  string
	YearlyCreditDisplay string
	YearlyBalanceDisplay string
	YearlyAmountDigits  []string
}

// GenerateHTMLPrint 生成 HTML 打印版文件
func GenerateHTMLPrint(entries []voucher.Entry, initials map[string]int64, configPath, month, outputDir string) error {
	// 加载配置
	cfg, err := loadConfigForHTML(configPath)
	if err != nil {
		return fmt.Errorf("加载配置: %w", err)
	}

	// 按总账科目分组
	groups := groupEntriesByAccount(entries)

	// 为每个科目生成 HTML 文件
	for account, groupEntries := range groups {
		if err := generateAccountHTML(account, groupEntries, initials, cfg, month, outputDir); err != nil {
			return fmt.Errorf("生成科目 %s HTML: %w", account, err)
		}
	}

	return nil
}

// loadConfigForHTML 加载配置（简化版）
func loadConfigForHTML(configPath string) (interface{}, error) {
	// 这里简化处理，实际需要加载 balance.GlobalConfig
	return nil, nil
}

// groupEntriesByAccount 按总账科目分组
func groupEntriesByAccount(entries []voucher.Entry) map[string][]voucher.Entry {
	groups := make(map[string][]voucher.Entry)
	for _, e := range entries {
		groups[e.GeneralAccount] = append(groups[e.GeneralAccount], e)
	}
	return groups
}

// generateAccountHTML 为单个科目生成 HTML 文件
func generateAccountHTML(account string, entries []voucher.Entry, initials map[string]int64, cfg interface{}, month, outputDir string) error {
	// 解析模板
	tmpl, err := template.ParseFS(templateFS, "templates/print.html")
	if err != nil {
		return fmt.Errorf("解析模板: %w", err)
	}

	// 准备数据
	data := preparePrintData(account, entries, initials)

	// 创建 html 子目录
	htmlDir := filepath.Join(outputDir, "html")
	if err := os.MkdirAll(htmlDir, 0o755); err != nil {
		return fmt.Errorf("创建 html 目录: %w", err)
	}

	// 创建输出文件
	outputPath := filepath.Join(htmlDir, fmt.Sprintf("%s-%s-print.html", month, account))
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件 %s: %w", outputPath, err)
	}
	defer file.Close()

	// 渲染模板
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("渲染模板: %w", err)
	}

	return nil
}

// preparePrintData 准备打印数据
func preparePrintData(account string, entries []voucher.Entry, initials map[string]int64) PrintData {
	data := PrintData{
		Title:         fmt.Sprintf("总分类账 — %s", account),
		AmountHeaders: []string{"十", "亿", "千", "百", "十", "万", "千", "百", "十", "元", "角", "分"},
	}

	balance := initials[account]
	var monthlyDebit, monthlyCredit int64

	for _, e := range entries {
		balance = balance + e.DebitCents - e.CreditCents
		monthlyDebit += e.DebitCents
		monthlyCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		entry := PrintEntry{
			Date:           e.Date,
			VoucherNum:     fmt.Sprintf("%d", e.VoucherNum),
			Summary:        e.Summary,
			DebitDisplay:   formatAmountForDisplay(e.DebitCents),
			CreditDisplay:  formatAmountForDisplay(e.CreditCents),
			Direction:      dir,
			BalanceDisplay: formatAmountForDisplay(dispBal),
			AmountDigits:   centsToDigitStrings(e.DebitCents),
		}

		data.Entries = append(data.Entries, entry)
	}

	// 本月合计
	if len(entries) > 0 {
		data.ShowMonthlyTotal = true
		data.MonthlyDebitDisplay = formatAmountForDisplay(monthlyDebit)
		data.MonthlyCreditDisplay = formatAmountForDisplay(monthlyCredit)
		_, dispBal := directionFor(balance, 0)
		data.MonthlyBalanceDisplay = formatAmountForDisplay(dispBal)
		data.MonthlyAmountDigits = centsToDigitStrings(monthlyDebit)
	}

	return data
}

// centsToDigitStrings 将金额转为 12 位数字字符串数组
func centsToDigitStrings(cents int64) []string {
	digits := centsToDigits(cents)
	result := make([]string, 12)
	for i, d := range digits {
		if d > 0 || i >= 9 {
			result[i] = fmt.Sprintf("%d", d)
		} else {
			result[i] = ""
		}
	}
	return result
}
