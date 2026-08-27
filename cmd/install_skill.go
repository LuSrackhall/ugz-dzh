package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ledger/embedded"

	"github.com/spf13/cobra"
)

// skillTargets 各 agent 的接入目录（相对项目根）。
var skillTargets = []struct {
	key  string // 编号/标识
	name string // 显示名
	dir  string // 相对项目根
}{
	{"1", "WorkBuddy", filepath.Join(".workbuddy", "skills", "ledger-accounting")},
	{"2", "Claude Code", filepath.Join(".claude", "skills", "ledger-accounting")},
	{"3", "Cursor", filepath.Join(".cursor", "skills", "ledger-accounting")},
}

func init() {
	rootCmd.AddCommand(installSkillCmd)
	installSkillCmd.Flags().Bool("real-workbuddy", false, "WorkBuddy 技能目录用真实复制（软链接不被加载时的回退）")
	installSkillCmd.Flags().String("select", "", "选择接入的 agent（如 \"1,2\" / \"all\"；缺省时交互询问，非终端默认全部）")
}

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "安装 ledger-accounting 会计技能（标准源 .agents/skills + 按选择接入各工具）",
	Long: "把内嵌（embed）的 ledger-accounting 技能安装到项目级：\n" +
		"  .agents/skills/ledger-accounting/  始终安装（真实文件，Agent Skills 开放标准源：dsh/Cursor/Copilot 原生读）\n" +
		"  再按选择接入各工具（软链接优先，失败自动降级复制）：WorkBuddy / Claude Code / Cursor\n" +
		"交互：运行后按提示输入编号（逗号分隔）或 all；非终端环境（脚本/CI）默认接入全部。\n" +
		"覆盖安装（幂等）；安装产物不进 git（可从二进制重建）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		realWorkbuddy, _ := cmd.Flags().GetBool("real-workbuddy")
		selectFlag, _ := cmd.Flags().GetString("select")

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("无法定位可执行文件: %w", err)
		}
		root := filepath.Dir(exe) // 项目根（CLI 所在目录）

		// ① 标准源：.agents/skills/ledger-accounting/（真实文件，始终安装）
		agentsTarget := filepath.Join(root, ".agents", "skills", "ledger-accounting")
		if err := os.RemoveAll(agentsTarget); err != nil {
			return fmt.Errorf("清理旧安装: %w", err)
		}
		n, err := writeSkillTree(agentsTarget)
		if err != nil {
			return fmt.Errorf("写入标准源: %w", err)
		}

		// 确定接入哪些工具
		selected, err := resolveSelection(selectFlag)
		if err != nil {
			return err
		}

		fmt.Printf("已安装 ledger-accounting 技能（%d 个文件，真实源）\n", n)
		fmt.Printf("  标准源(真实): %s\n", agentsTarget)

		for _, t := range skillTargets {
			if !selected[t.key] {
				continue
			}
			target := filepath.Join(root, t.dir)
			var mode string
			if t.key == "1" && realWorkbuddy {
				if err := os.RemoveAll(target); err != nil {
					return fmt.Errorf("清理旧安装: %w", err)
				}
				if _, err := writeSkillTree(target); err != nil {
					return fmt.Errorf("写入 %s 副本: %w", t.name, err)
				}
				mode = "真实复制（--real-workbuddy）"
			} else {
				mode, err = linkSkill(target, agentsTarget)
				if err != nil {
					return fmt.Errorf("接入 %s: %w", t.name, err)
				}
			}
			fmt.Printf("  %-12s: %s（%s）\n", t.name, target, mode)
		}

		if realWorkbuddy {
			fmt.Printf("提示: 若 WorkBuddy 会话中未加载到该技能（软链接不被跟随），请重跑: ledger install-skill --real-workbuddy\n")
		}
		fmt.Printf("提示: 改技能内容后重跑本命令即可全工具同步；安装产物不进 git（embedded/ 为 git 源）\n")
		return nil
	},
}

// resolveSelection 解析用户选择（--select 或交互），返回选中编号集合。
func resolveSelection(selectFlag string) (map[string]bool, error) {
	all := func() map[string]bool {
		m := make(map[string]bool)
		for _, t := range skillTargets {
			m[t.key] = true
		}
		return m
	}
	if selectFlag != "" {
		if selectFlag == "all" {
			return all(), nil
		}
		m := make(map[string]bool)
		for _, part := range strings.Split(selectFlag, ",") {
			k := strings.TrimSpace(part)
			valid := false
			for _, t := range skillTargets {
				if t.key == k || strings.EqualFold(t.name, k) {
					m[t.key] = true
					valid = true
				}
			}
			if !valid {
				return nil, fmt.Errorf("未知选择 %q（可选: 1=WorkBuddy, 2=Claude Code, 3=Cursor, all）", part)
			}
		}
		return m, nil
	}
	// 交互询问（仅终端）；非终端默认全部
	if !isTerminal() {
		return all(), nil
	}
	fmt.Println("选择要接入的 agent（.agents/skills/ 标准源始终安装；输入编号逗号分隔，或 all=全部，Enter=全部）:")
	for _, t := range skillTargets {
		fmt.Printf("  %s) %s\n", t.key, t.name)
	}
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" || line == "all" {
		return all(), nil
	}
	m := make(map[string]bool)
	for _, part := range strings.Split(line, ",") {
		k := strings.TrimSpace(part)
		for _, t := range skillTargets {
			if t.key == k || strings.EqualFold(t.name, k) {
				m[t.key] = true
			}
		}
	}
	if len(m) == 0 {
		fmt.Println("未选择有效项，默认接入全部")
		return all(), nil
	}
	return m, nil
}

// isTerminal 判断 stdin 是否为终端（交互输入）。
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
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
