package balance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ledger/voucher"
)

func TestComputeLeafSummaries(t *testing.T) {
	entries := []voucher.Entry{
		{GeneralAccount: "库存现金", DetailAccount: "", DebitCents: 100000, CreditCents: 20000},
		{GeneralAccount: "库存现金", DetailAccount: "", DebitCents: 50000, CreditCents: 0},
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 0, CreditCents: 150000},
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 30000, CreditCents: 0},
		{GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 70000, CreditCents: 0},
		{GeneralAccount: "管理费用", DetailAccount: "差旅费", DebitCents: 30000, CreditCents: 0},
		{GeneralAccount: "经营收入", DetailAccount: "", DebitCents: 0, CreditCents: 80000},
	}

	summaries := ComputeLeafSummaries(entries)

	if len(summaries) != 5 {
		t.Fatalf("expected 5 leaf accounts, got %d", len(summaries))
	}

	// Verify type ordering: 资产 first
	if summaries[0].AccountType != "资产" {
		t.Errorf("first account type should be 资产, got %s", summaries[0].AccountType)
	}
	if summaries[0].FullPath != "库存现金" {
		t.Errorf("first account should be 库存现金, got %s", summaries[0].FullPath)
	}

	// Verify 管理费用 明细分类
	found := make(map[string]LeafSummary)
	for _, s := range summaries {
		found[s.FullPath] = s
	}

	if s, ok := found["管理费用-办公费"]; !ok || s.DebitTotal != 70000 || s.Direction != "借" {
		t.Errorf("管理费用-办公费 wrong: %+v", s)
	}
	if s, ok := found["管理费用-差旅费"]; !ok || s.DebitTotal != 30000 || s.Direction != "借" {
		t.Errorf("管理费用-差旅费 wrong: %+v", s)
	}
	if s, ok := found["银行存款-工行"]; !ok || s.CreditTotal != 150000 || s.Direction != "贷" || s.Balance != 120000 {
		t.Errorf("银行存款-工行 wrong: %+v", s)
	}

	// Verify full path format
	if summaries[0].General == "" {
		t.Error("General should not be empty")
	}
}

func TestComputeLeafSummariesEmpty(t *testing.T) {
	summaries := ComputeLeafSummaries(nil)
	if len(summaries) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(summaries))
	}
}

func TestComputeSummariesWithParents(t *testing.T) {
	entries := []voucher.Entry{
		{GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 70000, CreditCents: 0},
		{GeneralAccount: "管理费用", DetailAccount: "差旅费", DebitCents: 30000, CreditCents: 0},
		{GeneralAccount: "管理费用", DetailAccount: "办公费", DebitCents: 10000, CreditCents: 0},
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 0, CreditCents: 150000},
		{GeneralAccount: "银行存款", DetailAccount: "工行", DebitCents: 30000, CreditCents: 0},
		{GeneralAccount: "库存现金", DetailAccount: "", DebitCents: 100000, CreditCents: 20000},
	}

	summaries := ComputeSummariesWithParents(entries)

	// Should have: 管理费用(父) + 管理费用-办公费 + 管理费用-差旅费 + 银行存款(父) + 银行存款-工行 + 库存现金 = 6
	if len(summaries) != 6 {
		t.Fatalf("expected 6 rows (3 leaves + 2 parents + 1 leaf without detail), got %d", len(summaries))
	}

	// Verify ordering: 资产 first, parent before children
	assetStart := 0
	for assetStart < len(summaries) && summaries[assetStart].AccountType != "资产" {
		assetStart++
	}

	// 银行存款 parent should come before 银行存款-工行
	var bankParentIdx, bankChildIdx int
	for i, s := range summaries {
		if s.FullPath == "银行存款" && s.Detail == "" {
			bankParentIdx = i
		}
		if s.FullPath == "银行存款-工行" {
			bankChildIdx = i
		}
	}
	if bankParentIdx >= bankChildIdx {
		t.Errorf("银行存款 parent (idx %d) should come before 银行存款-工行 (idx %d)", bankParentIdx, bankChildIdx)
	}

	// Verify 管理费用 parent totals
	var mgmtParent LeafSummary
	for _, s := range summaries {
		if s.FullPath == "管理费用" && s.Detail == "" {
			mgmtParent = s
			break
		}
	}
	if mgmtParent.DebitTotal != 110000 || mgmtParent.CreditTotal != 0 {
		t.Errorf("管理费用 parent: debit=%d credit=%d, want debit=110000 credit=0", mgmtParent.DebitTotal, mgmtParent.CreditTotal)
	}

	// 库存现金 should have no parent row (no detail)
	for _, s := range summaries {
		if s.FullPath == "库存现金" && s.Detail != "" {
			t.Error("库存现金 should not have detail")
		}
	}
	// Count 库存现金 rows: should be exactly 1 (leaf only, no parent since no detail)
	stockCount := 0
	for _, s := range summaries {
		if s.General == "库存现金" {
			stockCount++
		}
	}
	if stockCount != 1 {
		t.Errorf("库存现金 should have exactly 1 row (no parent since no detail), got %d", stockCount)
	}
}

func TestComputeSummariesWithParentsEmpty(t *testing.T) {
	summaries := ComputeSummariesWithParents(nil)
	if len(summaries) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(summaries))
	}
}

func TestGetLeafAccounts(t *testing.T) {
	entries := []voucher.Entry{
		{GeneralAccount: "库存现金", DetailAccount: ""},
		{GeneralAccount: "银行存款", DetailAccount: "工行"},
		{GeneralAccount: "银行存款", DetailAccount: "建行"},
		{GeneralAccount: "库存现金", DetailAccount: ""},
	}
	paths := GetLeafAccounts(entries)
	if len(paths) != 3 {
		t.Fatalf("expected 3 unique leaf accounts, got %d", len(paths))
	}
	expected := map[string]bool{
		"库存现金":    true,
		"银行存款-工行": true,
		"银行存款-建行": true,
	}
	for _, p := range paths {
		if !expected[p] {
			t.Errorf("unexpected leaf account: %s", p)
		}
	}
}

func TestFullPath(t *testing.T) {
	tests := []struct {
		general, detail, want string
	}{
		{"库存现金", "", "库存现金"},
		{"管理费用", "办公费", "管理费用-办公费"},
		{"应收款", "  张三  ", "应收款-张三"},
	}
	for _, tt := range tests {
		got := fullPath(tt.general, tt.detail)
		if got != tt.want {
			t.Errorf("fullPath(%q, %q) = %q, want %q", tt.general, tt.detail, got, tt.want)
		}
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path       string
		wantGen    string
		wantDetail string
	}{
		{"库存现金", "库存现金", ""},
		{"管理费用-办公费", "管理费用", "办公费"},
		{"应收款-张三-李四", "应收款", "张三-李四"},
	}
	for _, tt := range tests {
		gen, det := splitPath(tt.path)
		if gen != tt.wantGen || det != tt.wantDetail {
			t.Errorf("splitPath(%q) = (%q, %q), want (%q, %q)", tt.path, gen, det, tt.wantGen, tt.wantDetail)
		}
	}
}

func TestLoadSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_config.json")

	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2026-01"},
		Tree: map[string]AccountNode{
			"库存现金": {
				Property:    "借",
				FirstRecord: FirstRecord{Method: "自动识别", Month: "2026-01", Amount: 0},
				Balances: map[string]MonthBalance{
					"2026-01": {Initial: 0, Debit: 60000, Credit: 41800, Final: 18200},
				},
			},
		},
		AutoItems: []AutoItem{
			{Account: "库存现金", FirstMonth: "2026-01", Adjustment: 0},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.Settings.StartMonth != "2026-01" {
		t.Errorf("StartMonth mismatch: %s", loaded.Settings.StartMonth)
	}
	if n, ok := loaded.Tree["库存现金"]; !ok || n.Property != "借" {
		t.Error("科目树 mismatch")
	}
	if loaded.Tree["库存现金"].Balances["2026-01"].Final != 18200 {
		t.Error("余额 mismatch")
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigJSONRoundTrip(t *testing.T) {
	// Verify the JSON structure matches 总纲领 spec
	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2026-01"},
		Tree: map[string]AccountNode{
			"其他应收款-张三": {
				Property:    "借",
				FirstRecord: FirstRecord{Method: "手动调整", Month: "2026-08", Amount: 150000},
				Balances: map[string]MonthBalance{
					"2026-01": {Initial: 150000, Debit: 0, Credit: 0, Final: 150000},
				},
			},
		},
		AutoItems: []AutoItem{
			{Account: "库存现金", FirstMonth: "2026-01", Adjustment: 0},
		},
		ManualItems: []ManualItem{
			{Account: "其他应收款-张三", EffectiveMonth: "2026-08", Adjustment: 1500, Note: "补录旧账"},
		},
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded GlobalConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify nested map
	item := decoded.Tree["其他应收款-张三"]
	if item.FirstRecord.Amount != 150000 {
		t.Errorf("Amount mismatch: %d", item.FirstRecord.Amount)
	}
	if len(decoded.ManualItems) != 1 {
		t.Fatalf("expected 1 ManualItem, got %d", len(decoded.ManualItems))
	}
	if decoded.ManualItems[0].Adjustment != 1500 {
		t.Errorf("Adjustment mismatch: %f", decoded.ManualItems[0].Adjustment)
	}
}

func TestYuanCentsRoundTrip(t *testing.T) {
	tests := []struct {
		yuan  float64
		cents int64
	}{
		{0, 0},
		{1.00, 100},
		{1.50, 150},
		{0.01, 1},
		{-1.00, -100},
		{100.00, 10000},
		{0.99, 99},
		{1.01, 101},
	}
	for _, tt := range tests {
		got := YuanToCents(tt.yuan)
		if got != tt.cents {
			t.Errorf("YuanToCents(%f) = %d, want %d", tt.yuan, got, tt.cents)
		}
	}
}

func TestInferAccountProperty(t *testing.T) {
	tests := []struct {
		amount int64
		want   string
	}{
		{0, "借"},
		{100, "借"},
		{-100, "贷"},
		{1, "借"},
		{-1, "贷"},
	}
	for _, tt := range tests {
		got := InferAccountProperty(tt.amount)
		if got != tt.want {
			t.Errorf("InferAccountProperty(%d) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}

func TestUpdateBalancesAfterGenerate(t *testing.T) {
	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2026-01"},
		Tree:     make(map[string]AccountNode),
	}

	activity := map[string]Activity{
		"库存现金":    {Debit: 500000, Credit: 300000},
		"银行存款-工行": {Debit: 0, Credit: 150000},
	}
	initialBalances := map[string]int64{
		"库存现金":    0,
		"银行存款-工行": 200000,
	}

	err := UpdateBalancesAfterGenerate(cfg, "2026-01", activity, initialBalances)
	if err != nil {
		t.Fatalf("UpdateBalancesAfterGenerate: %v", err)
	}

	// 库存现金 should be auto-created in tree and AutoItems
	stock, ok := cfg.Tree["库存现金"]
	if !ok {
		t.Fatal("库存现金 should exist in tree")
	}
	if stock.FirstRecord.Method != "自动识别" {
		t.Errorf("库存现金 first record method: %s", stock.FirstRecord.Method)
	}
	if bal, ok := stock.Balances["2026-01"]; !ok {
		t.Error("库存现金 missing 2026-01 balance")
	} else if bal.Final != 200000 {
		t.Errorf("库存现金 final = %d, want 200000", bal.Final)
	}

	// 银行存款-工行 should have correct final
	bank, ok := cfg.Tree["银行存款-工行"]
	if !ok {
		t.Fatal("银行存款-工行 should exist in tree")
	}
	if bal, ok := bank.Balances["2026-01"]; !ok {
		t.Error("银行存款-工行 missing 2026-01 balance")
	} else if bal.Final != 50000 {
		t.Errorf("银行存款-工行 final = %d, want 50000 (200000+0-150000)", bal.Final)
	}

	// AutoItems should contain both
	if len(cfg.AutoItems) != 2 {
		t.Errorf("expected 2 AutoItems, got %d", len(cfg.AutoItems))
	}

	// Validate tree
	if err := ValidateAccountTree(cfg); err != nil {
		t.Errorf("ValidateAccountTree: %v", err)
	}
}

func TestGetInitBalanceForGenerate(t *testing.T) {
	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2026-01"},
		Tree: map[string]AccountNode{
			"其他应收款-张三": {
				FirstRecord: FirstRecord{Method: "手动调整", Month: "2026-01", Amount: 150000},
			},
		},
		ManualItems: []ManualItem{
			{Account: "其他应收款-张三", EffectiveMonth: "2026-01", Adjustment: 1500.00},
		},
	}

	prev := map[string]int64{
		"库存现金":     500000,
		"其他应收款-张三": 160000, // 2026-01 期末（模拟续链）
	}

	// Priority 1: 建账月（启动月）→ 直取调整额
	if got := GetInitBalanceForGenerate(cfg, "其他应收款-张三", "2026-01", prev); got != 150000 {
		t.Errorf("其他应收款-张三 init = %d, want 150000（建账月调整额）", got)
	}

	// 非建账月续链：期初=上月期末，调整额不覆盖（铁律二）
	if got := GetInitBalanceForGenerate(cfg, "其他应收款-张三", "2026-02", prev); got != 160000 {
		t.Errorf("其他应收款-张三 2026-02 init = %d, want 160000（上月期末续链）", got)
	}

	// Priority 2: prevMonthEnd
	if got := GetInitBalanceForGenerate(cfg, "库存现金", "2026-02", prev); got != 500000 {
		t.Errorf("库存现金 init = %d, want 500000", got)
	}

	// Priority 3: 0
	if got := GetInitBalanceForGenerate(cfg, "未知科目", "2026-01", prev); got != 0 {
		t.Errorf("未知科目 init = %d, want 0", got)
	}
}

func TestGetInitBalanceCrossYearNoOverride(t *testing.T) {
	// 跨年：建账月为 2025-10，2026 各月非建账月 → 期初=上年末（prevFinals），调整额不覆盖
	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2025-10"},
		AutoItems: []AutoItem{
			{Account: "银行存款", FirstMonth: "2025-10", Adjustment: 5000.00}, // 5000 元 = 500000 分
		},
	}
	prev := map[string]int64{"银行存款": 12345600} // 2025-12 期末
	if got := GetInitBalanceForGenerate(cfg, "银行存款", "2026-01", prev); got != 12345600 {
		t.Errorf("跨年 2026-01 init = %d, want 12345600（上年末，调整额不覆盖）", got)
	}
	// 建账月（=启动月 2025-10）命中调整额
	if got := GetInitBalanceForGenerate(cfg, "银行存款", "2025-10", map[string]int64{}); got != 500000 {
		t.Errorf("2025-10 init = %d, want 500000（建账月调整额）", got)
	}
}

func TestPurgePhantomInitials(t *testing.T) {
	cfg := &GlobalConfig{
		Tree: map[string]AccountNode{
			// 自动识别科目：首次月前的幻影回填记录（无发生额）应被清理
			"银行存款": {
				FirstRecord: FirstRecord{Method: "自动识别", Month: "2025-10", Amount: 11811959},
				Balances: map[string]MonthBalance{
					"2025-01": {Initial: 11811959, Final: 11811959},
					"2025-05": {Initial: 11811959, Final: 11811959},
					"2025-10": {Initial: 0, Debit: 100, Credit: 200, Final: -100}, // 真实记录，保留
					"2025-11": {Initial: -100, Debit: 0, Credit: 0, Final: -100},  // 首次月后，保留
				},
			},
			// 手动调整科目不受影响
			"应收款-张三": {
				FirstRecord: FirstRecord{Method: "手动调整", Month: "2026-03", Amount: 150000},
			},
		},
	}
	PurgePhantomInitials(cfg)

	bank := cfg.Tree["银行存款"]
	if bank.FirstRecord.Amount != 0 {
		t.Errorf("自动科目 FirstRecord.Amount = %d, want 0", bank.FirstRecord.Amount)
	}
	if _, ok := bank.Balances["2025-01"]; ok {
		t.Error("幻影记录 2025-01 应被清理")
	}
	if _, ok := bank.Balances["2025-05"]; ok {
		t.Error("幻影记录 2025-05 应被清理")
	}
	if _, ok := bank.Balances["2025-10"]; !ok {
		t.Error("真实记录 2025-10 不应被清理")
	}
	if _, ok := bank.Balances["2025-11"]; !ok {
		t.Error("首次月后记录不应被清理")
	}
	if cfg.Tree["应收款-张三"].FirstRecord.Amount != 150000 {
		t.Error("手动调整科目不应被迁移")
	}
}

func TestInferPropertyByType(t *testing.T) {
	cases := map[string]string{
		"库存现金": "借", "银行存款": "借", "管理费用": "借",
		"应付款": "贷", "资本": "贷", "经营收入": "贷",
		"完全未知科目": "借",
	}
	for general, want := range cases {
		if got := inferPropertyByType(general); got != want {
			t.Errorf("inferPropertyByType(%s) = %s, want %s", general, got, want)
		}
	}
}

func TestInitialBalanceDiff(t *testing.T) {
	if d := InitialBalanceDiff(map[string]int64{"a": 100, "b": -100}); d != 0 {
		t.Errorf("balanced diff = %d, want 0", d)
	}
	if d := InitialBalanceDiff(map[string]int64{"a": 100, "b": -90}); d != 10 {
		t.Errorf("unbalanced diff = %d, want 10", d)
	}
}

func TestAddManualAdjustment(t *testing.T) {
	cfg := &GlobalConfig{
		Settings: GlobalSettings{StartMonth: "2026-01"},
		Tree:     make(map[string]AccountNode),
	}

	err := AddManualAdjustment(cfg, "其他应收款-李四", "2026-05", 2000.00, "补录旧账")
	if err != nil {
		t.Fatalf("AddManualAdjustment: %v", err)
	}

	// Check ManualItems
	if len(cfg.ManualItems) != 1 {
		t.Fatalf("expected 1 ManualItem, got %d", len(cfg.ManualItems))
	}
	if cfg.ManualItems[0].Adjustment != 2000.00 {
		t.Errorf("adjustment = %f, want 2000", cfg.ManualItems[0].Adjustment)
	}

	// Check tree
	node, ok := cfg.Tree["其他应收款-李四"]
	if !ok {
		t.Fatal("其他应收款-李四 should be in tree")
	}
	if node.FirstRecord.Method != "手动调整" {
		t.Errorf("method = %s, want 手动调整", node.FirstRecord.Method)
	}
	if node.FirstRecord.Amount != 200000 {
		t.Errorf("amount = %d, want 200000", node.FirstRecord.Amount)
	}

	// Duplicate: 幂等更新（修正建账月期初值），不报错
	err = AddManualAdjustment(cfg, "其他应收款-李四", "2026-05", 100.00, "重复")
	if err != nil {
		t.Errorf("重复 add-manual 应更新而非报错: %v", err)
	}
	if cfg.ManualItems[0].Adjustment != 100.00 {
		t.Errorf("调整额 = %f, want 100（已更新）", cfg.ManualItems[0].Adjustment)
	}
	updatedNode := cfg.Tree["其他应收款-李四"]
	if updatedNode.FirstRecord.Amount != 10000 {
		t.Errorf("FirstRecord.Amount = %d, want 10000（同步更新）", updatedNode.FirstRecord.Amount)
	}
}

func TestSetAccountProperty(t *testing.T) {
	cfg := &GlobalConfig{
		Tree: map[string]AccountNode{
			"库存现金": {Property: "借"},
		},
	}

	// Valid change
	if err := SetAccountProperty(cfg, "库存现金", "贷"); err != nil {
		t.Fatalf("SetAccountProperty: %v", err)
	}
	if cfg.Tree["库存现金"].Property != "贷" {
		t.Errorf("property = %s, want 贷", cfg.Tree["库存现金"].Property)
	}

	// Invalid property
	if err := SetAccountProperty(cfg, "库存现金", "平"); err == nil {
		t.Error("expected error for invalid property")
	}

	// Non-existent account
	if err := SetAccountProperty(cfg, "不存在", "借"); err == nil {
		t.Error("expected error for non-existent account")
	}
}

func TestValidateAccountTree(t *testing.T) {
	// Valid
	cfg := &GlobalConfig{
		Tree: map[string]AccountNode{
			"库存现金":   {},
			"应收款-张三": {},
		},
		AutoItems: []AutoItem{
			{Account: "库存现金"},
		},
		ManualItems: []ManualItem{
			{Account: "应收款-张三"},
		},
	}
	if err := ValidateAccountTree(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	// Extra in tree
	cfg2 := &GlobalConfig{
		Tree: map[string]AccountNode{
			"库存现金": {},
			"多余科目": {},
		},
		AutoItems: []AutoItem{{Account: "库存现金"}},
	}
	if err := ValidateAccountTree(cfg2); err == nil {
		t.Error("expected error for extra account in tree")
	}

	// Missing from tree
	cfg3 := &GlobalConfig{
		Tree:      map[string]AccountNode{},
		AutoItems: []AutoItem{{Account: "库存现金"}},
	}
	if err := ValidateAccountTree(cfg3); err == nil {
		t.Error("expected error for missing account in tree")
	}
}

func TestNextMonth(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"2026-01", "2026-02"},
		{"2026-12", "2027-01"},
		{"2026-06", "2026-07"},
	}
	for _, tt := range tests {
		got := nextMonth(tt.in)
		if got != tt.want {
			t.Errorf("nextMonth(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCmpMonth(t *testing.T) {
	if cmpMonth("2026-01", "2026-02") >= 0 {
		t.Error("2026-01 should be < 2026-02")
	}
	if cmpMonth("2026-02", "2026-01") <= 0 {
		t.Error("2026-02 should be > 2026-01")
	}
	if cmpMonth("2026-01", "2026-01") != 0 {
		t.Error("2026-01 should == 2026-01")
	}
}
