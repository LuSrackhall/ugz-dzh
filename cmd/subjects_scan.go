package cmd

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ledger/balance"
	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	subjectsCmd.AddCommand(subjectsScanCmd)
	subjectsScanCmd.Flags().StringP("voucher", "v", "", "凭证目录（递归扫描其中 *.md，宽容解析；必填）")
	subjectsScanCmd.MarkFlagRequired("voucher")
	subjectsScanCmd.Flags().StringP("out", "o", "", "候选审核表输出路径（缺省打印到标准输出）")
}

// scanCandidate 凭证中扫出的候选科目。
type scanCandidate struct {
	Account string // 全路径（总账 或 总账-明细）
	General string
	Detail  string
	Count   int    // 出现行数
	Sample  string // 首次出现的凭证文件名
}

// scanVoucherSubjects 宽容扫描凭证目录（递归 *.md），提取（总账,明细）科目对与出现频率。
// 宽容解析（docs/account-code-design.md v2 §2.3）：只提取科目名，不要求凭证号可解析/
// 借贷平衡/同年同月——必须能吃下原始 OCR md（否则重蹈 output-harvest 被 generate
// 级阻断卡住的覆辙）。
func scanVoucherSubjects(root string) ([]scanCandidate, error) {
	// 根目录可能是符号链接（如工作树中 test_data → 主工作树）：WalkDir 的
	// 根用 Lstat 不跟随符号链接，这里先解析，保证符号链接根也能递归扫描。
	if fi, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("凭证目录 %s 不可访问: %w", root, err)
	} else if fi.IsDir() {
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		}
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描凭证目录 %s: %w", root, err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("凭证目录 %s 下未找到任何 .md 文件", root)
	}

	freq := map[string]*scanCandidate{}
	for _, f := range files {
		entries, err := voucher.ParseFile(f)
		if err != nil {
			fmt.Printf("跳过无法读取的文件 %s: %v\n", f, err)
			continue
		}
		base := filepath.Base(f)
		for _, e := range entries {
			if strings.TrimSpace(e.GeneralAccount) == "" {
				continue
			}
			account := entryFullPath(e)
			general, detail := splitEntryPath(account)
			c, ok := freq[account]
			if !ok {
				c = &scanCandidate{Account: account, General: general, Detail: detail, Sample: base}
				freq[account] = c
			}
			c.Count++
		}
	}

	accounts := make([]string, 0, len(freq))
	for a := range freq {
		accounts = append(accounts, a)
	}
	sort.Strings(accounts)
	out := make([]scanCandidate, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, *freq[a])
	}
	return out, nil
}

func scanIsNew(cfg *balance.GlobalConfig, c scanCandidate) bool {
	_, exists := cfg.Tree[c.Account]
	return !exists
}

// scanSuggestedDirection 建议方向：已登记且属性为借/贷 → 沿用；官方表
// （财会〔2023〕14号）收录 → 官方属性；否则空串——subjects import 拒绝空方向，
// 该空格即确认闸门。
func scanSuggestedDirection(cfg *balance.GlobalConfig, c scanCandidate) string {
	if node, ok := cfg.Tree[c.Account]; ok && (node.Property == "借" || node.Property == "贷") {
		return node.Property
	}
	if a, ok := balance.OfficialAccountByName(c.General); ok {
		return a.Property
	}
	return ""
}

// scanNote 候选行备注。
func scanNote(cfg *balance.GlobalConfig, c scanCandidate) string {
	if node, ok := cfg.Tree[c.Account]; ok {
		return fmt.Sprintf("已登记（属性 %s）——期初余额请与旧账期末余额表核对", node.Property)
	}
	if a, ok := balance.OfficialAccountByName(c.General); ok {
		return fmt.Sprintf("官方科目（%s %s）——期初余额请从旧账期末余额表转录", a.Code, a.Name)
	}
	return "未分类：请先确认名称（OCR 错名用 ledger map 归并）并补方向"
}

// scanCSVRows 候选审核表行。前四列与建账审核表兼容（科目/方向/期初余额/备注，
// 列序无关、多余列被 parseReviewCSV 忽略）→ 本产物可直接回喂 subjects import；
// 期初余额一律留空：金额来自旧账期末余额表转录（权威基底原则，scan 不造数）。
func scanCSVRows(cfg *balance.GlobalConfig, cands []scanCandidate) [][]string {
	sorted := make([]scanCandidate, len(cands))
	copy(sorted, cands)
	sort.Slice(sorted, func(i, j int) bool {
		ni, nj := scanIsNew(cfg, sorted[i]), scanIsNew(cfg, sorted[j])
		if ni != nj {
			return ni // 新科目在前（待办优先）
		}
		return sorted[i].Account < sorted[j].Account
	})
	rows := [][]string{{"科目", "方向", "期初余额", "备注", "出现次数", "样例凭证", "状态"}}
	for _, c := range sorted {
		status := "已登记"
		if scanIsNew(cfg, c) {
			status = "新科目"
		}
		rows = append(rows, []string{
			c.Account,
			scanSuggestedDirection(cfg, c),
			"",
			scanNote(cfg, c),
			fmt.Sprintf("%d", c.Count),
			c.Sample,
			status,
		})
	}
	return rows
}

var subjectsScanCmd = &cobra.Command{
	Use:   "scan -v <凭证目录> [-o 候选.csv]",
	Short: "扫凭证提取科目清单 → 候选建账审核表（迁移/确认用）",
	Long: "宽容扫描凭证目录（递归 *.md），提取出现的科目对（总账/明细）与出现频率，\n" +
		"生成候选建账审核表 CSV（科目/方向/期初余额/备注/出现次数/样例凭证/状态）。\n" +
		"权威基底原则：审核表基底=旧账期末科目余额表逐行转录，本命令产物仅并入做双向 diff；\n" +
		"「期初余额」一律留空（金额来自旧账，scan 不造数）。\n" +
		"方向建议：已登记沿用原属性；官方表（财会〔2023〕14号）收录按官方属性；否则留空待确认。\n" +
		"下一步：人工/agent 确认补全 → subjects import → opening import。",
	RunE: func(cmd *cobra.Command, args []string) error {
		vdir, _ := cmd.Flags().GetString("voucher")
		out, _ := cmd.Flags().GetString("out")
		configPath, _ := cmd.Flags().GetString("json")

		cands, err := scanVoucherSubjects(vdir)
		if err != nil {
			return err
		}
		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		rows := scanCSVRows(cfg, cands)
		var buf strings.Builder
		w := csv.NewWriter(&buf)
		_ = w.WriteAll(rows)
		w.Flush()
		if err := w.Error(); err != nil {
			return fmt.Errorf("生成 CSV: %w", err)
		}

		newCnt, existCnt := 0, 0
		for _, c := range cands {
			if scanIsNew(cfg, c) {
				newCnt++
			} else {
				existCnt++
			}
		}

		if out == "" {
			fmt.Print(buf.String())
		} else {
			if err := os.WriteFile(out, []byte(buf.String()), 0o644); err != nil {
				return fmt.Errorf("写入候选表: %w", err)
			}
			fmt.Printf("已生成候选审核表 → %s\n", out)
		}
		fmt.Printf("扫描完成: %d 个科目（新科目 %d，已登记 %d）\n", len(cands), newCnt, existCnt)
		fmt.Println("下一步: 与旧账期末科目余额表双向 diff（权威基底）→ 确认方向/转录期初 → ledger subjects import → ledger opening import")
		return nil
	},
}
