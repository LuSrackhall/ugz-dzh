package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ledger/embedded"

	"github.com/spf13/cobra"
)

// tmplEntry 内置配置模板条目（文件名 + _comment 适用说明）。
type tmplEntry struct {
	Name    string
	Comment string
}

// templateEntries 列出内置配置模板库（templates/*.json），读各模板的 _comment。
func templateEntries() ([]tmplEntry, error) {
	dirs, err := embedded.SkillFiles.ReadDir("ledger-accounting/templates")
	if err != nil {
		return nil, err
	}
	var out []tmplEntry
	for _, d := range dirs {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			continue
		}
		data, err := embedded.SkillFiles.ReadFile("ledger-accounting/templates/" + d.Name())
		if err != nil {
			continue
		}
		var meta struct {
			Comment string `json:"_comment"`
		}
		_ = json.Unmarshal(data, &meta)
		out = append(out, tmplEntry{Name: d.Name(), Comment: meta.Comment})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// matchTemplate 按模板名匹配（支持 文件名 / 简称 / 带/不带 .json 后缀）。
func matchTemplate(entries []tmplEntry, name string) *tmplEntry {
	n := strings.TrimSuffix(strings.TrimPrefix(name, "print-config."), ".json")
	for i := range entries {
		base := strings.TrimSuffix(strings.TrimPrefix(entries[i].Name, "print-config."), ".json")
		if entries[i].Name == name || base == n || entries[i].Name == n+".json" {
			return &entries[i]
		}
	}
	return nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "打印版配置模板库（list 列出 / apply 应用内置模板）",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出内置配置模板（含适用说明）",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := templateEntries()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("模板库为空")
			return nil
		}
		fmt.Println("内置配置模板库（templates/）：")
		for _, e := range entries {
			fmt.Printf("  %-40s %s\n", e.Name, e.Comment)
		}
		fmt.Println("\n应用：ledger config apply <模板名> [-o 输出根目录] [-f]（-f 覆盖已有 print-config.json）")
		return nil
	},
}

var configApplyCmd = &cobra.Command{
	Use:   "apply <模板名>",
	Short: "把内置模板应用到 <输出根目录>/print-config.json（重新 generate 生效）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")
		entries, err := templateEntries()
		if err != nil {
			return err
		}
		m := matchTemplate(entries, args[0])
		if m == nil {
			fmt.Printf("未找到模板 %q。可用：ledger config list\n", args[0])
			return nil
		}
		target := filepath.Join(output, "print-config.json")
		if _, err := os.Stat(target); err == nil && !force {
			return fmt.Errorf("%s 已存在（如需覆盖请加 -f）", target)
		}
		data, err := embedded.SkillFiles.ReadFile("ledger-accounting/templates/" + m.Name)
		if err != nil {
			return fmt.Errorf("读取模板 %s: %w", m.Name, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("写入 %s: %w", target, err)
		}
		fmt.Printf("已应用模板 %s → %s\n%s\n重新 generate 即生效（自动发现，无需 --config）\n", m.Name, target, m.Comment)
		return nil
	},
}

func init() {
	configApplyCmd.Flags().StringP("output", "o", ".", "输出根目录（print-config.json 写入位置）")
	configApplyCmd.Flags().BoolP("force", "f", false, "覆盖已有的 print-config.json")
	configCmd.AddCommand(configListCmd, configApplyCmd)
	rootCmd.AddCommand(configCmd)
}
