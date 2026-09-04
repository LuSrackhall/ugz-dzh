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
	ClosingMonth       string            `json:"结账月"` // Change 11：已结账的最后月份（<=该月拒绝无 -f 生成）
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
			gen, _ := splitPath(account)
			node = AccountNode{
				Property: inferPropertyByType(gen),
				FirstRecord: FirstRecord{
					Method: "自动识别",
					Month:  month,
					// 首月净额不进入期初链路（审计 M2/M1）：期初只来自调整额或余额链
					Amount: 0,
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

	// 补充回写：期初≠0 但当月无分录的科目（如 add-manual 建账期初、无活动但有余额科目），
	// 使其 Balances 与期初表/GL 期初行一致——否则 check 期初平衡校验会漏检这些科目的期初。
	for account, init := range initialBalances {
		if init == 0 {
			continue
		}
		if _, hasAct := activity[account]; hasAct {
			continue
		}
		node, ok := cfg.Tree[account]
		if !ok {
			continue
		}
		if node.Balances == nil {
			node.Balances = make(map[string]MonthBalance)
		}
		node.Balances[month] = MonthBalance{
			Initial: init,
			Debit:   0,
			Credit:  0,
			Final:   init,
		}
		cfg.Tree[account] = node
	}

	return nil
}

// GetInitBalanceForGenerate 获取某科目在某月的期初余额（分）。
// prevMonthEnd 为上月各科目的期末余额。
// 期初来源优先级（期初锚定建账月，铁律二：除建账月当月外，一律走连续链）：
//  1. 建账月（启动月==month）时：手动调整科目 / 自动识别科目 调整额≠0 → 期初=调整额
//  2. 上月期末 prevMonthEnd
//  3. 科目树中最近月份的期末余额（m < month）
//  4. 0
func GetInitBalanceForGenerate(cfg *GlobalConfig, account, month string, prevMonthEnd map[string]int64) int64 {
	// 1. 期初调整额只锚定建账月（启动月）：生成启动月时直取，其余月份一律续链
	if month == cfg.Settings.StartMonth {
		for _, m := range cfg.ManualItems {
			if m.Account == account && m.Adjustment != 0 {
				return YuanToCents(m.Adjustment)
			}
		}
		for _, a := range cfg.AutoItems {
			if a.Account == account && a.Adjustment != 0 {
				return YuanToCents(a.Adjustment)
			}
		}
	}

	// 2. 上月期末
	if end, ok := prevMonthEnd[account]; ok {
		return end
	}

	// 3. 从 JSON 科目树中取最近月份的期末余额作为期初
	node, ok := cfg.Tree[account]
	if ok {
		// 取最新月份的期末余额（含 0）——不得跳过期末=0 的月份回退到更早非零月
		// （审计二审 H1：年末结平科目跨年首月会凭空复活更早月余额）
		var latestMonth string
		var latestBal int64
		for m, mb := range node.Balances {
			if m < month && (latestMonth == "" || m > latestMonth) {
				latestMonth = m
				latestBal = mb.Final
			}
		}
		if latestMonth != "" {
			return latestBal
		}
	}

	return 0
}

// HasInitialAdjustment 判断某科目在某月的期初是否来自期初调整额（仅建账月/启动月）。
func HasInitialAdjustment(cfg *GlobalConfig, account, month string) bool {
	if month != cfg.Settings.StartMonth {
		return false
	}
	for _, m := range cfg.ManualItems {
		if m.Account == account && m.Adjustment != 0 {
			return true
		}
	}
	for _, a := range cfg.AutoItems {
		if a.Account == account && a.Adjustment != 0 {
			return true
		}
	}
	return false
}

// AddManualAdjustment 设置/修改手动调整科目的建账月期初值。
// 同科目已有条目 → 更新调整额/说明（修正期初）；无则追加。幂等，不报"已存在"。
// effectiveMonth 仅作信息性记录（补录时点），不参与期初计算（期初锚定建账月）。
func AddManualAdjustment(cfg *GlobalConfig, account, effectiveMonth string, adjustmentYuan float64, note string) error {
	amount := YuanToCents(adjustmentYuan)

	updated := false
	for i := range cfg.ManualItems {
		if cfg.ManualItems[i].Account == account {
			cfg.ManualItems[i].Adjustment = adjustmentYuan
			cfg.ManualItems[i].Note = note
			if effectiveMonth != "" {
				cfg.ManualItems[i].EffectiveMonth = effectiveMonth
			}
			updated = true
			break
		}
	}
	if !updated {
		cfg.ManualItems = append(cfg.ManualItems, ManualItem{
			Account:        account,
			EffectiveMonth: effectiveMonth,
			Adjustment:     adjustmentYuan,
			Note:           note,
		})
	}

	if cfg.Tree == nil {
		cfg.Tree = make(map[string]AccountNode)
	}
	node, exists := cfg.Tree[account]
	gen, _ := splitPath(account)
	if !exists {
		node = AccountNode{
			Property: inferPropertyByType(gen),
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

// --- 期初迁移与校验 ---

// inferPropertyByType 按总账科目类别推断属性：资产/费用→借，负债/权益/收入→贷，未知→未分类。
// 替代按首月净额推断（审计 M1：银行存款曾因首月净额为负被标为"贷"）。
// 未知科目返回"未分类"（设计专家审查 Change 11：防误归类，默认"借"会误导）。
func inferPropertyByType(general string) string {
	switch accountTypes[general] {
	case "负债", "权益", "收入":
		return "贷"
	case "资产", "费用":
		return "借"
	default:
		return "未分类"
	}
}

// PurgePhantomInitials 清理历史幻影期初（幂等，generate 加载 JSON 后自动调用）：
// 自动识别科目 FirstRecord.Amount 置 0；删除余额历史中 月份<首次月 且 借方==0 && 贷方==0 的记录
// （首次月之前不可能有发生额，此类记录只可能是旧 ensureBackfillForAll 回填产生）。
func PurgePhantomInitials(cfg *GlobalConfig) {
	for account, node := range cfg.Tree {
		if node.FirstRecord.Method != "自动识别" {
			continue
		}
		if node.FirstRecord.Amount != 0 {
			node.FirstRecord.Amount = 0
		}
		if node.FirstRecord.Month != "" && node.Balances != nil {
			for m, mb := range node.Balances {
				if m < node.FirstRecord.Month && mb.Debit == 0 && mb.Credit == 0 {
					delete(node.Balances, m)
				}
			}
		}
		cfg.Tree[account] = node
	}
}

// InitialBalanceDiff 返回某月期初映射的借贷差额（分）。借正贷负求和，0=平衡。
func InitialBalanceDiff(initials map[string]int64) int64 {
	var sum int64
	for _, v := range initials {
		sum += v
	}
	return sum
}

// LatestBalanceMonth 返回科目树中所有余额记录的最大月份（无记录返回 ""）。
func LatestBalanceMonth(cfg *GlobalConfig) string {
	var latest string
	for _, node := range cfg.Tree {
		for m := range node.Balances {
			if m > latest {
				latest = m
			}
		}
	}
	return latest
}

// CheckInitialBalanceAt 校验某月快照的期初借贷平衡（借正贷负求和）。返回差额（分）。
func CheckInitialBalanceAt(cfg *GlobalConfig, month string) int64 {
	var sum int64
	for _, node := range cfg.Tree {
		if mb, ok := node.Balances[month]; ok {
			sum += mb.Initial
		}
	}
	return sum
}

// IsUnknownType 判断总账科目类别是否未收录（未知类别默认按"借"处理）。
func IsUnknownType(general string) bool {
	_, ok := accountTypes[general]
	return !ok
}

// AccountTypeOf 返回总账科目类别（资产/负债/权益/收入/费用），未知返回 ("", false)。
func AccountTypeOf(general string) (string, bool) {
	t, ok := accountTypes[general]
	return t, ok
}

// --- 月份辅助 ---

func prevMonth(m string) string {
	yy := int(m[0]-'0')*1000 + int(m[1]-'0')*100 + int(m[2]-'0')*10 + int(m[3]-'0')
	mm := int(m[5]-'0')*10 + int(m[6]-'0')
	mm--
	if mm < 1 {
		mm = 12
		yy--
		if yy < 0 {
			return ""
		}
	}
	return fmt.Sprintf("%04d-%02d", yy, mm)
}

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

// accountTypes（名称→现行类别）已迁移至 official_accounts.go：
// 由财会〔2023〕14号官方 42 科目常量表生成（docs/account-code-design.md v2 §2.1）。
