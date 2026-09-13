package voucher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// regexpWhitespace 与解析器 cleanDetail 同口径的空白折叠。
var regexpWhitespace = regexp.MustCompile(`\s+`)

// 借贷方向常量（WriteEntry.Side）。
const (
	SideDebit  = "debit"
	SideCredit = "credit"
)

// WriteEntry 一张凭证中的一行分录。
type WriteEntry struct {
	Summary     string // 行摘要；空 → 回落凭证级摘要
	Account     string // 科目全路径（"总账" 或 "总账-明细"）
	Side        string // SideDebit / SideCredit
	AmountCents int64  // 分；负数 = 红字（同侧，与解析器 Entry 语义一致）
}

// Signers 凭证签章行（留空 = 打印后手签）。
type Signers struct {
	AccountSupervisor string // 会计主管
	Bookkeeper        string // 记帐
	Auditor           string // 审核
	Preparer          string // 制单
}

// WriteRequest 一张待创建的凭证（写盘入参）。
type WriteRequest struct {
	Date       string // YYYY-MM-DD
	VoucherNum int
	Summary    string
	Attachment int
	UnitName   string
	Signers    Signers
	Entries    []WriteEntry
}

// SplitAccountPath 按首个 '-' 切分科目全路径（与 balance.splitPath / cmd.splitEntryPath 同口径）。
func SplitAccountPath(path string) (general, detail string) {
	path = strings.TrimSpace(path)
	if i := strings.IndexByte(path, '-'); i > 0 && i < len(path)-1 {
		return path[:i], path[i+1:]
	}
	return path, ""
}

// VoucherFileName 正式凭证文件名（月目录白名单要求形如 记字第0001号.md）。
func VoucherFileName(num int) string {
	return fmt.Sprintf("记字第%04d号.md", num)
}

// MonthDirName 由日期推导凭证月目录名（2026-03-05 → 2026_03）。
func MonthDirName(date string) string {
	date = strings.TrimSpace(date)
	if len(date) < 7 {
		return ""
	}
	return strings.Replace(date[:7], "-", "_", 1)
}

// ParseISODate 校验并解析 YYYY-MM-DD。
func ParseISODate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("日期须为 YYYY-MM-DD 格式，收到 %q", s)
	}
	return t, nil
}

// parserHeaderKeywords 解析器把"任意单元格含这些子串"的行当作表头丢弃
// （parser.go parseVoucherText 的 containsAnyInSlice 判定）。写入器必须提前挡住，
// 否则该行会静默消失、整张凭证缺行——往返自检发现此坑后固化为校验。
var parserHeaderKeywords = []string{"摘要", "总帐科目", "总账科目", "明细科目", "借方", "贷方"}

// validateParserSafeText 拒绝会让解析器丢弃整行的文本。
func validateParserSafeText(where, s string) error {
	for _, kw := range parserHeaderKeywords {
		if strings.Contains(s, kw) {
			return fmt.Errorf("%s 含表头关键字 %q：解析器会把整行当作表头丢弃（该行会从账本中静默消失），请改写措辞", where, kw)
		}
	}
	if strings.EqualFold(strings.TrimSpace(s), "合计") {
		return fmt.Errorf("%s 为「合计」：解析器会把该行当作合计行跳过，请改写措辞", where)
	}
	return nil
}

// ValidateWriteRequest 校验写盘入参（不含借贷平衡与科目定义——那两项由 cmd 层复用既有闸门）。
func ValidateWriteRequest(req WriteRequest) error {
	if _, err := ParseISODate(req.Date); err != nil {
		return err
	}
	if req.VoucherNum <= 0 {
		return fmt.Errorf("凭证号须为正整数，收到 %d", req.VoucherNum)
	}
	if strings.TrimSpace(req.Summary) == "" {
		return fmt.Errorf("凭证摘要不能为空")
	}
	if err := validateParserSafeText("凭证摘要", req.Summary); err != nil {
		return err
	}
	if req.Attachment < 0 {
		return fmt.Errorf("附件数不能为负")
	}
	if len(req.Entries) == 0 {
		return fmt.Errorf("凭证至少需要一条分录")
	}
	for i, e := range req.Entries {
		if strings.TrimSpace(e.Account) == "" {
			return fmt.Errorf("第 %d 条分录科目为空", i+1)
		}
		switch e.Side {
		case SideDebit, SideCredit:
		default:
			return fmt.Errorf("第 %d 条分录借贷方向非法：%q（须为 %q 或 %q）", i+1, e.Side, SideDebit, SideCredit)
		}
		if e.AmountCents == 0 {
			return fmt.Errorf("第 %d 条分录金额为 0（解析器会跳过金额为零的行，无法往返）", i+1)
		}
		if strings.TrimSpace(e.Summary) != "" {
			if err := validateParserSafeText(fmt.Sprintf("第 %d 条分行摘要", i+1), e.Summary); err != nil {
				return err
			}
		}
		general, detail := SplitAccountPath(e.Account)
		if err := validateParserSafeText(fmt.Sprintf("第 %d 条科目", i+1), general+detail); err != nil {
			return err
		}
	}
	return nil
}

// normalizeDetail 复刻解析器 cleanDetail 的归一（全角空格→空格、连续空白折叠为单个空格）。
// 不归一的话，入参里的连续空格会与解析结果不一致、被往返自检判为失败。
func normalizeDetail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "　", " ")
	return regexpWhitespace.ReplaceAllString(s, " ")
}

// ParseAmountCents 解析金额写法为分，与凭证解析器同口径
// （千分位、括号红字、全角括号/减号等四种红字写法均等价）。
func ParseAmountCents(s string) (int64, bool) { return parseAmountToCents(s) }

// FormatAmountCents 将分格式化为凭证金额写法：千分位 + 两位小数；负数（红字）用半角括号。
func FormatAmountCents(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	s := addThousands(fmt.Sprintf("%d.%02d", c/100, c%100))
	if neg {
		return "(" + s + ")"
	}
	return s
}

func addThousands(s string) string {
	intPart, fracPart := s, ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart, fracPart = s[:dot], s[dot:]
	}
	var b strings.Builder
	n := len(intPart)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	return b.String() + fracPart
}

// RenderVoucher 渲染凭证 Markdown 正文。
//
// 格式与 test/e2e/test_data 下的手写凭证一致（解析器为唯一裁定方，见 WriteVoucher 的往返自检）：
//
//	<单位名>
//	记字第XXXX号 1/1
//	记帐凭证
//	YYYY年MM月DD日
//	附件 N 张
//	<table>…</table>
//	会计主管 … / 记帐 … / 审核 … / 制单 …
//
// 表格只写真实分录行，不做空行填充——抽样既有凭证发现填充行数为 0–3 不等，
// 属 OCR 原件的产物而非约定，故不作为生成规则（解析器本就跳过空行）。
func RenderVoucher(req WriteRequest) (string, error) {
	if err := ValidateWriteRequest(req); err != nil {
		return "", err
	}
	t, _ := ParseISODate(req.Date)

	var b strings.Builder
	if unit := strings.TrimSpace(req.UnitName); unit != "" {
		b.WriteString(unit + "\n\n")
	}
	fmt.Fprintf(&b, "%s 1/1\n\n", VoucherFileNameNoExt(req.VoucherNum))
	b.WriteString("记帐凭证\n\n")
	fmt.Fprintf(&b, "%04d年%02d月%02d日\n\n", t.Year(), int(t.Month()), t.Day())
	fmt.Fprintf(&b, "附件 %d 张\n\n", req.Attachment)

	b.WriteString("<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody>")
	var totalDebit, totalCredit int64
	for _, e := range req.Entries {
		general, detail := SplitAccountPath(e.Account)
		detail = normalizeDetail(detail)
		summary := strings.TrimSpace(e.Summary)
		if summary == "" {
			summary = strings.TrimSpace(req.Summary)
		}
		debit, credit := "", ""
		if e.Side == SideDebit {
			debit = FormatAmountCents(e.AmountCents)
			totalDebit += e.AmountCents
		} else {
			credit = FormatAmountCents(e.AmountCents)
			totalCredit += e.AmountCents
		}
		// 摘要与科目均做 HTML 转义，避免 & < > 破坏表格结构
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			escapeCell(summary), escapeCell(general), escapeCell(detail), debit, credit)
	}
	fmt.Fprintf(&b, "<tr><td>合计</td><td></td><td></td><td>%s</td><td>%s</td></tr>",
		FormatAmountCents(totalDebit), FormatAmountCents(totalCredit))
	b.WriteString("</tbody></table>\n\n")

	for _, s := range []struct{ label, name string }{
		{"会计主管", req.Signers.AccountSupervisor},
		{"记帐", req.Signers.Bookkeeper},
		{"审核", req.Signers.Auditor},
		{"制单", req.Signers.Preparer},
	} {
		if n := strings.TrimSpace(s.name); n != "" {
			fmt.Fprintf(&b, "%s %s\n", s.label, n)
		} else {
			fmt.Fprintf(&b, "%s \n", s.label)
		}
	}
	return b.String(), nil
}

// VoucherFileNameNoExt 凭证号写法（记字第0001号），用于正文抬头行。
func VoucherFileNameNoExt(num int) string {
	return strings.TrimSuffix(VoucherFileName(num), ".md")
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// WriteVoucher 渲染并写盘，随后立即用解析器做往返自检（round-trip）：
// 解析结果必须与入参逐项一致，否则删除该文件并以错误返回。
//
// 之所以把正确性交给解析器裁定：写入器与解析器是同一份格式契约的两端，
// 人工比对格式无法在 CI 里挡住漂移。
func WriteVoucher(dir string, req WriteRequest) (string, error) {
	content, err := RenderVoucher(req)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, VoucherFileName(req.VoucherNum))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建凭证目录 %s: %w", dir, err)
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("凭证文件已存在：%s", path)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入凭证 %s: %w", path, err)
	}
	if err := verifyFn(path, req); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("写入自检失败（已删除 %s）：%w", path, err)
	}
	return path, nil
}

// verifyFn 是往返自检的可替换入口（测试用于验证"自检失败必须删除文件"这条安全路径）。
var verifyFn = verifyRoundTrip

// verifyRoundTrip 解析刚写出的文件并与入参逐项比对。
func verifyRoundTrip(path string, req WriteRequest) error {
	got, err := ParseFile(path)
	if err != nil {
		return err
	}
	if len(got) != len(req.Entries) {
		return fmt.Errorf("分录条数不一致：解析得 %d 条，入参 %d 条", len(got), len(req.Entries))
	}
	for i, want := range req.Entries {
		g := got[i]
		if g.VoucherNum != req.VoucherNum {
			return fmt.Errorf("第 %d 条凭证号不一致：解析得 %d，入参 %d", i+1, g.VoucherNum, req.VoucherNum)
		}
		if g.Date != req.Date {
			return fmt.Errorf("第 %d 条日期不一致：解析得 %q，入参 %q", i+1, g.Date, req.Date)
		}
		wantGeneral, wantDetail := SplitAccountPath(want.Account)
		// 明细列在渲染时做了与解析器同口径的空白归一，故期望值同样归一后再比对
		// （否则合法输入会被自检误判为失败）。归一函数本身由 TestNormalizeDetail 覆盖。
		wantDetail = normalizeDetail(wantDetail)
		if g.GeneralAccount != wantGeneral || g.DetailAccount != wantDetail {
			return fmt.Errorf("第 %d 条科目不一致：解析得 %q-%q，入参 %q-%q",
				i+1, g.GeneralAccount, g.DetailAccount, wantGeneral, wantDetail)
		}
		wantDebit, wantCredit := int64(0), int64(0)
		if want.Side == SideDebit {
			wantDebit = want.AmountCents
		} else {
			wantCredit = want.AmountCents
		}
		if g.DebitCents != wantDebit || g.CreditCents != wantCredit {
			return fmt.Errorf("第 %d 条金额/方向不一致：解析得 借 %d 贷 %d，入参 借 %d 贷 %d",
				i+1, g.DebitCents, g.CreditCents, wantDebit, wantCredit)
		}
		wantSummary := strings.TrimSpace(want.Summary)
		if wantSummary == "" {
			wantSummary = strings.TrimSpace(req.Summary)
		}
		if g.Summary != wantSummary {
			return fmt.Errorf("第 %d 条摘要不一致：解析得 %q，入参 %q", i+1, g.Summary, wantSummary)
		}
	}
	return nil
}
