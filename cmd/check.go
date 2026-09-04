package cmd

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ledger/balance"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	checkCmd.Flags().String("vs", "", "建账审核表 CSV 路径——逐项比对 JSON 期初调整额与老板批准的审核表（防录入漂移）")
	checkCmd.MarkFlagRequired("json")
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "检测 JSON 科目树、期初试算平衡与 xlsx 漂移",
	Long: "验证科目余额总览.json：① 科目树一致性（自动识别/手动调整科目一一对应）；② 期初试算平衡（借=贷）；" +
		"③ xlsx 漂移比对（最新月期末表 vs JSON 余额，防手工改表——-f 重建可修复）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		if err := balance.ValidateAccountTree(cfg); err != nil {
			return fmt.Errorf("科目树验证失败: %w", err)
		}
		fmt.Println("✓ 科目树验证通过")

		// 期初试算平衡校验（最新月份快照，借正贷负求和，0=平衡）
		if month := balance.LatestBalanceMonth(cfg); month != "" {
			if diff := balance.CheckInitialBalanceAt(cfg, month); diff != 0 {
				return fmt.Errorf("期初借贷不平衡（%s 快照），差额 %.2f 元（借正贷负）。请核对期初设置", month, float64(diff)/100)
			}
			fmt.Printf("✓ 期初试算平衡校验通过（%s 快照）\n", month)
		}

		// 未知类别科目提示 + 统计（v2 §2.5 可观测化：逐科目余额 + N 个/余额合计
		// 统计行——"未分类余额>0 连续 3 个月未处理或跨年带入"即科目体系事故）。
		// 余额取 ≤最新月的最近记录（对齐 GetInitBalanceForGenerate 回退语义，
		// 休眠科目不漏统计）
		latest := balance.LatestBalanceMonth(cfg)
		var unknownAccounts []string
		var unknownBalance int64
		for account := range cfg.Tree {
			gen := account
			if idx := strings.IndexByte(gen, '-'); idx > 0 {
				gen = gen[:idx]
			}
			if balance.IsUnknownType(gen) {
				unknownAccounts = append(unknownAccounts, account)
				if latest != "" {
					if bal, _, ok := balance.FinalAtOrBefore(cfg.Tree[account], latest); ok {
						unknownBalance += bal
					}
				}
			}
		}
		sort.Strings(unknownAccounts)
		for _, account := range unknownAccounts {
			bal := ""
			if latest != "" {
				if v, recMonth, ok := balance.FinalAtOrBefore(cfg.Tree[account], latest); ok {
					bal = fmt.Sprintf("（%s 余额 %+.2f 元）", recMonth, float64(v)/100)
				}
			}
			fmt.Printf("提示: 科目 %s 类别未知（属性=未分类）%s——不计入资产负债表/收支结余表合计；总账科目请使用官方科目表（财会〔2023〕14号）名称（或 map 映射到官方名），余额方向可用 ledger subjects import 指定\n", account, bal)
		}
		if len(unknownAccounts) > 0 {
			if latest == "" {
				fmt.Printf("⚠ 未分类科目 %d 个（无余额记录）——请补属性（subjects import）或纠名（map）\n", len(unknownAccounts))
			} else {
				fmt.Printf("⚠ 未分类科目 %d 个（余额合计 %+.2f 元）——请补属性（subjects import）或纠名（map）；余额非 0 连续 3 个月未处理或跨年带入即科目体系事故\n",
					len(unknownAccounts), float64(unknownBalance)/100)
			}
		}

		// xlsx 漂移比对（Change 12）：读最新月 xlsx 期末表，与 JSON Balances 对比（防手工改表漂移）
		if month := balance.LatestBalanceMonth(cfg); month != "" {
			xlsxPath := filepath.Join(filepath.Dir(configPath), month+".xlsx")
			f, err := excelize.OpenFile(xlsxPath)
			if err != nil {
				fmt.Printf("提示: 未找到 %s，跳过 xlsx 漂移比对\n", xlsxPath)
			} else {
				rows, _ := f.GetRows(month + "期末")
				f.Close()
				drift := 0
				checked := 0
				for _, row := range rows {
					if len(row) < 1 {
						continue
					}
					account := strings.TrimSpace(row[0])
					if account == "" || account == "科目" || account == "合计" {
						continue
					}
					node, ok := cfg.Tree[account]
					if !ok {
						continue
					}
					jsonFinal := node.Balances[month].Final
					// 按列号显式读借/贷金额（越界当 0）——验收修复：借方余额行 GetRows 仅 3 列，
					// 原 len(row)<4 跳过导致借方科目漂移漏检
					var debit, credit float64
					if len(row) > 2 {
						debit = parseYuanFloat(row[2])
					}
					if len(row) > 3 {
						credit = parseYuanFloat(row[3])
					}
					// 分精确舍入（验收修复：int64 截断会 0.29→0.28 误报）
					xFinal := int64(math.Round((debit - credit) * 100))
					if xFinal != jsonFinal {
						drift++
						fmt.Printf("⚠ 漂移: %s JSON=%.2f xlsx=%.2f\n", account, float64(jsonFinal)/100, float64(xFinal)/100)
					}
					checked++
				}
				if checked == 0 {
					fmt.Printf("提示: %s 期末表无可比对科目（可能未生成），跳过漂移比对\n", month)
				} else if drift == 0 {
					fmt.Printf("✓ xlsx 期末与 JSON 一致（%s，%d 个科目）\n", month, checked)
				} else {
					fmt.Printf("⚠ %d/%d 个科目 xlsx 与 JSON 漂移——以 JSON 为准，generate -f 可重建修复\n", drift, checked)
				}
			}
		}

		// 审核表逐项比对（迁移验收）：JSON 期初调整额 vs 建账审核表（权威批准件）
		if vsPath, _ := cmd.Flags().GetString("vs"); vsPath != "" {
			vsRows, err := parseReviewCSV(vsPath)
			if err != nil {
				return fmt.Errorf("加载审核表: %w", err)
			}
			actual := openingAdjustmentsOf(cfg)
			mismatch := 0
			unregistered := 0
			for _, r := range vsRows {
				expected := 0.0
				if r.OpeningSet && r.OpeningYuan != 0 {
					expected = r.OpeningYuan
					if r.Direction == "贷" {
						expected = -r.OpeningYuan
					}
				}
				if _, ok := cfg.Tree[r.Account]; !ok {
					unregistered++
					fmt.Printf("✗ 审核表科目未登记: %s（先 subjects import）\n", r.Account)
					continue
				}
				got := float64(actual[r.Account]) / 100
				if math.Abs(got-expected) >= 0.005 {
					mismatch++
					fmt.Printf("✗ 期初与审核表不一致: %s 审核表=%.2f JSON=%.2f\n", r.Account, expected, got)
				}
			}
			extra := 0
			for a, v := range actual {
				if v == 0 {
					continue
				}
				listed := false
				for _, r := range vsRows {
					if r.Account == a {
						listed = true
						break
					}
				}
				if !listed {
					extra++
					fmt.Printf("⚠ JSON 存在审核表未列出的期初调整: %s %+.2f 元（add-manual 补录？请同步审核表）\n", a, float64(v)/100)
				}
			}
			if unregistered > 0 || mismatch > 0 {
				return fmt.Errorf("期初与审核表比对失败: %d 项不一致, %d 个科目未登记——重跑 opening import 或修正审核表", mismatch, unregistered)
			}
			fmt.Printf("✓ 期初与审核表逐项一致（%d 个科目）\n", len(vsRows))
		}

		return nil
	},
}

// parseYuanFloat 解析金额字符串（如 "1,234.56" / "30000"）为元。
func parseYuanFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
