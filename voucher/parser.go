package voucher

import (
	"fmt"
	"html"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Entry represents a single voucher entry line parsed from a Markdown HTML table.
type Entry struct {
	GeneralAccount string `json:"generalAccount"` // 总账科目
	DetailAccount  string `json:"detailAccount"`  // 明细科目 (可空)
	VoucherNum     int    `json:"voucherNum"`     // 凭证号码 (数值型)
	Date           string `json:"date"`           // YYYY-MM-DD
	Summary        string `json:"summary"`        // 摘要
	DebitCents     int64  `json:"debitCents"`     // 借方金额（分）
	CreditCents    int64  `json:"creditCents"`    // 贷方金额（分）
}

// ParseFile parses a single .md file and returns all voucher entries.
func ParseFile(filePath string) ([]Entry, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read voucher file %s: %w", filePath, err)
	}
	content := string(b)
	entries := parseVoucherText(content, filepath.Base(filePath))
	for i := range entries {
		entries[i].DetailAccount = cleanDetail(entries[i].DetailAccount)
	}
	return entries, nil
}

func parseVoucherText(text string, filename string) []Entry {
	entries := []Entry{}

	vnum := extractVoucherNum(text)
	if vnum <= 0 {
		if fn := strings.TrimSpace(filename); fn != "" {
			if v := extractFromFilename(fn); v > 0 {
				vnum = v
				log.Printf("警告: 正文未解析到凭证号，使用文件名回退解析 %s -> %d", filename, vnum)
			}
		}
	}
	if vnum <= 0 {
		log.Printf("警告: 未能解析到凭证号（文件=%s）。请检查该文件的凭证号书写格式。", filename)
	}

	date := ""
	rxDate := regexp.MustCompile(`(\d{4})年\s*0*([0-9]{1,2})月\s*0*([0-9]{1,2})日`)
	if m := rxDate.FindStringSubmatch(text); len(m) >= 4 {
		year := m[1]
		month := padLeft(m[2], 2, '0')
		day := padLeft(m[3], 2, '0')
		date = fmt.Sprintf("%s-%s-%s", year, month, day)
	} else {
		rxISO := regexp.MustCompile(`(\d{4})[-/](\d{1,2})[-/](\d{1,2})`)
		if m := rxISO.FindStringSubmatch(text); len(m) >= 4 {
			date = fmt.Sprintf("%s-%02s-%02s", m[1], padLeft(m[2], 2, '0'), padLeft(m[3], 2, '0'))
		}
	}

	tableRx := regexp.MustCompile(`(?is)<table.*?>(.*?)</table>`)
	tableMatches := tableRx.FindAllStringSubmatch(text, -1)
	if len(tableMatches) == 0 {
		return entries
	}

	trRx := regexp.MustCompile(`(?is)<tr.*?>(.*?)</tr>`)
	cellRx := regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)
	tagStripRx := regexp.MustCompile(`(?is)<[^>]+>`)
	for _, tm := range tableMatches {
		tableInner := tm[1]
		rows := trRx.FindAllStringSubmatch(tableInner, -1)
		for _, r := range rows {
			rowInner := r[1]
			cellsRaw := cellRx.FindAllStringSubmatch(rowInner, -1)
			if len(cellsRaw) == 0 {
				continue
			}
			cols := make([]string, 0, len(cellsRaw))
			for _, cr := range cellsRaw {
				cell := cr[1]
				cell = strings.ReplaceAll(cell, "<br/>", " ")
				cell = strings.ReplaceAll(cell, "<br />", " ")
				cell = strings.ReplaceAll(cell, "<br>", " ")
				cell = tagStripRx.ReplaceAllString(cell, "")
				cell = html.UnescapeString(cell)
				cell = strings.TrimSpace(cell)
				cols = append(cols, cell)
			}
			if len(cols) > 0 && (containsAny(cols[0], "摘要", "摘要内容") || containsAnyInSlice(cols, []string{"摘要", "总帐科目", "总账科目", "明细科目", "借方", "贷方"})) {
				continue
			}
			get := func(i int) string {
				if i < 0 || i >= len(cols) {
					return ""
				}
				return cols[i]
			}
			summary := get(0)
			genAcct := get(1)
			detail := get(2)
			debitCell := get(3)
			creditCell := get(4)

			if strings.EqualFold(summary, "合计") || strings.TrimSpace(genAcct) == "" {
				continue
			}

			debitCents, _ := parseAmountToCents(debitCell)
			creditCents, _ := parseAmountToCents(creditCell)
			if debitCents == 0 && creditCents == 0 {
				continue
			}

			entry := Entry{
				GeneralAccount: genAcct,
				DetailAccount:  detail,
				VoucherNum:     vnum,
				Date:           date,
				Summary:        summary,
				DebitCents:     debitCents,
				CreditCents:    creditCents,
			}
			entries = append(entries, entry)
		}
	}

	return entries
}

func extractVoucherNum(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	patterns := []string{
		`(?is)记(?:字|帐|账)?第\D{0,10}?0*([0-9]{1,6})\s*号`,
		`(?is)记(?:字|帐|账)?.{0,20}?第\D{0,8}?0*([0-9]{1,6})\s*号`,
		`(?is)记(?:字|帐|账)?第\D{0,10}?0*([0-9]{1,6})\b`,
		`(?is)记字第\D*0*([0-9]{1,6})`,
	}

	for _, p := range patterns {
		rx := regexp.MustCompile(p)
		if m := rx.FindStringSubmatch(text); len(m) >= 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				return n
			}
		}
	}

	rxContext := regexp.MustCompile(`(?is)(记[^\n]{0,30}?|凭证[^\n]{0,30}?|记账[^\n]{0,30}?)([0-9]{1,6})`)
	if m := rxContext.FindStringSubmatch(text); len(m) >= 3 {
		if n, err := strconv.Atoi(m[2]); err == nil && n > 0 && n < 1000000 {
			return n
		}
	}

	rxLoose := regexp.MustCompile(`(?is)记[^0-9]{0,8}([0-9]{1,6})[^0-9]{0,8}号`)
	if m := rxLoose.FindStringSubmatch(text); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n
		}
	}

	return 0
}

func extractFromFilename(filename string) int {
	name := filename
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	numRx := regexp.MustCompile(`\d+`)
	all := numRx.FindAllString(name, -1)
	candidates := []int{}
	for _, s := range all {
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if n >= 1900 && n <= 2100 {
			continue
		}
		if n <= 0 || n > 100000 {
			continue
		}
		candidates = append(candidates, n)
	}
	if len(candidates) == 0 {
		return 0
	}
	min := candidates[0]
	for _, v := range candidates {
		if v < min {
			min = v
		}
	}
	return min
}

func parseAmountToCents(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	keepRx := regexp.MustCompile(`[^0-9.\-]`)
	clean := keepRx.ReplaceAllString(s, "")
	if clean == "" {
		return 0, false
	}
	if strings.Count(clean, ".") > 1 {
		parts := strings.SplitN(clean, ".", 2)
		clean = parts[0] + "." + strings.ReplaceAll(parts[1], ".", "")
	}
	val, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	cents := int64(math.Round(val * 100))
	return cents, true
}

func cleanDetail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "　", " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func containsAnyInSlice(cols []string, subs []string) bool {
	for _, c := range cols {
		for _, sub := range subs {
			if strings.Contains(c, sub) {
				return true
			}
		}
	}
	return false
}

func padLeft(s string, width int, pad rune) string {
	for len(s) < width {
		s = string(pad) + s
	}
	return s
}
