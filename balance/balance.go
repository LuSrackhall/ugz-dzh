package balance

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"ledger/voucher"
)

// --- JSON 结构体 ---

// GlobalConfig 对应 科目余额总览.json 的顶层结构。
type GlobalConfig struct {
	Settings    GlobalSettings         `json:"全局设置"`
	Tree        map[string]AccountNode `json:"科目树"`
	AutoItems   []AutoItem             `json:"自动识别科目"`
	ManualItems []ManualItem           `json:"手动调整科目"`
	DetailOrder map[string][]string    `json:"明细列顺序,omitempty"` // 多科目明细账列序配置
}

// GlobalSettings 全局设置。
type GlobalSettings struct {
	StartMonth         string            `json:"启动月"`
	Order              []string          `json:"科目顺序"`
	AccountMap         map[string]string `json:"科目映射表"`
	MergeGLAccounts    []string          `json:"合并总账科目"`
	GLSuppressAccounts []string          `json:"总分类账忽略科目"`
	MLSuppressAccounts []string          `json:"多科目明细账忽略科目"`
}

// AccountNode 科目树中的一个节点（叶子科目）。
type AccountNode struct {
	Property    string                  `json:"科目属性"`
	FirstRecord FirstRecord             `json:"首次记录"`
	Balances    map[string]MonthBalance `json:"余额"`
}

// FirstRecord 科目的首次记录信息。
type FirstRecord struct {
	Method string `json:"方式"`
	Month  string `json:"月份"`
	Amount int64  `json:"金额"` // 分
}

// MonthBalance 某月的余额快照。
type MonthBalance struct {
	Initial int64 `json:"期初"`
	Debit   int64 `json:"借方"`
	Credit  int64 `json:"贷方"`
	Final   int64 `json:"期末"`
}

// AutoItem 自动识别科目条目。
type AutoItem struct {
	Account    string  `json:"科目"`
	FirstMonth string  `json:"首次月份"`
	Adjustment float64 `json:"期初调整额"` // 元
}

// ManualItem 手动调整科目条目。
type ManualItem struct {
	Account        string  `json:"科目"`
	EffectiveMonth string  `json:"生效月"`
	Adjustment     float64 `json:"期初调整额"`
	Note           string  `json:"说明"`
}

// --- Leaf 汇总 ---

// LeafSummary 叶子科目（总账科目 + "-" + 明细科目）的期间汇总。
type LeafSummary struct {
	FullPath    string `json:"fullPath"`
	General     string `json:"general"`
	Detail      string `json:"detail"`
	AccountType string `json:"accountType"`
	DebitTotal  int64  `json:"debitTotal"`
	CreditTotal int64  `json:"creditTotal"`
	Balance     int64  `json:"balance"`
	Direction   string `json:"direction"`
}

// ComputeLeafSummaries 按叶子科目（全路径）汇总所有分录。
func ComputeLeafSummaries(entries []voucher.Entry) []LeafSummary {
	type agg struct {
		debit  int64
		credit int64
	}
	m := make(map[string]*agg)
	for _, e := range entries {
		path := fullPath(e.GeneralAccount, e.DetailAccount)
		a, ok := m[path]
		if !ok {
			a = &agg{}
			m[path] = a
		}
		a.debit += e.DebitCents
		a.credit += e.CreditCents
	}

	summaries := make([]LeafSummary, 0, len(m))
	for path, a := range m {
		net := a.debit - a.credit
		direction := "平"
		balance := net
		if net > 0 {
			direction = "借"
		} else if net < 0 {
			direction = "贷"
			balance = -net
		}
		gen, det := splitPath(path)
		summaries = append(summaries, LeafSummary{
			FullPath:    path,
			General:     gen,
			Detail:      det,
			AccountType: classifyAccount(gen),
			DebitTotal:  a.debit,
			CreditTotal: a.credit,
			Balance:     balance,
			Direction:   direction,
		})
	}

	sortLeafSummaries(summaries)
	return summaries
}

// ComputeSummariesWithParents 在叶子科目基础上追加父级（总账科目）汇总行。
// 有明细科目的总账科目会生成一个汇总行，汇总该科目下所有明细的借贷合计。
func ComputeSummariesWithParents(entries []voucher.Entry) []LeafSummary {
	leaves := ComputeLeafSummaries(entries)

	groups := make(map[string][]LeafSummary)
	for _, s := range leaves {
		groups[s.General] = append(groups[s.General], s)
	}

	seen := make(map[string]bool)
	var result []LeafSummary
	for _, s := range leaves {
		if seen[s.General] {
			continue
		}
		seen[s.General] = true

		children := groups[s.General]
		hasDetail := false
		for _, c := range children {
			if c.Detail != "" {
				hasDetail = true
				break
			}
		}
		if !hasDetail {
			continue
		}

		var parentDebit, parentCredit int64
		for _, c := range children {
			parentDebit += c.DebitTotal
			parentCredit += c.CreditTotal
		}
		net := parentDebit - parentCredit
		direction := "平"
		balance := net
		if net > 0 {
			direction = "借"
		} else if net < 0 {
			direction = "贷"
			balance = -net
		}

		result = append(result, LeafSummary{
			FullPath:    s.General,
			General:     s.General,
			Detail:      "",
			AccountType: s.AccountType,
			DebitTotal:  parentDebit,
			CreditTotal: parentCredit,
			Balance:     balance,
			Direction:   direction,
		})
	}

	result = append(result, leaves...)
	sortLeafSummariesWithParents(result)
	return result
}

// GetLeafAccounts 返回所有叶子科目全路径（去重排序）。
func GetLeafAccounts(entries []voucher.Entry) []string {
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[fullPath(e.GeneralAccount, e.DetailAccount)] = true
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// --- JSON 读写 ---

// LoadConfig 从文件加载全局配置。
func LoadConfig(path string) (*GlobalConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}
	var cfg GlobalConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}

	// normalize: 确保旧配置缺失字段时不为 nil，避免生成时 nil slice 行为不一致
	if cfg.Settings.MergeGLAccounts == nil {
		cfg.Settings.MergeGLAccounts = []string{}
	}
	if cfg.Settings.GLSuppressAccounts == nil {
		cfg.Settings.GLSuppressAccounts = []string{}
	}
	if cfg.Settings.MLSuppressAccounts == nil {
		cfg.Settings.MLSuppressAccounts = []string{}
	}

	return &cfg, nil
}

// SaveConfig 保存全局配置到文件（格式化缩进，键排序，git diff 友好）。
func SaveConfig(path string, cfg *GlobalConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("写入配置 %s: %w", path, err)
	}
	return nil
}

// --- 辅助 ---

// CentsToYuan 分转元字符串。
func CentsToYuan(c int64) string {
	if c == 0 {
		return "0"
	}
	return fmt.Sprintf("%.2f", float64(c)/100.0)
}

// YuanToCents 元（float64）转分，四舍五入。
func YuanToCents(y float64) int64 {
	return int64(math.Round(y * 100))
}

// InferAccountProperty 根据金额推断科目属性。金额 >= 0 则 "借"，否则 "贷"。
func InferAccountProperty(amount int64) string {
	if amount >= 0 {
		return "借"
	}
	return "贷"
}

// --- 余额管理与期初计算 ---

// Activity 某一科目在当月发生额的借/贷合计。
type Activity struct {
	Debit  int64
	Credit int64
}

// UpdateBalancesAfterGenerate 每月 xlsx 生成成功后调用，回写余额历史。
// month 为当月标识（如 "2026-01"）；activity 为当月有发生的科目及其借/贷合计（分）；
// initialBalances 为该月各科目期初（分）。
func UpdateBalancesAfterGenerate(cfg *GlobalConfig, month string, activity map[string]Activity, initialBalances map[string]int64) error {
	if cfg.Tree == nil {
		cfg.Tree = make(map[string]AccountNode)
	}

	for account, act := range activity {
		node, exists := cfg.Tree[account]
		if !exists {
			node = AccountNode{
				Property: InferAccountProperty(act.Debit - act.Credit),
				FirstRecord: FirstRecord{
					Method: "自动识别",
					Month:  month,
					Amount: act.Debit - act.Credit,
				},
				Balances: make(map[string]MonthBalance),
			}
			cfg.Tree[account] = node

			found := false
			for _, a := range cfg.AutoItems {
				if a.Account == account {
					found = true
					break
				}
			}
			if !found {
				cfg.AutoItems = append(cfg.AutoItems, AutoItem{
					Account:    account,
					FirstMonth: month,
					Adjustment: 0,
				})
			}
		}

		initBal := int64(0)
		if ib, ok := initialBalances[account]; ok {
			initBal = ib
		}
		finalBal := initBal + act.Debit - act.Credit

		node.Balances[month] = MonthBalance{
			Initial: initBal,
			Debit:   act.Debit,
			Credit:  act.Credit,
			Final:   finalBal,
		}
		cfg.Tree[account] = node
	}

	ensureBackfillForAll(cfg, month)
	return nil
}

// GetInitBalanceForGenerate 获取某科目在某月的期初余额（分）。
// prevMonthEnd 为上月各科目的期末余额。
func GetInitBalanceForGenerate(cfg *GlobalConfig, account, month string, prevMonthEnd map[string]int64) int64 {
	if end, ok := prevMonthEnd[account]; ok {
		return end
	}

	node, ok := cfg.Tree[account]
	if !ok {
		return 0
	}
	if node.FirstRecord.Month == month {
		return node.FirstRecord.Amount
	}

	return 0
}

// AddManualAdjustment 添加手动调整科目到配置。
func AddManualAdjustment(cfg *GlobalConfig, account, effectiveMonth string, adjustmentYuan float64, note string) error {
	amount := YuanToCents(adjustmentYuan)

	for _, m := range cfg.ManualItems {
		if m.Account == account && m.EffectiveMonth == effectiveMonth {
			return fmt.Errorf("手动调整科目 %s 在 %s 已存在", account, effectiveMonth)
		}
	}

	cfg.ManualItems = append(cfg.ManualItems, ManualItem{
		Account:        account,
		EffectiveMonth: effectiveMonth,
		Adjustment:     adjustmentYuan,
		Note:           note,
	})

	if cfg.Tree == nil {
		cfg.Tree = make(map[string]AccountNode)
	}
	node, exists := cfg.Tree[account]
	if !exists {
		node = AccountNode{
			Property: InferAccountProperty(amount),
			FirstRecord: FirstRecord{
				Method: "手动调整",
				Month:  effectiveMonth,
				Amount: amount,
			},
			Balances: make(map[string]MonthBalance),
		}
	} else {
		node.FirstRecord = FirstRecord{
			Method: "手动调整",
			Month:  effectiveMonth,
			Amount: amount,
		}
	}
	cfg.Tree[account] = node

	ensureBackfillForAll(cfg, effectiveMonth)
	return nil
}

// SetAccountProperty 设置科目属性（"借"/"贷"）。
func SetAccountProperty(cfg *GlobalConfig, account, property string) error {
	if property != "借" && property != "贷" {
		return fmt.Errorf("无效的科目属性 %q，必须为 \"借\" 或 \"贷\"", property)
	}
	node, ok := cfg.Tree[account]
	if !ok {
		return fmt.Errorf("科目 %s 不存在于科目树中", account)
	}
	node.Property = property
	cfg.Tree[account] = node
	return nil
}

// ValidateAccountTree 验证科目树为自动识别+手动调整的合集，无遗漏无多余。
func ValidateAccountTree(cfg *GlobalConfig) error {
	expected := make(map[string]bool)
	for _, a := range cfg.AutoItems {
		expected[a.Account] = true
	}
	for _, m := range cfg.ManualItems {
		expected[m.Account] = true
	}

	for account := range cfg.Tree {
		if !expected[account] {
			return fmt.Errorf("科目树中存在多余科目 %s（不在自动识别和手动调整列表中）", account)
		}
	}

	for account := range expected {
		if _, ok := cfg.Tree[account]; !ok {
			return fmt.Errorf("科目 %s 在列表中但不在科目树中", account)
		}
	}

	return nil
}

// --- 期初前推 ---

// ensureBackfillForAll 遍历所有科目，若调整额 ≠ 0，则从启动月到首次记录月-1 补齐余额记录。
func ensureBackfillForAll(cfg *GlobalConfig, currentMonth string) {
	start := cfg.Settings.StartMonth
	if start == "" {
		return
	}

	for account, node := range cfg.Tree {
		if node.FirstRecord.Amount == 0 {
			continue
		}
		frMonth := node.FirstRecord.Month
		if frMonth == "" || cmpMonth(frMonth, start) <= 0 {
			continue
		}

		if node.Balances == nil {
			node.Balances = make(map[string]MonthBalance)
		}

		for m := start; cmpMonth(m, frMonth) < 0; m = nextMonth(m) {
			if _, exists := node.Balances[m]; !exists {
				node.Balances[m] = MonthBalance{
					Initial: node.FirstRecord.Amount,
					Debit:   0,
					Credit:  0,
					Final:   node.FirstRecord.Amount,
				}
			}
		}
		cfg.Tree[account] = node
	}
}

// --- 月份辅助 ---

func cmpMonth(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func nextMonth(m string) string {
	yy := int(m[0]-'0')*1000 + int(m[1]-'0')*100 + int(m[2]-'0')*10 + int(m[3]-'0')
	mm := int(m[5]-'0')*10 + int(m[6]-'0')
	mm++
	if mm > 12 {
		mm = 1
		yy++
	}
	return fmt.Sprintf("%04d-%02d", yy, mm)
}

func fullPath(general, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return general
	}
	return general + "-" + detail
}

func splitPath(path string) (string, string) {
	idx := strings.IndexByte(path, '-')
	if idx >= 0 && idx < len(path)-1 {
		return path[:idx], path[idx+1:]
	}
	return path, ""
}

func sortLeafSummaries(s []LeafSummary) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].AccountType != s[j].AccountType {
			return typeOrder(s[i].AccountType) < typeOrder(s[j].AccountType)
		}
		return s[i].FullPath < s[j].FullPath
	})
}

func sortLeafSummariesWithParents(s []LeafSummary) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].AccountType != s[j].AccountType {
			return typeOrder(s[i].AccountType) < typeOrder(s[j].AccountType)
		}
		if s[i].General == s[j].General {
			if s[i].Detail == "" && s[j].Detail != "" {
				return true
			}
			if s[i].Detail != "" && s[j].Detail == "" {
				return false
			}
		}
		if s[i].General != s[j].General {
			return s[i].General < s[j].General
		}
		return s[i].FullPath < s[j].FullPath
	})
}

func classifyAccount(account string) string {
	if t, ok := accountTypes[account]; ok {
		return t
	}
	return "费用"
}

func typeOrder(t string) int {
	switch t {
	case "资产":
		return 1
	case "负债":
		return 2
	case "权益":
		return 3
	case "收入":
		return 4
	case "费用":
		return 5
	default:
		return 6
	}
}

var accountTypes = map[string]string{
	"库存现金": "资产",
	"银行存款": "资产",
	"应收款":   "资产",
	"内部往来": "资产",
	"长期投资": "资产",
	"固定资产": "资产",
	"应付款":   "负债",
	"资本":     "权益",
	"公积公益金": "权益",
	"经营收入": "收入",
	"其他收入": "收入",
	"投资收益": "收入",
	"补助收入": "收入",
	"管理费用": "费用",
	"公益支出": "费用",
	"其他支出": "费用",
}
