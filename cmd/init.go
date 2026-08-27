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
	Short: "系统初始化 — 创建账本管理体系（凭证目录/配置模板/年份 JSON）",
	Long:  "根据启动月推导年份，在输出根目录建立完整的账本管理体系：\n  - vouchers/            凭证输入目录（每月凭证放 vouchers/YYYY_MM/*.md）\n  - print-config.json    打印版配置模板（generate 自动发现，放本目录即生效）\n  - README.md            体系使用说明\n  - {year}/{year}.json   年份账本配置（权威源）\n启动月决定手动补科目的期初回溯起点。",
	RunE: func(cmd *cobra.Command, args []string) error {
		startMonth, _ := cmd.Flags().GetString("start-month")
		output, _ := cmd.Flags().GetString("output")

		year := strings.Split(startMonth, "-")[0]
		yearDir := filepath.Join(output, year)
		path := filepath.Join(yearDir, year+".json")

		// ① 凭证输入目录（幂等：目录已存在则跳过）
		voucherDir := filepath.Join(output, "vouchers")
		if err := os.MkdirAll(voucherDir, 0o755); err != nil {
			return fmt.Errorf("创建凭证目录: %w", err)
		}
		voucherReadme := filepath.Join(voucherDir, "README.md")
		if _, err := os.Stat(voucherReadme); os.IsNotExist(err) {
			if err := os.WriteFile(voucherReadme, []byte(voucherReadmeContent), 0o644); err != nil {
				return fmt.Errorf("写入凭证目录说明: %w", err)
			}
		}
		fmt.Printf("凭证目录: %s（每月凭证放 vouchers/YYYY_MM/*.md）\n", voucherDir)

		// ② 打印版配置模板（幂等：已存在不覆盖，用户改过的保留）
		printCfgPath := filepath.Join(output, "print-config.json")
		if _, err := os.Stat(printCfgPath); os.IsNotExist(err) {
			if err := os.WriteFile(printCfgPath, []byte(printConfigTemplate), 0o644); err != nil {
				return fmt.Errorf("写入打印版配置模板: %w", err)
			}
			fmt.Printf("打印版配置模板: %s（generate 自动发现，改系数/字体后重新生成即生效）\n", printCfgPath)
		} else {
			fmt.Printf("打印版配置已存在（保留用户修改）: %s\n", printCfgPath)
		}

		// ③ 根 README（幂等）
		rootReadme := filepath.Join(output, "README.md")
		if _, err := os.Stat(rootReadme); os.IsNotExist(err) {
			if err := os.WriteFile(rootReadme, []byte(rootReadmeContent), 0o644); err != nil {
				return fmt.Errorf("写入根目录说明: %w", err)
			}
		}

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在，不会覆盖已有配置", path)
		}

		config := map[string]interface{}{
			"全局设置": map[string]interface{}{
				"启动月":         startMonth,
				"科目顺序":       []string{},
				"科目映射表":     map[string]string{},
				"合并总账科目":   []string{},
				"总分类账忽略科目": []string{},
				"多科目明细账忽略科目": []string{},
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
		fmt.Printf("账本管理体系就绪：\n  - 凭证目录 %s\n  - 打印版配置 %s\n  - 年度配置 %s\n", voucherDir, printCfgPath, path)
		fmt.Printf("下一步：凭证放 vouchers/YYYY_MM/ 后运行 ledger generate -v vouchers/YYYY_MM -o %s\n", output)
		return nil
	},
}

// voucherReadmeContent 凭证目录说明（init 创建 vouchers/README.md）。
const voucherReadmeContent = `# 凭证目录（输入）

每个月的凭证放独立子目录，命名二选一（均可识别）：
- vouchers/YYYY_MM/   （如 2025_10）
- vouchers/YYYY/MM/   （如 2025/10）

凭证文件：*.md，文件名含凭证号，如"记字第0001号.md"。
同一目录下所有凭证必须同一年同一月。

生成账本：
  ledger generate -v vouchers/2025_10 -o .
`

// rootReadmeContent 账本根目录说明（init 创建 README.md）。
const rootReadmeContent = `# 手工账本目录（ledger 管理体系）

结构：
  vouchers/          凭证输入目录（每月凭证放 vouchers/YYYY_MM/*.md）
  print-config.json  打印版配置（平台补偿系数 + 分区域字体；generate 自动发现，
                     放本目录即生效，改完重新生成即可，无需传参）
  <年份>/            各年度账本输出（<年份>.json 为唯一权威源，git 提交）

常用命令：
  ledger init -s 2025-10 -o .                          # 建账（已自动建好目录与配置模板）
  ledger generate -v vouchers/2025_10 -o .             # 生成月度账本（自动应用 print-config.json）
  ledger year-close -j 2025/2025.json -o .             # 跨年结转
  ledger install-skill                                 # 安装 ledger-accounting 技能供 agent 使用

详细用法见项目 docs/ 目录（commands.md、print-config.md）。
`

// printConfigTemplate 打印版配置模板（init 创建 print-config.json）。
// 与 docs/print-config.example.json 保持一致；字段说明见 docs/print-config.md。
const printConfigTemplate = `{
  "platforms": {
    "windows": {
      "colScale": 1.1075,
      "rowScale": 0.992,
      "fonts": {
        "normal": "Calibri",
        "digit": "Noteworthy",
        "title": "仿宋",
        "default": "宋体"
      }
    },
    "mac": {
      "colScale": 1.0,
      "rowScale": 1.0,
      "fonts": {
        "normal": "Calibri",
        "digit": "Noteworthy",
        "title": "仿宋",
        "default": "宋体"
      }
    }
  }
}
`
