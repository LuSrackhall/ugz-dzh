package balance

// 官方科目常量表——财会〔2023〕14号《农村集体经济组织会计制度》42 个一级科目。
// 设计依据：docs/account-code-design.md v2 §2.1——
//   - 本表按"最终结构"定义（编码/官方大类/损益段位一次到位），是延后项
//     （科目编码主键化、报表按段位归类，触发器见设计稿 §四）的唯一留口子；
//   - 运行时当前只消费「现行类别」字段，AccountTypeOf 返回值集合冻结为
//     现行五类（资产/负债/权益/收入/费用）——报表 switch 只认这五个字符串，
//     引入"成本/损益"等官方大类作运行时类别会静默消失于报表之外；
//   - 401 生产(劳务)成本 现行类别显式决策为"资产"：年末余额经济实质是在
//     产品，应进资产负债表左列且不被年末清零；映射"费用"会被 gen-close
//     清零，留空则资产负债表漏项。官方大类记录为"成本"，P4 校正时改挂。

// OfficialAccount 官方一级科目定义。
type OfficialAccount struct {
	Order    int    // 官方顺序号（1~42，财会〔2023〕14号科目表次序）
	Code     string // 官方一级编码（101~521）
	Name     string // 科目名称（凭证/账本使用名）
	Property string // 属性：借 / 贷
	Category string // 官方大类：资产 / 负债 / 权益 / 成本 / 损益
	Segment  string // 损益段位：收入 / 支出（非损益类为空）
	Type     string // 现行类别：资产 / 负债 / 权益 / 收入 / 费用（运行时唯一消费字段）
}

// officialAccounts 官方 42 个一级科目（按官方顺序号排列）。
// 字段序：Order, Code, Name, Property, Category, Segment, Type
var officialAccounts = []OfficialAccount{
	// 一、资产类（1xx，属性借）——官方顺序号 1~19
	{1, "101", "库存现金", "借", "资产", "", "资产"},
	{2, "102", "银行存款", "借", "资产", "", "资产"},
	{3, "111", "短期投资", "借", "资产", "", "资产"},
	{4, "112", "应收款", "借", "资产", "", "资产"},
	{5, "113", "内部往来", "借", "资产", "", "资产"},
	{6, "121", "库存物资", "借", "资产", "", "资产"},
	{7, "131", "消耗性生物资产", "借", "资产", "", "资产"},
	{8, "132", "生产性生物资产", "借", "资产", "", "资产"},
	{9, "133", "生产性生物资产累计折旧", "借", "资产", "", "资产"},
	{10, "134", "公益性生物资产", "借", "资产", "", "资产"},
	{11, "141", "长期投资", "借", "资产", "", "资产"},
	{12, "151", "固定资产", "借", "资产", "", "资产"},
	{13, "152", "累计折旧", "借", "资产", "", "资产"},
	{14, "153", "在建工程", "借", "资产", "", "资产"},
	{15, "154", "固定资产清理", "借", "资产", "", "资产"},
	{16, "161", "无形资产", "借", "资产", "", "资产"},
	{17, "162", "累计摊销", "借", "资产", "", "资产"},
	{18, "171", "长期待摊费用", "借", "资产", "", "资产"},
	{19, "181", "待处理财产损溢", "借", "资产", "", "资产"},
	// 二、负债类（2xx，属性贷）——官方顺序号 20~27
	{20, "201", "短期借款", "贷", "负债", "", "负债"},
	{21, "211", "应付款", "贷", "负债", "", "负债"},
	{22, "212", "应付工资", "贷", "负债", "", "负债"},
	{23, "213", "应付劳务费", "贷", "负债", "", "负债"},
	{24, "214", "应交税费", "贷", "负债", "", "负债"},
	{25, "221", "长期借款及应付款", "贷", "负债", "", "负债"},
	{26, "231", "一事一议资金", "贷", "负债", "", "负债"},
	{27, "241", "专项应付款", "贷", "负债", "", "负债"},
	// 三、权益类（3xx，属性贷）——官方顺序号 28~31
	{28, "301", "资本", "贷", "权益", "", "权益"},
	{29, "311", "公积公益金", "贷", "权益", "", "权益"},
	{30, "321", "本年收益", "贷", "权益", "", "权益"},
	{31, "322", "收益分配", "贷", "权益", "", "权益"},
	// 四、成本类（4xx）——官方顺序号 32（现行类别显式决策=资产，见文件头注释）
	{32, "401", "生产(劳务)成本", "借", "成本", "", "资产"},
	// 五、损益类（5xx）——官方顺序号 33~42
	// 收入段（属性贷）
	{33, "501", "经营收入", "贷", "损益", "收入", "收入"},
	{34, "502", "投资收益", "贷", "损益", "收入", "收入"},
	{35, "503", "补助收入", "贷", "损益", "收入", "收入"},
	{36, "504", "其他收入", "贷", "损益", "收入", "收入"},
	// 支出段（属性借）
	{37, "511", "经营支出", "借", "损益", "支出", "费用"},
	{38, "512", "税金及附加", "借", "损益", "支出", "费用"},
	{39, "513", "管理费用", "借", "损益", "支出", "费用"},
	{40, "514", "公益支出", "借", "损益", "支出", "费用"},
	{41, "515", "其他支出", "借", "损益", "支出", "费用"},
	{42, "521", "所得税费用", "借", "损益", "支出", "费用"},
}

// officialNameAliases 常见书写变体 → 官方名。
// 系统性名称归一化（空格/全半角/同音字模糊匹配）延后至语音输入立项工作项，
// 此处仅收录高频变体，且同步进入 accountTypes 索引，保证三层匹配口径一致。
var officialNameAliases = map[string]string{
	"生产（劳务）成本": "生产(劳务)成本", // 全角括号变体
}

// buildAccountTypeIndex 由官方表构建 名称→现行类别 索引（含别名），
// 替代原 17 类硬编码 map。仍以名称为 key（三层精确匹配现状不变）。
func buildAccountTypeIndex() map[string]string {
	m := make(map[string]string, len(officialAccounts)+len(officialNameAliases))
	for _, a := range officialAccounts {
		m[a.Name] = a.Type
	}
	for alias, name := range officialNameAliases {
		for _, a := range officialAccounts {
			if a.Name == name {
				m[alias] = a.Type
				break
			}
		}
	}
	return m
}

// accountTypes 名称 → 现行类别（资产/负债/权益/收入/费用）。
// 由官方 42 科目表生成，消费方：inferPropertyByType / IsUnknownType /
// AccountTypeOf / classifyAccount（balance.go）。
var accountTypes = buildAccountTypeIndex()

// OfficialAccountByName 按名称查官方科目定义（含书写变体别名），未收录返回 false。
func OfficialAccountByName(name string) (OfficialAccount, bool) {
	if n, ok := officialNameAliases[name]; ok {
		name = n
	}
	for _, a := range officialAccounts {
		if a.Name == name {
			return a, true
		}
	}
	return OfficialAccount{}, false
}

// OfficialAccountByCode 按官方一级编码查科目定义，未收录返回 false。
func OfficialAccountByCode(code string) (OfficialAccount, bool) {
	for _, a := range officialAccounts {
		if a.Code == code {
			return a, true
		}
	}
	return OfficialAccount{}, false
}

// OfficialAccounts 返回全部官方科目（按官方顺序号升序的副本）。
func OfficialAccounts() []OfficialAccount {
	out := make([]OfficialAccount, len(officialAccounts))
	copy(out, officialAccounts)
	return out
}

// OfficialAccountCount 官方一级科目总数（42）。
const OfficialAccountCount = 42
