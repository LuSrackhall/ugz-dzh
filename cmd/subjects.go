package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(subjectsCmd)
	subjectsCmd.AddCommand(subjectsImportCmd)
	subjectsCmd.AddCommand(subjectsListCmd)
	subjectsCmd.AddCommand(subjectsExportCmd)
	subjectsCmd.PersistentFlags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	subjectsCmd.MarkPersistentFlagRequired("json")

	subjectsImportCmd.Flags().StringP("file", "f", "", "建账审核表 CSV 路径（必填；列：科目/方向/期初余额/备注，期初余额列在此忽略）")
	subjectsImportCmd.Flags().Bool("dry-run", false, "预演模式：只打印将执行的变更，不写入 JSON")
	subjectsImportCmd.MarkFlagRequired("file")

	subjectsExportCmd.Flags().StringP("out", "o", "", "导出 CSV 路径（缺省打印到标准输出）")
}

var subjectsCmd = &cobra.Command{
	Use:   "subjects",
	Short: "科目登记与管理（建账审核表批量导入）",
	Long: "子命令：\n" +
		"  import  按建账审核表批量登记科目并设置科目属性（借/贷）——迁移时科目建立的入口\n" +
		"  list    列出科目树（科目/属性/类别）\n" +
		"  export  导出科目为建账审核表格式（供盘点 diff）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var subjectsImportCmd = &cobra.Command{
	Use:   "import -f <建账审核表.csv>",
	Short: "按建账审核表批量登记科目并设置科目属性",
	Long: "读取建账审核表 CSV（列：科目/方向/期初余额/备注，列序无关），批量登记科目并设置科目属性。\n" +
		"「方向」列（借/贷）即科目属性；已存在的科目只更新属性（不动期初调整额）；新科目以 0 期初登记（期初由 opening import 导入）。\n" +
		"--dry-run 预演不落盘。",
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

		// 先整体校验（合并父级禁记账），任一失败即拒绝，不落半套
		for _, r := range rows {
			if err := validateReviewRow(cfg, r); err != nil {
				return err
			}
		}

		prefix := ""
		if dryRun {
			prefix = "【预演】"
		}
		var created, propChanged, propSame []string
		for _, r := range rows {
			_, exists := cfg.Tree[r.Account]
			if !exists {
				if !dryRun {
					// 以 0 期初登记（期初余额由 opening import 导入，不在此处混写）
					if err := balance.AddManualAdjustment(cfg, r.Account, cfg.Settings.StartMonth, 0, r.Note); err != nil {
						return fmt.Errorf("登记科目 %s: %w", r.Account, err)
					}
					// 「方向」列即科目属性——新登记科目同样应用（S3：此前只有已存在
					// 科目才设置属性，自创名新科目的方向被静默丢弃，需二次 import）
					if err := balance.SetAccountProperty(cfg, r.Account, r.Direction); err != nil {
						return fmt.Errorf("设置新科目属性 %s: %w", r.Account, err)
					}
				}
				created = append(created, r.Account)
				continue
			}
			// 属性以审核表「方向」为准（未分类科目的唯一指定入口）；新科目不重复计入属性变更
			oldProp := cfg.Tree[r.Account].Property
			if !dryRun {
				if err := balance.SetAccountProperty(cfg, r.Account, r.Direction); err != nil {
					return fmt.Errorf("设置科目属性 %s: %w", r.Account, err)
				}
			}
			if oldProp == r.Direction {
				propSame = append(propSame, r.Account)
			} else {
				propChanged = append(propChanged, fmt.Sprintf("%s（%s → %s）", r.Account, oldProp, r.Direction))
			}
		}

		if !dryRun {
			if err := balance.SaveConfig(configPath, cfg); err != nil {
				return fmt.Errorf("保存配置: %w", err)
			}
		}

		fmt.Printf("%s科目导入完成: 新登记 %d 个，属性更新 %d 个，属性不变 %d 个\n",
			prefix, len(created), len(propChanged), len(propSame))
		for _, a := range created {
			fmt.Printf("  新登记: %s\n", a)
		}
		for _, a := range propChanged {
			fmt.Printf("  属性更新: %s\n", a)
		}
		if dryRun {
			fmt.Println("（预演模式，未写入 JSON）")
		} else {
			fmt.Printf("提示: 期初余额请用 ledger opening import 导入；随后 ledger check 校验\n")
		}
		return nil
	},
}

var subjectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出科目树（科目/属性/类别）",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}
		accounts := make([]string, 0, len(cfg.Tree))
		for a := range cfg.Tree {
			accounts = append(accounts, a)
		}
		sort.Strings(accounts)
		if len(accounts) == 0 {
			fmt.Println("科目树为空")
			return nil
		}
		fmt.Println("科目\t属性\t类别")
		for _, a := range accounts {
			gen := generalOf(a)
			known := "已知"
			if balance.IsUnknownType(gen) {
				known = "未分类"
			}
			fmt.Printf("%s\t%s\t%s\n", a, cfg.Tree[a].Property, known)
		}
		fmt.Printf("共 %d 个科目\n", len(accounts))
		return nil
	},
}

var subjectsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "导出科目为建账审核表格式（供盘点 diff）",
	Long:  "把当前科目树导出为建账审核表 CSV（科目/方向/期初余额/备注；期初余额留空），供与旧账科目余额表做双向 diff。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		out, _ := cmd.Flags().GetString("out")
		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		accounts := make([]string, 0, len(cfg.Tree))
		for a := range cfg.Tree {
			accounts = append(accounts, a)
		}
		sort.Strings(accounts)

		var buf strings.Builder
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"科目", "方向", "期初余额", "备注"})
		for _, a := range accounts {
			_ = w.Write([]string{a, cfg.Tree[a].Property, "", ""})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return fmt.Errorf("生成 CSV: %w", err)
		}

		if out == "" {
			fmt.Print(buf.String())
			return nil
		}
		if err := os.WriteFile(out, []byte(buf.String()), 0o644); err != nil {
			return fmt.Errorf("写入导出文件: %w", err)
		}
		fmt.Printf("已导出 %d 个科目 → %s\n", len(accounts), out)
		return nil
	},
}
