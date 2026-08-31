package cmd

import (
	"fmt"
	"math"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(openingCmd)
	openingCmd.AddCommand(openingImportCmd)
	openingCmd.PersistentFlags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	openingCmd.MarkPersistentFlagRequired("json")

	openingImportCmd.Flags().StringP("file", "f", "", "建账审核表 CSV 路径（必填；列：科目/方向/期初余额/备注）")
	openingImportCmd.Flags().Bool("dry-run", false, "预演模式：只打印将写入的期初与校验结果，不写入 JSON")
	openingImportCmd.MarkFlagRequired("file")
}

var openingCmd = &cobra.Command{
	Use:   "opening",
	Short: "期初余额批量导入（建账审核表）",
	Long: "子命令：\n" +
		"  import  按建账审核表批量导入期初余额——迁移时科目期初的入口（替代逐条 add-manual）\n" +
		"写入语义与 add-manual 完全一致（手动调整科目.期初调整额，锚定建账月）；\n" +
		"方向借 → 调整额 +金额，方向贷 → 调整额 -金额（净额口径：借正贷负）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var openingImportCmd = &cobra.Command{
	Use:   "import -f <建账审核表.csv>",
	Short: "按建账审核表批量导入期初余额（试算平衡强制闸门）",
	Long: "读取建账审核表 CSV，把「期初余额」列非空的行批量写入 期初调整额（借→+，贷→-）。\n" +
		"写入前逐项校验：① 科目已存在（先 subjects import）且非合并总账父级；② 科目属性与方向一致；\n" +
		"③ 全部期初（含已有调整）试算平衡（借正贷负求和 = 0），不平拒绝并列出明细——不平衡说明旧账本身有问题，先对账。\n" +
		"同科目重复导入 = 更新（修正用）；--dry-run 预演不落盘。",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		configPath, _ := cmd.Flags().GetString("json")

		rows, err := parseReviewCSV(file)
		if err != nil {
			return err
		}
		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}
		if cfg.Settings.StartMonth == "" {
			return fmt.Errorf("JSON 缺少 全局设置.启动月——请先 ledger init 建账")
		}

		// ① 逐项校验（合并父级 / 科目存在 / 属性与方向一致），任一失败即拒绝
		type pending struct {
			account string
			yuan    float64 // 带符号调整额（借+/贷-）
			note    string
		}
		var pendings []pending
		skippedNoBalance := 0
		var debitSum, creditSum float64
		for _, r := range rows {
			if err := validateReviewRow(cfg, r); err != nil {
				return err
			}
			if !r.OpeningSet || r.OpeningYuan == 0 {
				skippedNoBalance++
				continue // 仅登记科目行（或显式 0），无期初可导入
			}
			node, ok := cfg.Tree[r.Account]
			if !ok {
				return fmt.Errorf("科目 %s 不在科目树中——请先运行 ledger subjects import", r.Account)
			}
			if node.Property != r.Direction {
				return fmt.Errorf("科目 %s 属性为 %q，与审核表方向 %q 不一致——请先重跑 ledger subjects import 统一属性", r.Account, node.Property, r.Direction)
			}
			signed := r.OpeningYuan
			if r.Direction == "贷" {
				signed = -r.OpeningYuan
			}
			pendings = append(pendings, pending{account: r.Account, yuan: signed, note: r.Note})
			if signed > 0 {
				debitSum += signed
			} else {
				creditSum += -signed
			}
		}

		// ② 试算平衡闸门：合并已有调整（手动覆盖自动）+ 本次导入，借正贷负求和必须为 0
		merged := openingAdjustmentsOf(cfg)
		for _, p := range pendings {
			merged[p.account] = balance.YuanToCents(p.yuan)
		}
		var total int64
		for _, v := range merged {
			total += v
		}
		if total != 0 {
			fmt.Printf("✗ 期初试算不平衡：借方合计 %.2f 元，贷方合计 %.2f 元，差额 %.2f 元（借正贷负）\n",
				debitSum, creditSum, float64(total)/100)
			fmt.Println("各科目期初调整额（含已有）：")
			for a, v := range merged {
				if v != 0 {
					fmt.Printf("  %s\t%+.2f\n", a, float64(v)/100)
				}
			}
			return fmt.Errorf("期初借贷不平衡（差额 %.2f 元）——不平衡说明旧账本身有问题，请先与老板对账，不要把错误带进新账", float64(total)/100)
		}

		prefix := ""
		if dryRun {
			prefix = "【预演】"
		}
		if dryRun {
			fmt.Printf("%s期初导入校验全部通过：拟导入 %d 个科目（借方合计 %.2f 元，贷方合计 %.2f 元，试算平衡 ✓）\n",
				prefix, len(pendings), debitSum, creditSum)
			for _, p := range pendings {
				fmt.Printf("  %s\t%s\t%.2f\n", p.account, map[bool]string{true: "借", false: "贷"}[p.yuan > 0], math.Abs(p.yuan))
			}
			if skippedNoBalance > 0 {
				fmt.Printf("  跳过无余额科目行 %d 个（仅登记，不写期初）\n", skippedNoBalance)
			}
			fmt.Println("（预演模式，未写入 JSON）")
			return nil
		}

		// ③ 写入（与 add-manual 同语义：手动调整科目.期初调整额，锚定建账月；同科目 = 更新）
		for _, p := range pendings {
			if err := balance.AddManualAdjustment(cfg, p.account, cfg.Settings.StartMonth, p.yuan, p.note); err != nil {
				return fmt.Errorf("写入期初 %s: %w", p.account, err)
			}
		}
		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return fmt.Errorf("保存配置: %w", err)
		}

		fmt.Printf("✓ 期初导入完成: %d 个科目（借方合计 %.2f 元，贷方合计 %.2f 元，试算平衡 ✓）\n",
			len(pendings), debitSum, creditSum)
		if skippedNoBalance > 0 {
			fmt.Printf("  跳过无余额科目行 %d 个（仅登记，不写期初）\n", skippedNoBalance)
		}
		fmt.Printf("下一步: ledger check -j %s → generate 建账月 %s（已生成过则 -f）→ git commit\n",
			configPath, cfg.Settings.StartMonth)
		return nil
	},
}
