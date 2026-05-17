package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("start-month", "s", "", "启动月 (YYYY-MM，必填)")
	initCmd.Flags().StringP("output", "o", ".", "输出目录")
	initCmd.MarkFlagRequired("start-month")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "系统初始化 — 创建 科目余额总览.json",
	Long:  "创建初始的科目余额总览.json，包含启动月设置和空的科目树。",
	RunE: func(cmd *cobra.Command, args []string) error {
		startMonth, _ := cmd.Flags().GetString("start-month")
		output, _ := cmd.Flags().GetString("output")

		path := filepath.Join(output, "科目余额总览.json")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在，不会覆盖已有配置", path)
		}

		config := map[string]interface{}{
			"全局设置": map[string]interface{}{
				"启动月":  startMonth,
				"科目顺序": []string{},
			},
			"科目树":     map[string]interface{}{},
			"自动识别科目": []interface{}{},
			"手动调整科目": []interface{}{},
		}

		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化配置: %w", err)
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return fmt.Errorf("写入配置: %w", err)
		}
		fmt.Printf("已创建 %s\n", path)
		return nil
	},
}
