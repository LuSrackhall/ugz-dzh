package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	voucherCmd.AddCommand(voucherCheckCmd)
	cf := voucherCheckCmd.Flags()
	cf.StringP("voucherDir", "v", "", "凭证月目录（如 vouchers/2026_03，必填）")
	cf.Bool("strict", false, "严格模式：发现问题以非零码退出（默认只告警）")
	cf.Bool("json", false, "以 JSON 输出检查结果（供 agent/MCP 消费）")
	voucherCheckCmd.MarkFlagRequired("voucherDir")

	voucherCmd.AddCommand(voucherListCmd)
	lf := voucherListCmd.Flags()
	lf.StringP("voucherDir", "v", "", "凭证月目录（如 vouchers/2026_03，必填）")
	lf.Bool("json", false, "以 JSON 输出凭证清单")
	voucherListCmd.MarkFlagRequired("voucherDir")
}

var voucherCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "检查凭证序列完整性与月目录卫生",
	Long: `检查月目录，判定三类问题：

  - 缺号（1..max 中缺失，可能是漏录或作废留空）
  - 重号（同一凭证号出现在多个文件）
  - 非正式文件名/位置（月目录内除 记字第XXXX号.md 之外的任何 .md，
    含子目录里的 .md——它们会被 generate 递归解析并静默计入账本）

默认只告警（退出码 0），--strict 才以非零码退出。
默认口径是为兼容既有账套：历史文件里可能存在作废留空号或带后缀的文件名，
若默认阻断将使这些账套无法继续生成（铁律一：历史文件永不修改）。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("voucherDir")
		strict, _ := cmd.Flags().GetBool("strict")
		asJSON, _ := cmd.Flags().GetBool("json")

		report, err := voucher.CheckSequence(dir)
		if err != nil {
			return err
		}

		if asJSON {
			b, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
		} else {
			fmt.Print(report.Summary())
		}

		if strict && !report.OK() {
			return fmt.Errorf("凭证序列检查未通过：%d 项问题（--strict）", len(report.Issues))
		}
		return nil
	},
}

// voucherListRow 一张凭证的清单行。
type voucherListRow struct {
	Num         int    `json:"num"`
	VoucherNo   string `json:"voucherNo"`
	File        string `json:"file"`
	Date        string `json:"date"`
	Summary     string `json:"summary"`
	EntryCount  int    `json:"entryCount"`
	DebitCents  int64  `json:"debitCents"`
	CreditCents int64  `json:"creditCents"`
}

var voucherListCmd = &cobra.Command{
	Use:   "list",
	Short: "列示某月凭证清单（凭证号/日期/摘要/借贷合计）",
	Long: `列示月目录中的正式凭证清单。

只列示符合正式命名（记字第XXXX号.md）且位于月目录根层的凭证；
非正式文件由 ledger voucher check 报告。本命令只读，不修改任何文件。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("voucherDir")
		asJSON, _ := cmd.Flags().GetBool("json")

		rows, err := collectVoucherList(dir)
		if err != nil {
			return err
		}

		if asJSON {
			b, err := json.MarshalIndent(rows, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}

		if len(rows) == 0 {
			fmt.Printf("目录 %s 中没有正式凭证（记字第XXXX号.md）\n", dir)
			return nil
		}
		var totalDebit, totalCredit int64
		fmt.Printf("凭证目录 %s：共 %d 张\n", dir, len(rows))
		fmt.Printf("%-14s %-12s %-16s %14s %14s  %s\n", "凭证号", "日期", "摘要", "借方合计", "贷方合计", "行数")
		for _, r := range rows {
			fmt.Printf("%-14s %-12s %-16s %14s %14s  %d\n",
				r.VoucherNo, r.Date, truncate(r.Summary, 8),
				voucher.FormatAmountCents(r.DebitCents), voucher.FormatAmountCents(r.CreditCents), r.EntryCount)
			totalDebit += r.DebitCents
			totalCredit += r.CreditCents
		}
		fmt.Printf("%-14s %-12s %-16s %14s %14s\n", "合计", "", "",
			voucher.FormatAmountCents(totalDebit), voucher.FormatAmountCents(totalCredit))
		return nil
	},
}

// collectVoucherList 读取月目录根层的正式凭证并按凭证号排序。
func collectVoucherList(dir string) ([]voucherListRow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取凭证目录 %s: %w", dir, err)
	}
	var rows []voucherListRow
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		num, ok := voucher.ParseVoucherFileNum(e.Name())
		if !ok {
			continue
		}
		path := filepath.Join(dir, e.Name())
		parsed, err := voucher.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("解析 %s: %w", path, err)
		}
		row := voucherListRow{Num: num, VoucherNo: voucher.VoucherFileNameNoExt(num), File: e.Name(), EntryCount: len(parsed)}
		for _, p := range parsed {
			row.DebitCents += p.DebitCents
			row.CreditCents += p.CreditCents
			if row.Date == "" {
				row.Date = p.Date
			}
			if row.Summary == "" {
				row.Summary = p.Summary
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Num < rows[j].Num })
	return rows, nil
}

// truncate 按显示宽度粗略截断（中文按 2 计），避免表格错位。
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
