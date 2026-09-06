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
	genCloseCmd.Flags().Bool("no-transfer", false, "跳过第二段结转（本年收益→收益分配-未分配收益），仅生成第一段损益结转凭证")
	genCloseCmd.MarkFlagRequired("json")
}

// transferTargetAccount 年末二段结转目标科目（制度 322 收益分配——未分配收益明细）。
// 财会〔2023〕14号 322 使用说明明确"应设置'各项分配''未分配收益'明细科目"，
// 年末结转官方口径即记入未分配收益（净亏损时为借余=未弥补亏损）。
const transferTargetAccount = "收益分配-未分配收益"

var genCloseCmd = &cobra.Command{
	Use:   "gen-close",
	Short: "生成年末损益结转凭证（到 output/{year}/closing/）",
	Long: "读取 {year}.json，对仍有余额的收入/费用科目生成年末结转凭证（结转至 本年收益），" +
		"并追加二段凭证（本年收益→收益分配-未分配收益，净亏损方向相反；--no-transfer 跳过二段），" +
		"写入输出目录 {year}/closing/，不写入手工凭证目录。已结转科目（closing/ 已有凭证覆盖）自动跳过。" +
		"生成后重新 generate 该月即完成年末两段结转（收入/费用与本年收益归零，净额挂账 收益分配-未分配收益）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		output, _ := cmd.Flags().GetString("output")
		noTransfer, _ := cmd.Flags().GetBool("no-transfer")

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
		closingFiles, _ := filepath.Glob(filepath.Join(closingDir, "*.md"))
		existing := len(closingFiles)
		closed, hasTransfer := scanClosingVouchers(closingFiles)

		// 收集待结转损益科目（余额≠0 且未结转）
		lines := collectPnlClosingLines(cfg, month, closed)

		// 类别未知且余额≠0 的科目显式告警（v2 §2.2：此类科目不结转、跨年带入，
		// 此前静默跳过；须在提前返回之前打印）
		printUnknownPnlWarning(unknownPnlAccounts(cfg.Tree, month))

		// 二段（本年收益→收益分配-未分配收益，design.md D2）：
		// postFinal = 本年收益当期期末 + 全部损益科目期末（不排除已结转科目——
		// 覆盖"一段已生成未合并"与"一段已合并"两种运行态，三种状态求和同值）。
		postFinal := benClosingFinal(cfg, month) + pnlClosingSum(cfg, month)
		debitAcct, creditAcct, transferAmount, needTransfer := stage2Transfer(postFinal)
		needStage2 := needTransfer && !noTransfer && !hasTransfer

		if len(lines) == 0 && !needStage2 {
			if postFinal != 0 && noTransfer && !hasTransfer {
				fmt.Printf("无待结转的损益科目（closing/ 已含 %d 张凭证）——注意: 本年收益尚有余额 %.2f 元未二段结转（--no-transfer 跳过）\n",
					existing, float64(postFinal)/100)
				return nil
			}
			fmt.Printf("无待结转的损益科目（closing/ 已含 %d 张凭证，余额均已结转或为 0）\n", existing)
			return nil
		}

		// 结转目标科目必须已在科目树定义（先定义后生成，v2 §2.4）：gen-close 自产凭证
		// 会引用它们，而新建账套通常没有这些节点——不预登记则重新 generate 会被自家
		// 闸门拦截（流程自锁，红队 2026-09-04 实测）。此处自动登记（官方表属性=贷，
		// 系统自洽行为，非静默长科目）。
		registered := false
		if len(lines) > 0 || needStage2 {
			ok, err := registerClosingTarget(cfg, "本年收益", "年末损益结转目标科目（gen-close 自动登记）")
			if err != nil {
				return fmt.Errorf("登记结转目标科目 本年收益: %w", err)
			}
			registered = registered || ok
		}
		if needStage2 {
			ok, err := registerClosingTarget(cfg, transferTargetAccount, "年末收益结转目标科目（gen-close 自动登记，制度 322 未分配收益明细）")
			if err != nil {
				return fmt.Errorf("登记结转目标科目 %s: %w", transferTargetAccount, err)
			}
			registered = registered || ok
		}
		if registered {
			if err := balance.SaveConfig(configPath, cfg); err != nil {
				return fmt.Errorf("保存配置: %w", err)
			}
		}
		sort.Slice(lines, func(i, j int) bool { return lines[i].account < lines[j].account })

		if err := os.MkdirAll(closingDir, 0o755); err != nil {
			return fmt.Errorf("创建 closing 目录: %w", err)
		}

		// 凭证日期 = 余额月最后一天（验收发现：硬编码 12-31 会被 FilterByMonth 过滤，余额月非 12 月时结转不生效）
		yy, _ := strconv.Atoi(month[:4])
		mm, _ := strconv.Atoi(month[5:])
		lastDay := time.Date(yy, time.Month(mm)+1, 0, 0, 0, 0, 0, time.UTC).Day()

		if len(lines) > 0 {
			// 编号：已有文件数+1，避免重名
			num := existing + 1
			target := filepath.Join(closingDir, fmt.Sprintf(stage1VoucherNameFmt, num))
			for {
				if _, err := os.Stat(target); os.IsNotExist(err) {
					break
				}
				num++
				target = filepath.Join(closingDir, fmt.Sprintf(stage1VoucherNameFmt, num))
			}

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
		}

		if needStage2 {
			// 二段编号顺延（一段被跳过时取已有文件数+1）
			num2 := existing + 1
			if len(lines) > 0 {
				num2 = existing + 2
			}
			target2 := filepath.Join(closingDir, fmt.Sprintf(stage2VoucherNameFmt, num2))
			for {
				if _, err := os.Stat(target2); os.IsNotExist(err) {
					break
				}
				num2++
				target2 = filepath.Join(closingDir, fmt.Sprintf(stage2VoucherNameFmt, num2))
			}
			content := buildTransferVoucher(num2, yy, mm, lastDay, debitAcct, creditAcct, transferAmount)
			if err := os.WriteFile(target2, []byte(content), 0o644); err != nil {
				return fmt.Errorf("写入二段结转凭证: %w", err)
			}
			fmt.Printf("已生成二段结转凭证 %s（借 %s / 贷 %s，%.2f 元）——本年收益归零，净额挂账\n",
				target2, debitAcct, creditAcct, float64(transferAmount)/100)
		}

		if needStage2 {
			fmt.Printf("提示: 重新 generate %s 月账本即可完成年末结转（损益与本年收益归零，净额入 %s）；确认无误后 year-close 跨年\n", month, transferTargetAccount)
		} else {
			fmt.Printf("提示: 重新 generate %s 月账本即可完成损益结转（收入/费用归零）；确认无误后 year-close 跨年\n", month)
		}
		return nil
	},
}

const stage1VoucherNameFmt = "记字第%04d号 年末损益结转.md"
const stage2VoucherNameFmt = "记字第%04d号 年末收益结转.md"

// closingLine 一段结转分录行。
type closingLine struct {
	account string
	gen     string
	detail  string
	abs     int64
	debit   bool // true=借科目/贷本年收益（贷余）；false=借本年收益/贷科目（借余）
}

// collectPnlClosingLines 收集待结转损益科目（余额≠0 且未在 closed 中的收入/费用科目）。
func collectPnlClosingLines(cfg *balance.GlobalConfig, month string, closed map[string]bool) []closingLine {
	var lines []closingLine
	for account, node := range cfg.Tree {
		mb, ok := node.Balances[month]
		if !ok || mb.Final == 0 {
			continue
		}
		gen, detail := splitEntryPath(account)
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
		lines = append(lines, closingLine{account: account, gen: gen, detail: detail, abs: abs, debit: mb.Final < 0})
	}
	return lines
}

// scanClosingVouchers 扫描 closing/ 已有凭证：返回已结转科目集合（全路径）与
// 是否已含二段结转（任一分录总账科目段为 收益分配——收益分配制度上只出现在二段，
// 一段凭证不含它，误报面为零；不能用"本年收益出现过"判定，一段凭证本身就含）。
func scanClosingVouchers(files []string) (closed map[string]bool, hasTransfer bool) {
	closed = make(map[string]bool)
	for _, f := range files {
		if es, err := voucher.ParseFile(f); err == nil {
			for _, e := range es {
				key := e.GeneralAccount
				if e.DetailAccount != "" {
					key = e.GeneralAccount + "-" + e.DetailAccount
				}
				closed[key] = true
				if e.GeneralAccount == "收益分配" {
					hasTransfer = true
				}
			}
		}
	}
	return closed, hasTransfer
}

// pnlClosingSum 全部收入/费用科目在 month 的期末余额求和（借正贷负）。
// 不排除已结转科目——回写循环 2 保证非零科目必有当月记录，三种运行态
// （干净首跑/一段已生成未合并/一段已合并）求和同值（design.md D2）。
func pnlClosingSum(cfg *balance.GlobalConfig, month string) int64 {
	var sum int64
	for account, node := range cfg.Tree {
		mb, ok := node.Balances[month]
		if !ok || mb.Final == 0 {
			continue
		}
		gen, _ := splitEntryPath(account)
		if t, ok := balance.AccountTypeOf(gen); ok && (t == "收入" || t == "费用") {
			sum += mb.Final
		}
	}
	return sum
}

// benClosingFinal 本年收益在 month 的期末余额（借正贷负；节点或记录缺失=0——
// 干净账套的本年收益全年无发生、期初 0，回写循环 2 不为零值科目写当月记录）。
func benClosingFinal(cfg *balance.GlobalConfig, month string) int64 {
	if node, ok := cfg.Tree["本年收益"]; ok {
		if mb, ok := node.Balances[month]; ok {
			return mb.Final
		}
	}
	return 0
}

// stage2Transfer 二段方向判定（借正贷负）：
// 贷余（净收益，<0）→ 借 本年收益 / 贷 收益分配-未分配收益；
// 借余（净亏损，>0）→ 借 收益分配-未分配收益 / 贷 本年收益。
// 返回（借方科目, 贷方科目, 金额分, 是否需要）。
func stage2Transfer(postFinal int64) (debit, credit string, amount int64, ok bool) {
	if postFinal == 0 {
		return "", "", 0, false
	}
	abs := postFinal
	if abs < 0 {
		abs = -abs
	}
	if postFinal < 0 { // 贷余=净收益
		return "本年收益", transferTargetAccount, abs, true
	}
	return transferTargetAccount, "本年收益", abs, true
}

// registerClosingTarget 登记结转目标科目（先定义后生成自洽）。已存在返回 false。
func registerClosingTarget(cfg *balance.GlobalConfig, name, note string) (bool, error) {
	if _, ok := cfg.Tree[name]; ok {
		return false, nil
	}
	if err := balance.AddManualAdjustment(cfg, name, cfg.Settings.StartMonth, 0, note); err != nil {
		return false, err
	}
	if err := balance.SetAccountProperty(cfg, name, "贷"); err != nil {
		return false, err
	}
	fmt.Printf("已自动登记结转目标科目 %s（属性 贷）——先定义后生成要求其存在于科目树\n", name)
	return true, nil
}

// buildTransferVoucher 二段结转凭证（年末收益结转）：单借单贷，标准凭证格式。
func buildTransferVoucher(num, yy, mm, dd int, debitAcct, creditAcct string, amountCents int64) string {
	amt := fmt.Sprintf("%.2f", float64(amountCents)/100)
	row := func(acct string, debitCol, creditCol string) string {
		gen, detail := splitEntryPath(acct)
		return fmt.Sprintf("<tr><td>年末收益结转</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", gen, detail, debitCol, creditCol)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("记字第%04d号 1/1\n\n记帐凭证\n\n%04d年%02d月%02d日\n\n附件 张\n\n", num, yy, mm, dd))
	sb.WriteString("<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody>")
	if debitAcct == "本年收益" {
		sb.WriteString(row("本年收益", amt, ""))
		sb.WriteString(row(creditAcct, "", amt))
	} else {
		sb.WriteString(row(debitAcct, amt, ""))
		sb.WriteString(row("本年收益", "", amt))
	}
	sb.WriteString(fmt.Sprintf("<tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr>", amt, amt))
	sb.WriteString("</tbody></table>\n\n会计主管 \n\n记帐 系统生成\n\n审核 \n\n制单 系统\n")
	return sb.String()
}
