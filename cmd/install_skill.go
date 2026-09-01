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

// userLevelAgentDirs 各 agent 的用户级（home）专属技能目录（相对 home）。
// 用于检测历史版本/误操作把技能装到用户级而非仓库级的残留，提示用户清理。
// 覆盖常见 agent：WorkBuddy / Claude Code / Cursor / Codex / Gemini CLI / Windsurf / Zed 等。
var userLevelAgentDirs = []struct {
	name string // agent 名
	dir  string // 相对 home
}{
	{"WorkBuddy", filepath.Join(".workbuddy", "skills", "ledger-accounting")},
	{"Claude Code", filepath.Join(".claude", "skills", "ledger-accounting")},
	{"Cursor", filepath.Join(".cursor", "skills", "ledger-accounting")},
	{"OpenAI Codex", filepath.Join(".codex", "skills", "ledger-accounting")},
	{"Gemini CLI", filepath.Join(".gemini", "skills", "ledger-accounting")},
	{"Windsurf", filepath.Join(".windsurf", "skills", "ledger-accounting")},
	{"Zed", filepath.Join(".zed", "skills", "ledger-accounting")},
}

func init() {
	rootCmd.AddCommand(installSkillCmd)
	installSkillCmd.Flags().Bool("real-workbuddy", false, "WorkBuddy 技能目录用真实复制（软链接不被加载时的回退）")
	installSkillCmd.Flags().String("select", "", "选择接入的 agent（如 \"1,2\" / \"all\"；缺省时交互询问，非终端默认全部）")
	installSkillCmd.Flags().Bool("keep-user-level", false, "保留用户级（home 下）已安装的技能残留（默认检测到则提示移除，推荐仅保留仓库级）")
}

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "安装 ledger-accounting 会计技能（标准源 .agents/skills + 按选择接入各工具）",
	Long: "把内嵌（embed）的 ledger-accounting 技能安装到项目级：\n" +
		"  .agents/skills/ledger-accounting/  始终安装（真实文件，Agent Skills 开放标准源：dsh/Cursor/Copilot 原生读）\n" +
		"  再按选择接入各工具（软链接优先，失败自动降级复制）：WorkBuddy / Claude Code / Cursor\n" +
		"交互：运行后按提示输入编号（逗号分隔）或 all；非终端环境（脚本/CI）默认接入全部。\n" +
		"用户级残留：自动检测 home 下各 agent 用户级技能目录（历史版本/误操作残留），提示移除——\n" +
		"技能应仅安装于仓库级（CLI 所在目录）；--keep-user-level 可保留。\n" +
		"覆盖安装（幂等）；安装产物不进 git（可从二进制重建）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		realWorkbuddy, _ := cmd.Flags().GetBool("real-workbuddy")
		selectFlag, _ := cmd.Flags().GetString("select")
		keepUserLevel, _ := cmd.Flags().GetBool("keep-user-level")

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
		// 版本联动：写入 VERSION 文件（= 当前 ledger 版本），doctor 校验 skill 与 CLI 是否匹配
		if err := os.WriteFile(filepath.Join(agentsTarget, "VERSION"), []byte(version+"\n"), 0o644); err != nil {
			return fmt.Errorf("写入版本标记: %w", err)
		}
		n++

		// ①.5 用户级残留检测：历史版本/误操作可能把技能装到 home 下用户级目录。
		// 技能应仅存在于仓库级（CLI 所在目录）——用户级残留会导致版本不一致与维护混乱。
		if err := checkUserLevelRemnants(keepUserLevel); err != nil {
			return err
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

// findUserLevelSkills 扫描 home 下各 agent 用户级技能目录，返回已存在 ledger-accounting 的目录列表。
// 覆盖 WorkBuddy / Claude Code / Cursor / Codex / Gemini CLI / Windsurf / Zed 等常见 agent。
// 返回的每一项是 {agent 名, 目录绝对路径}；home 不可用时返回空。
func findUserLevelSkills() [][2]string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	var found [][2]string
	for _, a := range userLevelAgentDirs {
		p := filepath.Join(home, a.dir)
		if st, err := os.Lstat(p); err == nil && st.IsDir() {
			found = append(found, [2]string{a.name, p})
		}
	}
	return found
}

// checkUserLevelRemnants 检测并处理用户级技能残留。
// 技能应仅安装于仓库级（CLI 所在目录）——home 下用户级残留是历史版本/误操作的产物，
// 会造成版本不一致（doctor 版本匹配按仓库级校验，用户级旧版本静默失效）与维护混乱。
// 行为：
//   - 无残留：静默返回。
//   - 有残留且 --keep-user-level：仅提示，不删除。
//   - 有残留且交互终端：列出并询问是否移除（默认 y=移除）。
//   - 有残留且非终端（agent/脚本）：默认移除（推荐仅保留仓库级）。
func checkUserLevelRemnants(keep bool) error {
	found := findUserLevelSkills()
	if len(found) == 0 {
		return nil
	}
	fmt.Printf("\n检测到 %d 处用户级技能残留（技能应仅安装于仓库级，即 CLI 所在目录的 .agents/skills/）：\n", len(found))
	for _, f := range found {
		fmt.Printf("  - %s: %s\n", f[0], f[1])
	}
	if keep {
		fmt.Println("已指定 --keep-user-level，保留用户级残留（不推荐——用户级旧版本与仓库级不同步，doctor 无法校验）。")
		return nil
	}
	removeAll := true
	if isTerminal() {
		fmt.Print("是否移除以上用户级残留？[Y/n] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "n" || line == "no" {
			removeAll = false
		}
	}
	if !removeAll {
		fmt.Println("已保留用户级残留。提示：仓库级技能位于 CLI 所在目录 .agents/skills/，请确保以仓库级为准。")
		return nil
	}
	for _, f := range found {
		if err := os.RemoveAll(f[1]); err != nil {
			fmt.Printf("  ! 移除 %s 失败: %v\n", f[1], err)
		} else {
			fmt.Printf("  已移除 %s\n", f[1])
		}
	}
	return nil
}
