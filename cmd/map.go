package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mapCmd)
	mapCmd.AddCommand(mapAddCmd)
	mapCmd.AddCommand(mapDeleteCmd)
	mapCmd.AddCommand(mapListCmd)

	mapCmd.PersistentFlags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	mapCmd.MarkPersistentFlagRequired("json")

	mapAddCmd.Flags().StringP("from", "f", "", "OCR 识别到的原始名称（必填）")
	mapAddCmd.Flags().StringP("to", "t", "", "标准科目名（必填）")
	mapAddCmd.MarkFlagRequired("from")
	mapAddCmd.MarkFlagRequired("to")

	mapDeleteCmd.Flags().StringP("from", "f", "", "要删除的 OCR 原始名称（必填）")
	mapDeleteCmd.MarkFlagRequired("from")
}

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "管理科目名称映射表",
	Long:  "增删查科目名称映射（OCR识别名 → 标准科目名），确保相同科目唯一性。\n子命令：add, delete, list",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var mapAddCmd = &cobra.Command{
	Use:   "add",
	Short: "添加科目映射",
	Long:  "添加一条 OCR 识别名 → 标准科目名的映射，如 \"管埋费用→管理费用\"。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return err
		}

		if cfg.Settings.AccountMap == nil {
			cfg.Settings.AccountMap = make(map[string]string)
		}

		if existing, ok := cfg.Settings.AccountMap[from]; ok {
			return fmt.Errorf("映射 %q → %q 已存在", from, existing)
		}

		cfg.Settings.AccountMap[from] = to
		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("已添加映射: %q → %q\n", from, to)
		return nil
	},
}

var mapDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除科目映射",
	Long:  "删除一条 OCR 识别名的映射条目。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		from, _ := cmd.Flags().GetString("from")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return err
		}

		if cfg.Settings.AccountMap == nil {
			return fmt.Errorf("映射 %q 不存在", from)
		}

		if _, ok := cfg.Settings.AccountMap[from]; !ok {
			return fmt.Errorf("映射 %q 不存在", from)
		}

		delete(cfg.Settings.AccountMap, from)
		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("已删除映射: %q\n", from)
		return nil
	},
}

var mapListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有科目映射",
	Long:  "列出当前 JSON 配置中的所有科目名称映射。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return err
		}

		if cfg.Settings.AccountMap == nil || len(cfg.Settings.AccountMap) == 0 {
			fmt.Println("暂无科目映射")
			return nil
		}

		fmt.Println("OCR 识别名 → 标准科目名")
		for from, to := range cfg.Settings.AccountMap {
			fmt.Printf("  %q → %q\n", from, to)
		}
		fmt.Printf("共 %d 条映射\n", len(cfg.Settings.AccountMap))
		return nil
	},
}
