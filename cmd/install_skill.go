package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"ledger/embedded"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(installSkillCmd)
	installSkillCmd.Flags().StringP("output", "o", "", "安装目标目录（默认 ~/.workbuddy/skills/）")
}

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "安装 ledger-accounting 会计技能到 WorkBuddy 技能目录",
	Long: "把内嵌（embed）的 ledger-accounting 技能（SKILL.md + references/）安装到指定目录，" +
		"供 agent 在对话中自动加载、全程指导记账（建账/科目管理/月结/结转/跨年/git 管理）。" +
		"默认安装到 ~/.workbuddy/skills/（用户级，跨项目可用）；可用 -o 指定。覆盖安装（幂等）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("output")
		if out == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("无法获取用户目录: %w", err)
			}
			out = filepath.Join(home, ".workbuddy", "skills")
		}
		target := filepath.Join(out, "ledger-accounting")
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("清理旧安装: %w", err)
		}
		n := 0
		err := fs.WalkDir(embedded.SkillFiles, "ledger-accounting", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel("ledger-accounting", path)
			dest := filepath.Join(target, rel)
			if d.IsDir() {
				return os.MkdirAll(dest, 0o755)
			}
			data, err := embedded.SkillFiles.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			n++
			return nil
		})
		if err != nil {
			return fmt.Errorf("安装技能: %w", err)
		}
		fmt.Printf("已安装 ledger-accounting 技能（%d 个文件）到 %s\n", n, target)
		fmt.Printf("提示: agent 在对话中涉及记账/建账/结转等操作时自动加载该技能\n")
		return nil
	},
}
