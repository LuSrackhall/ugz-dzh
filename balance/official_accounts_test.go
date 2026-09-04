package balance

import "testing"

// TestOfficialAccountsTableIntegrity 官方 42 科目常量表完整性
// （docs/account-code-design.md v2 §2.1 / 附录 A）。
func TestOfficialAccountsTableIntegrity(t *testing.T) {
	if got := len(officialAccounts); got != OfficialAccountCount {
		t.Fatalf("官方科目数 = %d, want %d", got, OfficialAccountCount)
	}
	if OfficialAccountCount != 42 {
		t.Fatalf("OfficialAccountCount = %d, want 42（财会〔2023〕14号）", OfficialAccountCount)
	}
	seenCode := map[string]bool{}
	seenName := map[string]bool{}
	for i, a := range officialAccounts {
		if a.Order != i+1 {
			t.Errorf("官方顺序号不连续: index %d order %d", i, a.Order)
		}
		if seenCode[a.Code] {
			t.Errorf("编码重复: %s", a.Code)
		}
		seenCode[a.Code] = true
		if seenName[a.Name] {
			t.Errorf("名称重复: %s", a.Name)
		}
		seenName[a.Name] = true
		switch a.Category {
		case "资产", "负债", "权益", "成本", "损益":
		default:
			t.Errorf("%s 官方大类非法: %q", a.Code, a.Category)
		}
		switch a.Type {
		case "资产", "负债", "权益", "收入", "费用":
		default:
			t.Errorf("%s 现行类别非法: %q", a.Code, a.Type)
		}
		if a.Property != "借" && a.Property != "贷" {
			t.Errorf("%s 属性非法: %q", a.Code, a.Property)
		}
		if a.Category == "损益" && a.Segment != "收入" && a.Segment != "支出" {
			t.Errorf("%s 损益类段位缺失: %q", a.Code, a.Segment)
		}
		if a.Category != "损益" && a.Segment != "" {
			t.Errorf("%s 非损益类不应有段位: %q", a.Code, a.Segment)
		}
	}
}

// TestOfficialAccountsSpotChecks 此前 17 类表缺失的科目补全后归类正确
// （e2e 实证：专项应付款-日间照料中心曾被归"未分类"）。
func TestOfficialAccountsSpotChecks(t *testing.T) {
	cases := []struct{ name, wantType, wantProp string }{
		{"专项应付款", "负债", "贷"},
		{"累计折旧", "资产", "借"},
		{"在建工程", "资产", "借"},
		{"库存物资", "资产", "借"},
		{"短期借款", "负债", "贷"},
		{"一事一议资金", "负债", "贷"},
		{"应付工资", "负债", "贷"},
		{"收益分配", "权益", "贷"},
		{"经营支出", "费用", "借"},
		{"税金及附加", "费用", "借"},
		{"所得税费用", "费用", "借"},
		{"生产(劳务)成本", "资产", "借"}, // 401 显式决策：现行类别=资产
	}
	for _, c := range cases {
		if got := accountTypes[c.name]; got != c.wantType {
			t.Errorf("accountTypes[%s] = %q, want %q", c.name, got, c.wantType)
		}
		if got := inferPropertyByType(c.name); got != c.wantProp {
			t.Errorf("inferPropertyByType(%s) = %q, want %q", c.name, got, c.wantProp)
		}
	}
}

// TestAccountTypeOfFrozenFiveTypes AccountTypeOf 返回值集合冻结为现行五类
// （v2 §2.1：报表 switch 只认五类字符串，引入官方大类会静默消失）。
func TestAccountTypeOfFrozenFiveTypes(t *testing.T) {
	for _, a := range officialAccounts {
		got, ok := AccountTypeOf(a.Name)
		if !ok {
			t.Fatalf("AccountTypeOf(%s) 未收录", a.Name)
		}
		switch got {
		case "资产", "负债", "权益", "收入", "费用":
		default:
			t.Errorf("AccountTypeOf(%s) = %q，超出现行五类", a.Name, got)
		}
	}
	if _, ok := AccountTypeOf("完全未知科目"); ok {
		t.Error("未知科目不应被收录")
	}
}

// TestOfficialAccountLookup 编码/别名查询。
func TestOfficialAccountLookup(t *testing.T) {
	if a, ok := OfficialAccountByCode("241"); !ok || a.Name != "专项应付款" {
		t.Errorf("OfficialAccountByCode(241) = %+v, ok=%v", a, ok)
	}
	if _, ok := OfficialAccountByCode("999"); ok {
		t.Error("OfficialAccountByCode(999) 不应命中")
	}
	if a, ok := OfficialAccountByName("生产（劳务）成本"); !ok || a.Code != "401" {
		t.Errorf("全角括号别名查询失败: %+v, ok=%v", a, ok)
	}
	if got := accountTypes["生产（劳务）成本"]; got != "资产" {
		t.Errorf("别名未进类别索引: %q", got)
	}
}
