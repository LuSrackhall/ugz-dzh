package voucher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitAccountPath(t *testing.T) {
	cases := []struct {
		in                   string
		wantGeneral, wantDet string
	}{
		{"公益支出-补助费用", "公益支出", "补助费用"},
		{"银行存款", "银行存款", ""},
		{"", "", ""},
		{"-明细", "-明细", ""},              // 首字符即 '-'：不切分
		{"总账-", "总账-", ""},              // 尾字符即 '-'：不切分
		{"管理费用-办公费-子", "管理费用", "办公费-子"}, // 只在首个 '-' 切
	}
	for _, c := range cases {
		g, d := SplitAccountPath(c.in)
		if g != c.wantGeneral || d != c.wantDet {
			t.Errorf("SplitAccountPath(%q) = (%q,%q), want (%q,%q)", c.in, g, d, c.wantGeneral, c.wantDet)
		}
	}
}

func TestVoucherFileNameAndMonthDir(t *testing.T) {
	if got := VoucherFileName(5); got != "记字第0005号.md" {
		t.Errorf("VoucherFileName(5) = %q", got)
	}
	if got := VoucherFileName(12345); got != "记字第12345号.md" {
		t.Errorf("VoucherFileName(12345) = %q", got)
	}
	if got := VoucherFileNameNoExt(22); got != "记字第0022号" {
		t.Errorf("VoucherFileNameNoExt(22) = %q", got)
	}
	if got := MonthDirName("2026-03-05"); got != "2026_03" {
		t.Errorf("MonthDirName = %q", got)
	}
	if got := MonthDirName("bad"); got != "" {
		t.Errorf("MonthDirName(bad) = %q, want empty", got)
	}
}

func TestFormatAmountCents(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{100, "1.00"},
		{999000, "9,990.00"},
		{6319400, "63,194.00"},
		{100000000, "1,000,000.00"},
		{-50000, "(500.00)"},
		{-1, "(0.01)"},
	}
	for _, c := range cases {
		if got := FormatAmountCents(c.in); got != c.want {
			t.Errorf("FormatAmountCents(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderVoucherBasicShape(t *testing.T) {
	req := WriteRequest{
		Date:       "2026-03-05",
		VoucherNum: 7,
		Summary:    "付养老金",
		Attachment: 1,
		UnitName:   "红旗路办事处",
		Entries: []WriteEntry{
			{Account: "公益支出-补助费用", Side: SideDebit, AmountCents: 999000},
			{Account: "应付款-养老金", Side: SideCredit, AmountCents: 999000},
		},
	}
	got, err := RenderVoucher(req)
	if err != nil {
		t.Fatalf("RenderVoucher: %v", err)
	}
	for _, want := range []string{
		"红旗路办事处",
		"记字第0007号 1/1",
		"记帐凭证",
		"2026年03月05日",
		"附件 1 张",
		"<th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th>",
		"<td>付养老金</td><td>公益支出</td><td>补助费用</td><td>9,990.00</td><td></td>",
		"<td>付养老金</td><td>应付款</td><td>养老金</td><td></td><td>9,990.00</td>",
		"<tr><td>合计</td><td></td><td></td><td>9,990.00</td><td>9,990.00</td></tr>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q\n---\n%s", want, got)
		}
	}
}

// TestWriteVoucherRoundTrip 往返自检：写入器正确性由解析器裁定。
func TestWriteVoucherRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		req  WriteRequest
	}{
		{
			name: "单借多贷",
			req: WriteRequest{
				Date: "2026-04-30", VoucherNum: 1, Summary: "收街道拨社区经费", Attachment: 1,
				Entries: []WriteEntry{
					{Account: "银行存款", Side: SideDebit, AmountCents: 6319400},
					{Account: "补助收入-工作补助经费", Side: SideCredit, AmountCents: 5000000},
					{Account: "补助收入-工作补助经费", Side: SideCredit, AmountCents: 1319400},
				},
			},
		},
		{
			name: "含红字（同侧负数）",
			req: WriteRequest{
				Date: "2026-04-30", VoucherNum: 2, Summary: "冲销并重记", Attachment: 0,
				Entries: []WriteEntry{
					{Account: "银行存款", Side: SideDebit, AmountCents: 1000000},
					{Account: "银行存款", Side: SideDebit, AmountCents: -50000},
					{Account: "管理费用-办公费", Side: SideCredit, AmountCents: 950000},
				},
			},
		},
		{
			name: "行摘要覆盖凭证摘要",
			req: WriteRequest{
				Date: "2026-04-15", VoucherNum: 3, Summary: "凭证级说明", Attachment: 2, UnitName: "某居委会",
				Entries: []WriteEntry{
					{Summary: "行级说明A", Account: "现金", Side: SideDebit, AmountCents: 12345},
					{Account: "其他收入", Side: SideCredit, AmountCents: 12345},
				},
			},
		},
		{
			name: "明细科目含连续空格被归一",
			req: WriteRequest{
				Date: "2026-04-20", VoucherNum: 8, Summary: "空格归一", Attachment: 0,
				Entries: []WriteEntry{
					{Account: "管理费用-办公  用品", Side: SideDebit, AmountCents: 100},
					{Account: "现金", Side: SideCredit, AmountCents: 100},
				},
			},
		},
		{
			name: "分厘精度",
			req: WriteRequest{
				Date: "2026-12-31", VoucherNum: 99, Summary: "尾差", Attachment: 0,
				Entries: []WriteEntry{
					{Account: "银行存款", Side: SideDebit, AmountCents: 1},
					{Account: "其他收入", Side: SideCredit, AmountCents: 1},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path, err := WriteVoucher(dir, c.req)
			if err != nil {
				t.Fatalf("WriteVoucher: %v", err)
			}
			if filepath.Base(path) != VoucherFileName(c.req.VoucherNum) {
				t.Fatalf("文件名 = %q", filepath.Base(path))
			}
			got, err := ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if len(got) != len(c.req.Entries) {
				t.Fatalf("条数 = %d, want %d", len(got), len(c.req.Entries))
			}
			for i, want := range c.req.Entries {
				g := got[i]
				wg, wd := SplitAccountPath(want.Account)
				wd = normalizeDetail(wd)
				if g.GeneralAccount != wg || g.DetailAccount != wd {
					t.Errorf("第 %d 条科目 = %q-%q, want %q-%q", i+1, g.GeneralAccount, g.DetailAccount, wg, wd)
				}
				if g.Date != c.req.Date {
					t.Errorf("第 %d 条日期 = %q, want %q", i+1, g.Date, c.req.Date)
				}
				if g.VoucherNum != c.req.VoucherNum {
					t.Errorf("第 %d 条凭证号 = %d, want %d", i+1, g.VoucherNum, c.req.VoucherNum)
				}
				wantDebit, wantCredit := int64(0), int64(0)
				if want.Side == SideDebit {
					wantDebit = want.AmountCents
				} else {
					wantCredit = want.AmountCents
				}
				if g.DebitCents != wantDebit || g.CreditCents != wantCredit {
					t.Errorf("第 %d 条 借/贷 = %d/%d, want %d/%d", i+1, g.DebitCents, g.CreditCents, wantDebit, wantCredit)
				}
				wantSummary := strings.TrimSpace(want.Summary)
				if wantSummary == "" {
					wantSummary = strings.TrimSpace(c.req.Summary)
				}
				if g.Summary != wantSummary {
					t.Errorf("第 %d 条摘要 = %q, want %q（分行摘要应覆盖凭证级摘要）", i+1, g.Summary, wantSummary)
				}
			}
		})
	}
}

// TestWriteVoucherDoesNotPadRows 表格不做空行填充（抽样既有凭证填充数 0–3 不等，非约定）。
func TestWriteVoucherDoesNotPadRows(t *testing.T) {
	dir := t.TempDir()
	req := WriteRequest{
		Date: "2026-04-30", VoucherNum: 4, Summary: "两行", Attachment: 0,
		Entries: []WriteEntry{
			{Account: "现金", Side: SideDebit, AmountCents: 100},
			{Account: "其他收入", Side: SideCredit, AmountCents: 100},
		},
	}
	path, err := WriteVoucher(dir, req)
	if err != nil {
		t.Fatalf("WriteVoucher: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// 表头用 <tr><th>，数据区用 <tr><td>；2 条分录 + 1 合计 = 3
	if n := strings.Count(string(b), "<tr><td>"); n != 3 {
		t.Errorf("<tr><td> 行数 = %d, want 3（2 分录 + 合计，无空行填充）", n)
	}
}

func TestWriteVoucherRejectsExistingFile(t *testing.T) {
	dir := t.TempDir()
	req := WriteRequest{
		Date: "2026-04-30", VoucherNum: 5, Summary: "重复", Attachment: 0,
		Entries: []WriteEntry{
			{Account: "现金", Side: SideDebit, AmountCents: 100},
			{Account: "其他收入", Side: SideCredit, AmountCents: 100},
		},
	}
	if _, err := WriteVoucher(dir, req); err != nil {
		t.Fatalf("首次写入应成功: %v", err)
	}
	if _, err := WriteVoucher(dir, req); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("重复写入应被拒绝，得到: %v", err)
	}
}

func TestValidateWriteRequestRejections(t *testing.T) {
	base := func() WriteRequest {
		return WriteRequest{
			Date: "2026-04-30", VoucherNum: 1, Summary: "付办公费", Attachment: 0,
			Entries: []WriteEntry{{Account: "现金", Side: SideDebit, AmountCents: 100}},
		}
	}
	cases := []struct {
		name    string
		mutate  func(*WriteRequest)
		wantSub string
	}{
		{"日期非法", func(r *WriteRequest) { r.Date = "2026/04/30" }, "YYYY-MM-DD"},
		{"凭证号非正", func(r *WriteRequest) { r.VoucherNum = 0 }, "凭证号"},
		{"摘要为空", func(r *WriteRequest) { r.Summary = "  " }, "摘要不能为空"},
		{"附件为负", func(r *WriteRequest) { r.Attachment = -1 }, "附件数"},
		{"无分录", func(r *WriteRequest) { r.Entries = nil }, "至少需要一条分录"},
		{"科目为空", func(r *WriteRequest) { r.Entries[0].Account = " " }, "科目为空"},
		{"方向非法", func(r *WriteRequest) { r.Entries[0].Side = "debitX" }, "方向非法"},
		{"金额为零", func(r *WriteRequest) { r.Entries[0].AmountCents = 0 }, "金额为 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := base()
			c.mutate(&req)
			err := ValidateWriteRequest(req)
			if err == nil || !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("want error containing %q, got %v", c.wantSub, err)
			}
		})
	}
}

// TestValidateWriteRequestRejectsParserHostileText 挡住会让解析器静默丢行的文本。
//
// 背景（往返自检发现的真坑）：parser.go 的 parseVoucherText 会把"任意单元格含
// 摘要/总帐科目/总账科目/明细科目/借方/贷方"的行当作表头丢弃，把摘要为"合计"的行
// 当作合计行丢弃——都不报错。若写入器不拦，该行会从账本中静默消失。
func TestValidateWriteRequestRejectsParserHostileText(t *testing.T) {
	base := func() WriteRequest {
		return WriteRequest{
			Date: "2026-04-30", VoucherNum: 1, Summary: "付办公费", Attachment: 0,
			Entries: []WriteEntry{{Account: "现金", Side: SideDebit, AmountCents: 100}},
		}
	}
	cases := []struct {
		name    string
		mutate  func(*WriteRequest)
		wantSub string
	}{
		{"凭证摘要含表头关键字", func(r *WriteRequest) { r.Summary = "结转借方余额说明" }, "表头关键字"},
		{"分行摘要含表头关键字", func(r *WriteRequest) { r.Entries[0].Summary = "明细科目待定" }, "表头关键字"},
		{"科目含表头关键字", func(r *WriteRequest) { r.Entries[0].Account = "摘要往来" }, "表头关键字"},
		{"凭证摘要为合计", func(r *WriteRequest) { r.Summary = "合计" }, "合计"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := base()
			c.mutate(&req)
			err := ValidateWriteRequest(req)
			if err == nil || !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("want error containing %q, got %v", c.wantSub, err)
			}
		})
	}
}

// TestWriteVoucherSelfCheckDeletesOnFailure 自检失败必须删除已写出的文件（不留半成品）。
func TestWriteVoucherSelfCheckDeletesOnFailure(t *testing.T) {
	orig := verifyFn
	defer func() { verifyFn = orig }()
	verifyFn = func(string, WriteRequest) error { return errForcedSelfCheck }

	dir := t.TempDir()
	req := WriteRequest{
		Date: "2026-04-30", VoucherNum: 6, Summary: "自检失败", Attachment: 0,
		Entries: []WriteEntry{
			{Account: "现金", Side: SideDebit, AmountCents: 100},
			{Account: "其他收入", Side: SideCredit, AmountCents: 100},
		},
	}
	_, err := WriteVoucher(dir, req)
	if err == nil {
		t.Fatalf("自检失败时 WriteVoucher 应返回错误")
	}
	if !strings.Contains(err.Error(), "写入自检失败") {
		t.Errorf("错误信息应说明自检失败，得到: %v", err)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Errorf("自检失败后目录应为空，实际剩 %d 个文件", len(entries))
	}
}

var errForcedSelfCheck = &forcedSelfCheckError{}

type forcedSelfCheckError struct{}

func (*forcedSelfCheckError) Error() string { return "forced self-check failure" }

// TestVerifyRoundTripDetectsTamper 篡改已写文件后，往返自检必须报错。
func TestVerifyRoundTripDetectsTamper(t *testing.T) {
	dir := t.TempDir()
	req := WriteRequest{
		Date: "2026-04-30", VoucherNum: 9, Summary: "篡改检测", Attachment: 0,
		Entries: []WriteEntry{
			{Account: "现金", Side: SideDebit, AmountCents: 100},
			{Account: "其他收入", Side: SideCredit, AmountCents: 100},
		},
	}
	path, err := WriteVoucher(dir, req)
	if err != nil {
		t.Fatalf("WriteVoucher: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(b), "<td>1.00</td>", "<td>9.99</td>", 1)
	if tampered == string(b) {
		t.Fatalf("未找到可篡改的金额单元格")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRoundTrip(path, req); err == nil {
		t.Fatalf("篡改后往返自检应报错")
	}
}

// TestNormalizeDetail 明细列空白归一必须与解析器 cleanDetail 同口径。
func TestNormalizeDetail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"办公费", "办公费"},
		{" 办公费 ", "办公费"},
		{"办公  用品", "办公 用品"},
		{"办公　用品", "办公 用品"},
		{"办公 \t 用品", "办公 用品"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := normalizeDetail(c.in); got != c.want {
			t.Errorf("normalizeDetail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
