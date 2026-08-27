package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringP("output", "o", ".", "输出根目录（检查 print-config.json 自动发现与年度 JSON）")
}

// doctorCmd — 环境自检。面向 agent 的生产排障：一次定位
// 版本 / skill 安装是否最新且自包含 / print-config.json 是否能被自动发现 / 账本结构。
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "环境自检（版本/skill 安装/打印版配置/账本结构）",
	Long:  "面向 agent 的生产环境自检：一次查清 版本、skill 安装状态（是否最新且自包含）、print-config.json 能否被自动发现、输出目录账本结构。排障先跑这个。",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		ok, warn, fail := 0, 0, 0
		report := func(status, item, detail string) {
			switch status {
			case "OK":
				ok++
			case "WARN":
				warn++
			case "FAIL":
				fail++
			}
			fmt.Printf("[%s] %s: %s\n", status, item, detail)
		}

		// ① 版本与平台
		report("OK", "程序", fmt.Sprintf("ledger %s (%s/%s)", rootCmd.Version, runtime.GOOS, runtime.GOARCH))

		// ② skill 安装（exe 旁 .agents/skills/ledger-accounting）
		skillDir := ""
		if exe, err := os.Executable(); err == nil {
			skillDir = filepath.Join(filepath.Dir(exe), ".agents", "skills", "ledger-accounting")
		}
		if skillDir == "" {
			report("FAIL", "skill", "无法定位可执行文件，跳过 skill 检查")
		} else {
			n, _ := countFiles(skillDir)
			// 自包含标记：references/print-config.md 存在 = 新版自包含 skill
			selfContained := fileExists(filepath.Join(skillDir, "references", "print-config.md"))
			switch {
			case n > 0 && selfContained:
				report("OK", "skill", fmt.Sprintf("已安装且自包含（%d 文件）: %s", n, skillDir))
			case n > 0:
				report("FAIL", "skill", fmt.Sprintf("已安装但非自包含（缺 references/print-config.md，%d 文件）——旧版。请重跑 ledger install-skill", n))
			default:
				report("FAIL", "skill", fmt.Sprintf("未安装。请运行 ledger install-skill（安装到 %s）", skillDir))
			}
		}

		// ③ print-config.json 自动发现（cwd → 输出根目录）
		pc := ""
		if fileExists(filepath.Join(".", "print-config.json")) {
			pc = filepath.Join(".", "print-config.json")
		} else if fileExists(filepath.Join(output, "print-config.json")) {
			pc = filepath.Join(output, "print-config.json")
		}
		if pc != "" {
			report("OK", "打印版配置", fmt.Sprintf("可自动发现: %s（generate 无需 --config）", pc))
		} else {
			report("WARN", "打印版配置", "未发现 print-config.json（当前目录/输出根目录）——将用内置默认值；需自定义系数/字体时 ledger init 或拷模板")
		}

		// ④ 年度 JSON（输出根目录 {year}/{year}.json 形态，排除 dist/docs 等非账本 JSON）
		jsons, _ := filepath.Glob(filepath.Join(output, "*", "*.json"))
		var ledgerJsons []string
		for _, j := range jsons {
			dir := filepath.Base(filepath.Dir(j))
			name := strings.TrimSuffix(filepath.Base(j), ".json")
			if dir == name { // 目录名 == 文件名（如 2026/2026.json）才是账本配置
				ledgerJsons = append(ledgerJsons, j)
			}
		}
		if len(ledgerJsons) > 0 {
			report("OK", "账本", fmt.Sprintf("发现 %d 个年度配置: %s", len(ledgerJsons), strings.Join(ledgerJsons, ", ")))
		} else {
			report("WARN", "账本", fmt.Sprintf("输出根目录 %s 下无年度 JSON——需先 ledger init -s <YYYY-MM> -o %s", output, output))
		}

		fmt.Printf("\n结论: %d 项正常, %d 项警告, %d 项失败\n", ok, warn, fail)
		if fail > 0 {
			fmt.Println("请按上述 [FAIL] 项指引修复（多为重跑 ledger install-skill 或 ledger init）")
		}
		return nil
	},
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func countFiles(dir string) (int, error) {
	n := 0
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}
