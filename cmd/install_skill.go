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
	installSkillCmd.Flags().Bool("real-workbuddy", false, "WorkBuddy 技能目录用真实复制（软链接不被加载时的回退）")
}

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "安装 ledger-accounting 会计技能（标准源 .agents/skills + 软链接接入各工具）",
	Long: "把内嵌（embed）的 ledger-accounting 技能安装到项目级，一套真实文件、多工具可用：\n" +
		"  ① .agents/skills/ledger-accounting/    真实文件（Agent Skills 开放标准源：dsh/Cursor/Copilot 原生读）\n" +
		"  ② .claude/skills/ledger-accounting     软链接（Claude Code，DeepSeek Harness 同款做法）\n" +
		"  ③ .workbuddy/skills/ledger-accounting  软链接（WorkBuddy；--real-workbuddy 时用真实复制）\n" +
		"覆盖安装（幂等）。技能为安装产物，不进 git（可从二进制重建）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		realWorkbuddy, _ := cmd.Flags().GetBool("real-workbuddy")
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("无法定位可执行文件: %w", err)
		}
		root := filepath.Dir(exe) // 项目根（CLI 所在目录）

		// ① 标准源：.agents/skills/ledger-accounting/（真实文件）
		agentsTarget := filepath.Join(root, ".agents", "skills", "ledger-accounting")
		if err := os.RemoveAll(agentsTarget); err != nil {
			return fmt.Errorf("清理旧安装: %w", err)
		}
		n, err := writeSkillTree(agentsTarget)
		if err != nil {
			return fmt.Errorf("写入标准源: %w", err)
		}

		// ② .claude/skills/ledger-accounting → 软链接（Claude Code）
		claudeLink := filepath.Join(root, ".claude", "skills", "ledger-accounting")
		claudeMode, err := linkSkill(claudeLink, agentsTarget)
		if err != nil {
			return fmt.Errorf("接入 Claude Code: %w", err)
		}

		// ③ .workbuddy/skills/ledger-accounting（WorkBuddy；默认软链接，--real-workbuddy 复制）
		wbTarget := filepath.Join(root, ".workbuddy", "skills", "ledger-accounting")
		var wbMode string
		if realWorkbuddy {
			if err := os.RemoveAll(wbTarget); err != nil {
				return fmt.Errorf("清理旧安装: %w", err)
			}
			if _, err := writeSkillTree(wbTarget); err != nil {
				return fmt.Errorf("写入 WorkBuddy 副本: %w", err)
			}
			wbMode = "真实复制（--real-workbuddy）"
		} else {
			wbMode, err = linkSkill(wbTarget, agentsTarget)
			if err != nil {
				return fmt.Errorf("接入 WorkBuddy: %w", err)
			}
		}

		fmt.Printf("已安装 ledger-accounting 技能（%d 个文件，真实源）\n", n)
		fmt.Printf("  标准源(真实): %s\n", agentsTarget)
		fmt.Printf("  Claude Code:  %s（%s）\n", claudeLink, claudeMode)
		fmt.Printf("  WorkBuddy:    %s（%s）\n", wbTarget, wbMode)
		if !realWorkbuddy && wbMode == "软链接" {
			fmt.Printf("提示: 若 WorkBuddy 会话中未加载到该技能（软链接不被跟随），请重跑: ledger install-skill --real-workbuddy\n")
		}
		fmt.Printf("提示: 改技能内容后重跑本命令即可全工具同步；安装产物不进 git（embedded/ 为 git 源）\n")
		return nil
	},
}

// writeSkillTree 把内嵌技能写出到 target（真实文件），返回文件数。
func writeSkillTree(target string) (int, error) {
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
	return n, err
}

// linkSkill 接入技能：优先相对路径软链接；失败（如 Windows 无符号链接权限）自动回退复制。
// 返回实际接入方式（"软链接" / "复制（软链接不可用…）"）。
func linkSkill(link, agentsTarget string) (string, error) {
	_ = os.RemoveAll(link) // 幂等：先清旧链接/旧副本
	rel, err := filepath.Rel(filepath.Dir(link), agentsTarget)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	if serr := os.Symlink(rel, link); serr == nil {
		return "软链接", nil
	} else if _, werr := writeSkillTree(link); werr != nil {
		return "", fmt.Errorf("软链接失败(%v) 且回退复制失败(%v)", serr, werr)
	} else {
		return "复制（软链接不可用，可能无符号链接权限——Windows 需开发者模式/管理员）", nil
	}
}
