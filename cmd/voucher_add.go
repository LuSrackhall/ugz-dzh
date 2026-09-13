package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ledger/balance"
	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	voucherCmd.AddCommand(voucherAddCmd)
	f := voucherAddCmd.Flags()
	f.StringP("output", "o", ".", "账套输出根目录（凭证写入 <输出根>/vouchers/YYYY_MM/，月份由 --date 决定）")
	f.String("date", "", "凭证日期 YYYY-MM-DD（必填；决定写入哪个月目录）")
	f.String("summary", "", "凭证摘要（必填，可用 --json 提供）")
	f.StringArray("debit", nil, "借方分录「科目=金额」，可重复；与 --json 互斥")
	f.StringArray("credit", nil, "贷方分录「科目=金额」，可重复；与 --json 互斥")
	f.String("json", "", "从 JSON 读取整张凭证（\"-\" = stdin）；与 --debit/--credit 互斥")
	f.Int("attachment", 0, "附件张数")
	f.Int("num", 0, "显式指定凭证号（默认自动取当月最大号 +1）")
	f.String("idempotency-key", "", "幂等键：同键重复调用返回首次结果，不产生第二张凭证")
	f.String("unit", "", "记账单位名（凭证抬头）")
	f.BoolP("verbose", "V", false, "输出详细日志")
}

var voucherAddCmd = &cobra.Command{
	Use:   "add",
	Short: "创建一张凭证 md（不平不落盘）",
	Long: `创建一张凭证 Markdown 文件。

落盘前强制三道闸门（任一不过即零写盘）：
  1. 借贷平衡（净额口径，红字为负值参与）——复用 generate 的同一套校验
  2. 科目必须在科目树已定义——复用"先定义后生成"的同一套检查（无 --allow-new 逃生）
  3. 可解析性——写入后立即用解析器往返自检，不一致即删除文件

本命令不触发账本生成。`,
	Example: `  ledger voucher add --date 2026-03-05 --summary "付养老金" \
    --debit "公益支出-补助费用=9990.00" --credit "应付款-养老金=9990.00"

  echo '{"date":"2026-03-05","summary":"付养老金","entries":[
    {"account":"公益支出-补助费用","side":"debit","amount":"9990.00"},
    {"account":"应付款-养老金","side":"credit","amount":"9990.00"}]}' | ledger voucher add --json -`,
	RunE: runVoucherAdd,
}

// jsonAmount 金额字面量：既接受 JSON 字符串（"9990.00"）也接受 JSON 数字（9990.00）。
// 数字按原始字面量保留，避免 float64 精度损失。
type jsonAmount string

func (a *jsonAmount) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*a = ""
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*a = jsonAmount(v)
		return nil
	}
	*a = jsonAmount(s)
	return nil
}

type voucherAddJSONEntry struct {
	Summary string     `json:"summary"`
	Account string     `json:"account"`
	Side    string     `json:"side"`
	Amount  jsonAmount `json:"amount"`
}

type voucherAddJSON struct {
	Date       string                `json:"date"`
	Summary    string                `json:"summary"`
	Attachment *int                  `json:"attachment"`
	Unit       string                `json:"unit"`
	Num        int                   `json:"num"`
	Signers    *voucher.Signers      `json:"signers"`
	Entries    []voucherAddJSONEntry `json:"entries"`
}

func runVoucherAdd(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	dateFlag, _ := cmd.Flags().GetString("date")
	summaryFlag, _ := cmd.Flags().GetString("summary")
	debits, _ := cmd.Flags().GetStringArray("debit")
	credits, _ := cmd.Flags().GetStringArray("credit")
	jsonArg, _ := cmd.Flags().GetString("json")
	attachment, _ := cmd.Flags().GetInt("attachment")
	numFlag, _ := cmd.Flags().GetInt("num")
	idemKey, _ := cmd.Flags().GetString("idempotency-key")
	unit, _ := cmd.Flags().GetString("unit")
	verbose, _ := cmd.Flags().GetBool("verbose")

	if jsonArg != "" && (len(debits) > 0 || len(credits) > 0) {
		return fmt.Errorf("--json 与 --debit/--credit 互斥，请二选一")
	}

	// 组装入参：flag 形态 或 JSON 形态
	req := voucher.WriteRequest{Attachment: attachment, UnitName: unit, VoucherNum: numFlag}
	if jsonArg != "" {
		raw, err := readJSONArg(jsonArg)
		if err != nil {
			return err
		}
		var payload voucherAddJSON
		dec := json.NewDecoder(strings.NewReader(raw))
		if err := dec.Decode(&payload); err != nil {
			return fmt.Errorf("解析 --json 入参: %w", err)
		}
		req.Date = strings.TrimSpace(payload.Date)
		req.Summary = payload.Summary
		req.UnitName = payload.Unit
		if payload.Attachment != nil {
			req.Attachment = *payload.Attachment
		}
		if payload.Num > 0 {
			req.VoucherNum = payload.Num
		}
		if payload.Signers != nil {
			req.Signers = *payload.Signers
		}
		for i, e := range payload.Entries {
			side, err := normalizeSide(e.Side)
			if err != nil {
				return fmt.Errorf("第 %d 条分录: %w", i+1, err)
			}
			cents, ok := voucher.ParseAmountCents(string(e.Amount))
			if !ok {
				return fmt.Errorf("第 %d 条分录金额无法解析：%q", i+1, string(e.Amount))
			}
			req.Entries = append(req.Entries, voucher.WriteEntry{
				Summary: e.Summary, Account: e.Account, Side: side, AmountCents: cents,
			})
		}
	} else {
		req.Date = strings.TrimSpace(dateFlag)
		req.Summary = summaryFlag
		d, err := parseFlagEntries(debits, voucher.SideDebit)
		if err != nil {
			return err
		}
		c, err := parseFlagEntries(credits, voucher.SideCredit)
		if err != nil {
			return err
		}
		req.Entries = append(d, c...)
	}

	// 日期决定年份与月目录
	if req.Date == "" {
		return fmt.Errorf("缺少凭证日期（--date 或 --json 的 date 字段）")
	}
	if _, err := voucher.ParseISODate(req.Date); err != nil {
		return err
	}
	if strings.TrimSpace(req.Summary) == "" {
		return fmt.Errorf("缺少凭证摘要（--summary 或 --json 的 summary 字段）")
	}
	year := req.Date[:4]
	monthDir := filepath.Join(output, "vouchers", voucher.MonthDirName(req.Date))
	monthKey := voucher.MonthDirName(req.Date)

	// 账套配置（科目树）
	configJSON := filepath.Join(output, year, year+".json")
	cfg, err := balance.LoadConfig(configJSON)
	if err != nil {
		return fmt.Errorf("加载账套配置 %s: %w（voucher add 需要账套已建：先 ledger init）", configJSON, err)
	}

	// 幂等：同键命中且文件仍在 → 直接返回首次结果
	keysPath := filepath.Join(output, "vouchers", ".voucher-keys", monthKey+".json")
	store := loadIdemStore(keysPath)
	if idemKey != "" {
		if prev, ok := store.Keys[idemKey]; ok {
			if _, statErr := os.Stat(filepath.Join(output, prev.Path)); statErr == nil {
				fmt.Printf("幂等命中（未重复创建）：%s\n", prev.Path)
				fmt.Printf("  凭证号 记字第%04d号\n", prev.Num)
				return nil
			}
			// 文件已被人工删除 → 索引失效，继续重新创建
			delete(store.Keys, idemKey)
		}
	}

	// 序列/目录卫生前置检查：默认只告警（对仗 voucher check 的默认口径）
	if _, err := os.Stat(monthDir); err == nil {
		if report, err := voucher.CheckSequence(monthDir); err == nil && !report.OK() {
			fmt.Printf("⚠ 月目录存在 %d 项问题（不阻断，请核实）：\n%s", len(report.Issues), report.Summary())
		}
	}

	// 结账月提示（不阻断）：已结账月份的账本需 -f 级联重建才能反映新凭证
	if cm := cfg.Settings.ClosingMonth; cm != "" && req.Date[:7] <= cm {
		fmt.Printf("⚠ 凭证所属月份 %s 已结账（结账月 %s）——账本需 ledger generate -f 级联重建才会反映本凭证\n", req.Date[:7], cm)
	}

	// 凭证号分配
	if req.VoucherNum <= 0 {
		num, err := nextVoucherNum(monthDir)
		if err != nil {
			return err
		}
		req.VoucherNum = num
	} else if _, err := os.Stat(filepath.Join(monthDir, voucher.VoucherFileName(req.VoucherNum))); err == nil {
		return fmt.Errorf("凭证号冲突：%s 已存在", voucher.VoucherFileName(req.VoucherNum))
	}

	// 闸门一：借贷平衡（净额口径）
	entryForCheck := make([]voucher.Entry, 0, len(req.Entries))
	for _, e := range req.Entries {
		g, d := voucher.SplitAccountPath(e.Account)
		v := voucher.Entry{
			GeneralAccount: g, DetailAccount: d,
			VoucherNum: req.VoucherNum, Date: req.Date, Summary: e.Summary,
		}
		if e.Side == voucher.SideDebit {
			v.DebitCents = e.AmountCents
		} else {
			v.CreditCents = e.AmountCents
		}
		entryForCheck = append(entryForCheck, v)
	}
	warnings, err := voucher.ValidateVoucherBalance(entryForCheck)
	if err != nil {
		return fmt.Errorf("凭证借贷平衡校验失败（未写盘）: %w", err)
	}

	// 闸门二：科目必须在科目树已定义（复用"先定义后生成"同一套检查）
	if undefined := undefinedVoucherSubjects(cfg, entryForCheck); len(undefined) > 0 {
		fmt.Printf("⚠ 凭证中存在 %d 个未定义科目（科目树中没有），拒绝写盘：\n", len(undefined))
		for _, u := range undefined {
			fmt.Printf("  - %s（出现 %d 次，样例摘要：%s）\n", u.Account, u.Count, u.Sample)
		}
		fmt.Println("处理方式：ledger subjects scan → 确认方向 → ledger subjects import -f 审核表.csv；OCR/同音错名先 ledger map 纠错")
		return fmt.Errorf("存在 %d 个未定义科目，拒绝写盘（先定义后生成）", len(undefined))
	}

	// 闸门三：写入 + 往返自检
	path, err := voucher.WriteVoucher(monthDir, req)
	if err != nil {
		return err
	}

	for _, w := range warnings {
		fmt.Printf("提示: %s\n", w)
	}

	// 记录幂等键
	if idemKey != "" {
		rel, relErr := filepath.Rel(output, path)
		if relErr != nil {
			rel = path
		}
		store.Keys[idemKey] = idemEntry{
			Num: req.VoucherNum, Path: rel, CreatedAt: time.Now().Format(time.RFC3339),
		}
		if err := saveIdemStore(keysPath, store); err != nil {
			return fmt.Errorf("凭证已创建（%s），但幂等索引写入失败: %w", path, err)
		}
	}

	var totalDebit, totalCredit int64
	for _, e := range req.Entries {
		if e.Side == voucher.SideDebit {
			totalDebit += e.AmountCents
		} else {
			totalCredit += e.AmountCents
		}
	}
	rel, relErr := filepath.Rel(output, path)
	if relErr != nil {
		rel = path
	}
	fmt.Printf("凭证已创建：%s\n", rel)
	fmt.Printf("  凭证号 记字第%04d号\n", req.VoucherNum)
	fmt.Printf("  日期   %s\n", req.Date)
	fmt.Printf("  摘要   %s\n", req.Summary)
	fmt.Printf("  借方合计 %s  贷方合计 %s\n", voucher.FormatAmountCents(totalDebit), voucher.FormatAmountCents(totalCredit))
	if verbose {
		fmt.Printf("  月目录 %s\n  分录 %d 条\n", monthDir, len(req.Entries))
	}
	fmt.Println("账本未生成——如需入账请显式执行: ledger generate -v " + monthDir + " -o " + output)
	return nil
}

func readJSONArg(arg string) (string, error) {
	if arg == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("读取 stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(arg)
	if err != nil {
		return "", fmt.Errorf("读取 %s: %w", arg, err)
	}
	return string(b), nil
}

// normalizeSide 借贷方向归一：接受 debit/credit 与 借/贷/借方/贷方。
func normalizeSide(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debit", "借", "借方", "d":
		return voucher.SideDebit, nil
	case "credit", "贷", "贷方", "c":
		return voucher.SideCredit, nil
	}
	return "", fmt.Errorf("借贷方向非法：%q（可用 debit/credit 或 借/贷）", s)
}

func parseFlagEntries(pairs []string, side string) ([]voucher.WriteEntry, error) {
	var out []voucher.WriteEntry
	for _, p := range pairs {
		i := strings.Index(p, "=")
		if i <= 0 || i == len(p)-1 {
			return nil, fmt.Errorf("分录写法应为「科目=金额」，收到 %q", p)
		}
		account := strings.TrimSpace(p[:i])
		amountStr := strings.TrimSpace(p[i+1:])
		cents, ok := voucher.ParseAmountCents(amountStr)
		if !ok {
			return nil, fmt.Errorf("金额无法解析：%q（分录 %q）", amountStr, p)
		}
		out = append(out, voucher.WriteEntry{Account: account, Side: side, AmountCents: cents})
	}
	return out, nil
}

// nextVoucherNum 扫描月目录取最大凭证号 +1（正式命名由白名单检查保证）。
func nextVoucherNum(monthDir string) (int, error) {
	entries, err := os.ReadDir(monthDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("读取月目录 %s: %w", monthDir, err)
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n, ok := voucher.ParseVoucherFileNum(e.Name())
		if !ok {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// --- 幂等索引 ---

type idemEntry struct {
	Num       int    `json:"num"`
	Path      string `json:"path"`
	CreatedAt string `json:"createdAt"`
}

type idemStore struct {
	Keys map[string]idemEntry `json:"keys"`
}

func loadIdemStore(path string) *idemStore {
	store := &idemStore{Keys: map[string]idemEntry{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return store
	}
	var loaded idemStore
	if err := json.Unmarshal(b, &loaded); err != nil || loaded.Keys == nil {
		return store
	}
	return &loaded
}

func saveIdemStore(path string, store *idemStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
