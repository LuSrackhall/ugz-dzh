package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("start-month", "s", "", "启动月 (YYYY-MM，必填)")
	initCmd.Flags().StringP("output", "o", ".", "输出根目录")
	initCmd.MarkFlagRequired("start-month")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "系统初始化 — 创建年份 JSON 配置",
	Long:  "根据启动月推导年份，在输出目录下创建 {year}/{year}.json 配置文件。\n启动月决定手动补科目的期初回溯起点。",
	RunE: func(cmd *cobra.Command, args []string) error {
		startMonth, _ := cmd.Flags().GetString("start-month")
		output, _ := cmd.Flags().GetString("output")

		year := strings.Split(startMonth, "-")[0]
		yearDir := filepath.Join(output, year)
		path := filepath.Join(yearDir, year+".json")

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在，不会覆盖已有配置", path)
		}

		config := map[string]interface{}{
			"全局设置": map[string]interface{}{
				"启动月":  startMonth,
				"科目顺序": []string{},
				"科目映射表": map[string]string{},
			},
			"科目树":     map[string]interface{}{},
			"自动识别科目": []interface{}{},
			"手动调整科目": []interface{}{},
		}

		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化配置: %w", err)
		}
		if err := os.MkdirAll(yearDir, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return fmt.Errorf("写入配置: %w", err)
		}
		fmt.Printf("已创建 %s\n", path)
		return nil
	},
}
