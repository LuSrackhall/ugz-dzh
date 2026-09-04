package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"ledger/balance"
)

// 建账审核表 CSV 解析与共享校验（subjects import / opening import / check --vs 共用）。
// 表格式（列序无关，多余列忽略；UTF-8，带不带 BOM 均可）：
//
//	科目,方向,期初余额,备注
//	银行存款-工商银行,借,152300.00,
//	库存现金,借,2000.00,
//
// 「方向」= 该科目的正常余额方向（借/贷），同时决定科目属性；
// 「期初余额」一律填正数，方向含义由「方向」列表达；列留空 = 无余额科目（仅登记科目）。

type reviewRow struct {
	Account     string
	Direction   string  // 借 / 贷
	OpeningYuan float64 // 期初余额（正数）
	OpeningSet  bool    // 期初余额列非空（显式 0 也算填了，导入时按无余额处理）
	Note        string
}

// parseReviewCSV 解析建账审核表。必填列：科目、方向；可选列：期初余额、备注。
// 校验：方向 ∈ {借,贷}、期初余额非负、科目不重复。
func parseReviewCSV(path string) ([]reviewRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取审核表: %w", err)
	}
	content := strings.TrimPrefix(string(data), "\ufeff")

	r := csv.NewReader(strings.NewReader(content))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("审核表为空")
	}

	// 表头定位（列序无关）
	header := records[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}
	for _, required := range []string{"科目", "方向"} {
		if _, ok := colIdx[required]; !ok {
			return nil, fmt.Errorf("审核表缺少必填列「%s」（表头: %s）", required, strings.Join(header, ","))
		}
	}

	var rows []reviewRow
	seen := map[string]int{}
	for lineNo, rec := range records[1:] {
		// 整行空白跳过（含文件尾空行）
		allEmpty := true
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		row := reviewRow{}
		if i, ok := colIdx["科目"]; ok && i < len(rec) {
			row.Account = strings.TrimSpace(rec[i])
		}
		if row.Account == "" {
			return nil, fmt.Errorf("审核表第 %d 行科目为空", lineNo+2)
		}
		if i, ok := colIdx["方向"]; ok && i < len(rec) {
			row.Direction = strings.TrimSpace(rec[i])
		}
		if row.Direction != "借" && row.Direction != "贷" {
			return nil, fmt.Errorf("审核表第 %d 行科目 %s 方向无效 %q（必须为 借 或 贷）", lineNo+2, row.Account, row.Direction)
		}
		if i, ok := colIdx["期初余额"]; ok && i < len(rec) {
			cell := strings.TrimSpace(rec[i])
			if cell != "" {
				row.OpeningYuan = parseYuanFloat(cell)
				row.OpeningSet = true
				if row.OpeningYuan < 0 {
					return nil, fmt.Errorf("审核表第 %d 行科目 %s 期初余额为负（%.2f）——余额填正数，方向由「方向」列表达", lineNo+2, row.Account, row.OpeningYuan)
				}
			}
		}
		if i, ok := colIdx["备注"]; ok && i < len(rec) {
			row.Note = strings.TrimSpace(rec[i])
		}

		if prev, dup := seen[row.Account]; dup {
			return nil, fmt.Errorf("审核表科目重复: %s（第 %d 行与第 %d 行）", row.Account, prev, lineNo+2)
		}
		seen[row.Account] = lineNo + 2
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("审核表无数据行")
	}
	return rows, nil
}

// generalOf 取科目全路径的总账科目部分（第一个"-"前；无明细即全名）。
func generalOf(account string) string {
	if i := strings.IndexByte(account, '-'); i > 0 {
		return account[:i]
	}
	return account
}

// validateReviewRow 共享校验：科目本身不得为合并总账科目的父级（父级禁止直接记账/设期初，
// 子科目合法——与"科目 X 配置为合并总账科目，不能直接记账 → 用子科目"口径一致）。
func validateReviewRow(cfg *balance.GlobalConfig, r reviewRow) error {
	for _, m := range cfg.Settings.MergeGLAccounts {
		if r.Account == m {
			return fmt.Errorf("科目 %s 配置为合并总账科目，不能直接记账/设期初，请使用子科目（如 %s-明细）", r.Account, r.Account)
		}
	}
	return nil
}

// openingAdjustmentsOf 按科目归并非零期初调整额（分，借正贷负）。
// 与 GetInitBalanceForGenerate 优先级一致：AutoItems 先入、ManualItems 覆盖（同科目取手动值）。
func openingAdjustmentsOf(cfg *balance.GlobalConfig) map[string]int64 {
	m := map[string]int64{}
	for _, a := range cfg.AutoItems {
		if a.Adjustment != 0 {
			m[a.Account] = balance.YuanToCents(a.Adjustment)
		}
	}
	for _, it := range cfg.ManualItems {
		if it.Adjustment != 0 {
			m[it.Account] = balance.YuanToCents(it.Adjustment)
		}
	}
	return m
}
