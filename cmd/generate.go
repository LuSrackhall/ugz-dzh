package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ledger/balance"
	"ledger/generator"
	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("voucherDir", "v", "", "凭证 .md 文件所在目录（必填）")
	generateCmd.Flags().StringP("output", "o", ".", "输出根目录")
	generateCmd.Flags().BoolP("force", "f", false, "覆盖已有 xlsx")
	generateCmd.Flags().BoolP("verbose", "V", false, "输出详细日志")
	generateCmd.Flags().StringP("platform", "p", "auto", "打印版目标平台: auto(当前系统)/mac/windows")
	generateCmd.Flags().String("config", "", "打印版配置文件 (print-config.json)：显式 --config 优先；未传则自动发现 当前工作目录 → 输出根目录（ledger init 会自动生成模板）。英文键格式 platforms.{windows,mac}.{colScale,rowScale,fonts.{normal,digit,title,default}}，文件须 UTF-8 无 BOM；只作用于打印版 print/ 目录，须重新 generate 后查看新文件")
	generateCmd.Flags().Bool("allow-new", false, "显式允许自动登记凭证中未定义的新科目（旧行为：自动加入科目树，属性=名称匹配官方表或未分类；默认拒绝——先定义后生成）")
	generateCmd.MarkFlagRequired("voucherDir")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成月度账本",
	Long:  "解析凭证文件（必须全部来自同一年同一月），自动推导年份和月份，生成 CSV、XLSX 分录表和完整的累计 Excel 工作薄。",
	RunE: func(cmd *cobra.Command, args []string) error {
		voucherDir, _ := cmd.Flags().GetString("voucherDir")
		output, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")
		platform, _ := cmd.Flags().GetString("platform")
		printConfigPath, _ := cmd.Flags().GetString("config")
		generator.PrintPlatform = platform // 先定平台，配置加载与生成都基于它
		// 自动发现：未显式传 --config 时，依次查找 当前工作目录 → 输出根目录 下的 print-config.json；
		// 都无 → 自动创建默认配置模板到输出根目录（不允许"无配置"状态：配置永远显式存在、可见可改）
		autoConfig := ""
		if printConfigPath == "" {
			for _, dir := range []string{".", output} {
				cand := filepath.Join(dir, "print-config.json")
				if _, err := os.Stat(cand); err == nil {
					autoConfig = cand
					printConfigPath = cand
					break
				}
			}
			if printConfigPath == "" {
				created := filepath.Join(output, "print-config.json")
				if err := os.WriteFile(created, []byte(generator.PrintConfigTemplate()), 0o644); err != nil {
					return fmt.Errorf("创建默认打印版配置 %s: %w", created, err)
				}
				fmt.Printf("已创建默认打印版配置（可修改后重新 generate）: %s\n", created)
				autoConfig = created
				printConfigPath = created
			}
		}
		if err := generator.LoadPrintConfig(printConfigPath); err != nil {
			return err
		}
		if printConfigPath != "" {
			if autoConfig != "" {
				fmt.Printf("打印版配置已自动发现: %s → %s\n", autoConfig, generator.CurrentConfigSummary())
			} else {
				fmt.Printf("打印版配置已加载: %s → %s\n", printConfigPath, generator.CurrentConfigSummary())
			}
		} else {
			fmt.Printf("打印版配置: 默认 → %s（把 print-config.json 放当前目录或输出根目录即自动生效）\n", generator.CurrentConfigSummary())
		}

		// 收集所有凭证
		entries, err := CollectEntries(voucherDir)
		if err != nil {
			return fmt.Errorf("收集凭证: %w", err)
		}

		// 凭证号重号检测（Change 12）：文件名"记字第X号"数字重复 → 告警（如 记字第0005号.md 与 记字第0005号 更正.md）
		if files, _ := filepath.Glob(filepath.Join(voucherDir, "*.md")); len(files) > 0 {
			numMap := make(map[string]string)
			for _, f := range files {
				base := filepath.Base(f)
				m := voucherNumRe.FindStringSubmatch(base)
				if m == nil {
					continue
				}
				key := m[1] // 凭证号数字（含前导零）
				if prev, ok := numMap[key]; ok {
					fmt.Printf("警告: 凭证号重复 %q（%s 与 %s）——同一凭证号出现在两个文件，请检查是否漏账/重号\n", key, prev, base)
				} else {
					numMap[key] = base
				}
			}
		}
		if len(entries) == 0 {
			return fmt.Errorf("目录 %s 中没有解析到任何凭证分录", voucherDir)
		}

		// 同年同月校验 + 推导年份月份
		year, month, err := validateSameMonth(entries)
		if err != nil {
			return err
		}

		// 自动并入自动结转凭证（output/{year}/closing/，Change 10——系统生成的结转凭证，不污染手工目录）
		closingDir := filepath.Join(output, year, "closing")
		if closingFiles, _ := filepath.Glob(filepath.Join(closingDir, "*.md")); len(closingFiles) > 0 {
			for _, f := range closingFiles {
				ces, err := voucher.ParseFile(f)
				if err != nil {
					return fmt.Errorf("解析自动结转凭证 %s: %w", f, err)
				}
				entries = append(entries, ces...)
			}
			fmt.Printf("已并入 %d 张自动结转凭证（%s）\n", len(closingFiles), closingDir)
		}

		// 借贷平衡校验（审计 H2，CLI 内建安全：不平拒绝生成；含自动结转凭证）
		warnings, err := voucher.ValidateVoucherBalance(entries)
		if err != nil {
			return fmt.Errorf("凭证借贷平衡校验失败: %w", err)
		}
		for _, w := range warnings {
			fmt.Printf("提示: %s\n", w)
		}

		// 推导 JSON 路径: {output}/{year}/{year}.json
		yearDir := filepath.Join(output, year)
		configJSON := filepath.Join(yearDir, year+".json")

		if verbose {
			fmt.Printf("凭证目录: %s\n输出目录: %s/%s/\n月份: %s\n配置: %s\n", voucherDir, output, year, month, configJSON)
		}

		// 加载配置并应用映射
		cfg, err := balance.LoadConfig(configJSON)
		if err != nil {
			return fmt.Errorf("加载配置 %s: %w", configJSON, err)
		}

		// 结账标记（Change 11）：已结账月份默认拒绝生成（防误改），-f 例外
		if cfg.Settings.ClosingMonth != "" && month <= cfg.Settings.ClosingMonth && !force {
			return fmt.Errorf("月份 %s 已结账（结账月 %s）——如确认需修改，请使用 -f 强制重建（从该月起级联）", month, cfg.Settings.ClosingMonth)
		}
		if len(cfg.Settings.AccountMap) > 0 {
			ApplyAccountMap(entries, cfg.Settings.AccountMap)
			if verbose {
				fmt.Printf("已应用 %d 条科目名称映射\n", len(cfg.Settings.AccountMap))
			}
		}

		if verbose {
			fmt.Printf("收集到 %d 条原始分录\n", len(entries))
		}

		// 筛选当月分录
		entries = FilterByMonth(entries, month)
		if verbose {
			fmt.Printf("按月份 %s 筛选后剩余 %d 条分录\n", month, len(entries))
		}
		if len(entries) == 0 {
			return fmt.Errorf("月份 %s 没有匹配的凭证分录", month)
		}

		// 先定义后生成（docs/account-code-design.md v2 §2.4）：凭证科目必须已在
		// 科目树中定义，未定义科目默认拒绝生成并输出清单与指引；--allow-new 显式
		// 逃生（旧行为：静默自动登记）。
		allowNew, _ := cmd.Flags().GetBool("allow-new")
		if undefined := undefinedVoucherSubjects(cfg, entries); len(undefined) > 0 && !allowNew {
			fmt.Printf("⚠ 凭证中存在 %d 个未定义科目（科目树中没有）：\n", len(undefined))
			for _, u := range undefined {
				fmt.Printf("  - %s（出现 %d 次，样例摘要：%s）\n", u.Account, u.Count, u.Sample)
			}
			fmt.Println("处理方式（二选一）：")
			fmt.Println("  ① 登记: ledger subjects scan -v <凭证目录>（迁移场景以旧账期末科目余额表转录为基底，scan 产物并入双向 diff）→ 确认方向 → ledger subjects import -f 审核表.csv → 重新 generate")
			fmt.Println("  ② 若为 OCR 错名: ledger map add -f <错名> -t <对名> 后重跑")
			fmt.Println("  或显式使用 --allow-new 允许自动登记（不推荐：属性可能为未分类）")
			return fmt.Errorf("存在 %d 个未定义科目，拒绝生成（先定义后生成）", len(undefined))
		}

		if err := os.MkdirAll(yearDir, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}

		// 写入 CSV/XLSX 分录汇总
		if err := WriteCSV(yearDir, entries); err != nil {
			return fmt.Errorf("写入 CSV: %w", err)
		}
		if err := WriteXLSX(yearDir, entries); err != nil {
			return fmt.Errorf("写入 XLSX: %w", err)
		}

		summaries := balance.ComputeSummariesWithParents(entries)
		if err := WriteBalanceCSV(yearDir, summaries); err != nil {
			return fmt.Errorf("写入余额 CSV: %w", err)
		}
		if err := WriteBalanceXLSX(yearDir, summaries); err != nil {
			return fmt.Errorf("写入余额 XLSX: %w", err)
		}

		// 生成月度累计工作薄
		xlsxPath := filepath.Join(yearDir, month+".xlsx")
		if force {
			// 级联删除当月及之后所有月份的账本 xlsx（当月旧文件必须删除，否则 NewWorkbook 会加载旧当月文件
			// 并把旧版当月期末当作"上月期末"，导致 -f 重建期初被污染）
			// 仅删除账本文件（YYYY-MM.xlsx），绝不误删 ledger.xlsx / balance.xlsx 汇总文件
			entries, err := os.ReadDir(yearDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if isMonthXlsxName(name) && strings.TrimSuffix(name, ".xlsx") >= month {
						path := filepath.Join(yearDir, name)
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("删除 %s: %w", path, err)
						}
						if verbose {
							fmt.Printf("已删除: %s\n", path)
						}
					}
				}
			}
			// 同步清理 print/ 子目录中当月及之后月份的打印版 xlsx
			printDir := filepath.Join(yearDir, "print")
			if pentries, err := os.ReadDir(printDir); err == nil {
				for _, entry := range pentries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if isMonthXlsxName(name) && strings.TrimSuffix(name, ".xlsx") >= month {
						path := filepath.Join(printDir, name)
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("删除 %s: %w", path, err)
						}
						if verbose {
							fmt.Printf("已删除: %s\n", path)
						}
					}
				}
			}
		} else {
			if _, err := os.Stat(xlsxPath); err == nil {
				// M3 幂等保护：已含当月"本月合计"行视为已生成，要求 -f；
				// year-close 预生成的空工作薄（无本月合计行）放行直接生成
				if done, err := generator.AlreadyGenerated(xlsxPath); err == nil && done {
					return fmt.Errorf("%s 已生成过（含本月合计行），使用 -f 覆盖重建", xlsxPath)
				}
			}
		}
		if err := generator.GenerateWorkbook(configJSON, month, yearDir, entries); err != nil {
			return fmt.Errorf("生成工作薄: %w", err)
		}

		// 生成打印版位格 xlsx（失败仅告警，不影响已落盘的查看版）
		printPath := filepath.Join(yearDir, "print", month+".xlsx")
		generator.PrintPlatform = platform
		if err := generator.TransformToPrint(xlsxPath, printPath); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 生成打印版失败（查看版已成功）: %v\n", err)
		} else if verbose {
			fmt.Printf("已生成打印版: %s\n", printPath)
		}

		fmt.Printf("已生成 %s/%s 工作薄，共 %d 条分录\n", year, month, len(entries))
		return nil
	},
}

// undefinedSubject 分录中科目树未定义的科目（先定义后生成，v2 §2.4）。
type undefinedSubject struct {
	Account string // 全路径
	General string
	Count   int    // 出现行数
	Sample  string // 首次出现的摘要
}

// entryFullPath 拼科目全路径（与 balance.fullPath 同口径：明细空 → 仅总账）。
func entryFullPath(e voucher.Entry) string {
	general := strings.TrimSpace(e.GeneralAccount)
	detail := strings.TrimSpace(e.DetailAccount)
	if detail == "" {
		return general
	}
	return general + "-" + detail
}

// undefinedVoucherSubjects 找出分录中科目树未定义的科目（按名称排序）。
func undefinedVoucherSubjects(cfg *balance.GlobalConfig, entries []voucher.Entry) []undefinedSubject {
	freq := map[string]*undefinedSubject{}
	for _, e := range entries {
		account := entryFullPath(e)
		if strings.TrimSpace(e.GeneralAccount) == "" {
			continue
		}
		c, ok := freq[account]
		if !ok {
			general, _ := splitEntryPath(account)
			c = &undefinedSubject{Account: account, General: general, Sample: e.Summary}
			freq[account] = c
		}
		c.Count++
	}
	accounts := make([]string, 0, len(freq))
	for a := range freq {
		accounts = append(accounts, a)
	}
	sort.Strings(accounts)
	var out []undefinedSubject
	for _, a := range accounts {
		if _, ok := cfg.Tree[a]; !ok {
			out = append(out, *freq[a])
		}
	}
	return out
}

// splitEntryPath 按首个 '-' 切分全路径（与 balance.splitPath 同口径）。
func splitEntryPath(path string) (string, string) {
	if i := strings.IndexByte(path, '-'); i > 0 && i < len(path)-1 {
		return path[:i], path[i+1:]
	}
	return path, ""
}

// isMonthXlsxName 判断文件名是否为账本文件（YYYY-MM.xlsx），避免 -f 级联删除误伤 ledger.xlsx / balance.xlsx 汇总文件。
func isMonthXlsxName(name string) bool {
	trimmed := strings.TrimSuffix(name, ".xlsx")
	if len(trimmed) != 7 || trimmed[4] != '-' {
		return false
	}
	if _, err := strconv.Atoi(trimmed[:4]); err != nil {
		return false
	}
	if _, err := strconv.Atoi(trimmed[5:]); err != nil {
		return false
	}
	return true
}

// validateSameMonth 校验所有凭证是否来自同一年同一月，返回推导的年份和月份。
func validateSameMonth(entries []voucher.Entry) (year, month string, err error) {
	if len(entries) == 0 {
		return "", "", fmt.Errorf("没有分录可校验")
	}

	expected := ""
	for _, e := range entries {
		if len(e.Date) < 7 {
			return "", "", fmt.Errorf("分录日期格式无效: %q", e.Date)
		}
		m := e.Date[:7]
		if expected == "" {
			expected = m
		} else if m != expected {
			return "", "", fmt.Errorf("凭证目录中包含不同月份的分录: %s 与 %s。请确保所有凭证来自同一年同一月", expected, m)
		}
	}

	year = strings.Split(expected, "-")[0]
	month = expected
	return year, month, nil
}

// voucherNumRe 从文件名提取凭证号数字（Change 12 重号检测）。
var voucherNumRe = regexp.MustCompile(`记字第\D*0*([0-9]{1,6})`)
