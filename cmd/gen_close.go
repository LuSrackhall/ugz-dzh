package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ledger/balance"
	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(genCloseCmd)
	genCloseCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填，如 2025.json）")
	genCloseCmd.Flags().StringP("output", "o", ".", "输出根目录")
	genCloseCmd.MarkFlagRequired("json")
}

var genCloseCmd = &cobra.Command{
	Use:   "gen-close",
	Short: "生成年末损益结转凭证（到 output/{year}/closing/）",
	Long: "读取 {year}.json，对仍有余额的收入/费用科目生成年末结转凭证（结转至 本年收益），" +
		"写入输出目录 {year}/closing/，不写入手工凭证目录。已结转科目（closing/ 已有凭证覆盖）自动跳过。" +
		"生成后重新 generate 该月即完成损益结转（收入/费用归零）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		output, _ := cmd.Flags().GetString("output")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		year := strings.TrimSuffix(filepath.Base(configPath), ".json")
		if len(year) != 4 {
			return fmt.Errorf("无法从 %s 推断年份（期望文件名如 2025.json）", configPath)
		}

		// 余额月份：取该年最大月份
		month := ""
		for _, node := range cfg.Tree {
			for m := range node.Balances {
				if strings.HasPrefix(m, year) && m > month {
					month = m
				}
			}
		}
		if month == "" {
			return fmt.Errorf("科目树中无 %s 年余额记录", year)
		}

		closingDir := filepath.Join(output, year, "closing")
		// 已结转科目：扫描 closing/ 已有凭证（全路径匹配——结转凭证中 总账+明细 需组合）
		closed := make(map[string]bool)
		existing := 0
		if files, err := filepath.Glob(filepath.Join(closingDir, "*.md")); err == nil {
			existing = len(files)
			for _, f := range files {
				if es, err := voucher.ParseFile(f); err == nil {
					for _, e := range es {
						key := e.GeneralAccount
						if e.DetailAccount != "" {
							key = e.GeneralAccount + "-" + e.DetailAccount
						}
						closed[key] = true
					}
				}
			}
		}

		// 收集待结转损益科目（余额≠0 且未结转）
		type line struct {
			account string
			gen     string
			detail  string
			abs     int64
			debit   bool // true=借科目/贷本年收益（贷余）；false=借本年收益/贷科目（借余）
		}
		var lines []line
		for account, node := range cfg.Tree {
			mb, ok := node.Balances[month]
			if !ok || mb.Final == 0 {
				continue
			}
			gen := account
			detail := ""
			if idx := strings.IndexByte(gen, '-'); idx > 0 {
				gen, detail = account[:idx], account[idx+1:]
			}
			if t, ok := balance.AccountTypeOf(gen); !ok || (t != "收入" && t != "费用") {
				continue
			}
			if closed[account] {
				continue
			}
			abs := mb.Final
			if abs < 0 {
				abs = -abs
			}
			lines = append(lines, line{account: account, gen: gen, detail: detail, abs: abs, debit: mb.Final < 0})
		}
		// 类别未知且余额≠0 的科目显式告警（v2 §2.2：此类科目不结转、跨年带入，
		// 此前静默跳过；须在提前返回之前打印）
		printUnknownPnlWarning(unknownPnlAccounts(cfg.Tree, month))
		if len(lines) == 0 {
			fmt.Printf("无待结转的损益科目（closing/ 已含 %d 张凭证，余额均已结转或为 0）\n", existing)
			return nil
		}
		sort.Slice(lines, func(i, j int) bool { return lines[i].account < lines[j].account })

		// 编号：已有文件数+1，避免重名
		num := existing + 1
		target := filepath.Join(closingDir, fmt.Sprintf("记字第%04d号 年末损益结转.md", num))
		for {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				break
			}
			num++
			target = filepath.Join(closingDir, fmt.Sprintf("记字第%04d号 年末损益结转.md", num))
		}

		if err := os.MkdirAll(closingDir, 0o755); err != nil {
			return fmt.Errorf("创建 closing 目录: %w", err)
		}

		// 凭证日期 = 余额月最后一天（验收发现：硬编码 12-31 会被 FilterByMonth 过滤，余额月非 12 月时结转不生效）
		yy, _ := strconv.Atoi(month[:4])
		mm, _ := strconv.Atoi(month[5:])
		lastDay := time.Date(yy, time.Month(mm)+1, 0, 0, 0, 0, 0, time.UTC).Day()

		// 写凭证（标准格式，可被解析器识别）
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("记字第%04d号 1/1\n\n记帐凭证\n\n%04d年%02d月%02d日\n\n附件 张\n\n", num, yy, mm, lastDay))
		sb.WriteString("<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody>")
		var totalDebit, totalCredit float64
		for _, l := range lines {
			amt := fmt.Sprintf("%.2f", float64(l.abs)/100)
			if l.debit { // 贷余 → 借 科目 / 贷 本年收益
				sb.WriteString(fmt.Sprintf("<tr><td>年末损益结转</td><td>%s</td><td>%s</td><td>%s</td><td></td></tr>", l.gen, l.detail, amt))
				sb.WriteString(fmt.Sprintf("<tr><td>年末损益结转</td><td>本年收益</td><td></td><td></td><td>%s</td></tr>", amt))
				totalDebit += float64(l.abs) / 100
				totalCredit += float64(l.abs) / 100
			} else { // 借余 → 借 本年收益 / 贷 科目
				sb.WriteString(fmt.Sprintf("<tr><td>年末损益结转</td><td>本年收益</td><td></td><td>%s</td><td></td></tr>", amt))
				sb.WriteString(fmt.Sprintf("<tr><td>年末损益结转</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr>", l.gen, l.detail, amt))
				totalDebit += float64(l.abs) / 100
				totalCredit += float64(l.abs) / 100
			}
		}
		sb.WriteString(fmt.Sprintf("<tr><td>合计</td><td></td><td></td><td>%.2f</td><td>%.2f</td></tr>", totalDebit, totalCredit))
		sb.WriteString("</tbody></table>\n\n会计主管 \n\n记帐 系统生成\n\n审核 \n\n制单 系统\n")

		if err := os.WriteFile(target, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("写入结转凭证: %w", err)
		}

		fmt.Printf("已生成结转凭证 %s（%d 个损益科目，借贷合计 %.2f 元）\n", target, len(lines), totalDebit)
		fmt.Printf("提示: 重新 generate %s 月账本即可完成损益结转（收入/费用归零）；确认无误后 year-close 跨年\n", month)
		return nil
	},
}
