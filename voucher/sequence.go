package voucher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// voucherFileWhitelist 正式凭证文件名白名单。
//
// 刻意比解析器的 voucherNumRe（`记字第\D*0*([0-9]{1,6})`）更严：后者允许
// `记字第0005号 更正.md` 这类带后缀的历史文件名，那是为兼容既有账套留的宽松口径；
// 白名单检查要的是"月目录卫生"，故要求精确命名。
var voucherFileWhitelist = regexp.MustCompile(`^记字第(\d+)号\.md$`)

// ParseVoucherFileNum 从正式凭证文件名解析凭证号（不符合白名单命名则 ok=false）。
func ParseVoucherFileNum(name string) (int, bool) {
	m := voucherFileWhitelist.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// SequenceIssueKind 问题类别。
type SequenceIssueKind string

const (
	IssueMissingNum  SequenceIssueKind = "missing"     // 缺号（1..max 中缺失）
	IssueDuplicate   SequenceIssueKind = "duplicate"   // 重号（同号多文件）
	IssueNonStandard SequenceIssueKind = "nonstandard" // 非正式文件名或不在月目录根层
)

// SequenceIssue 一条问题记录（可 JSON 序列化，供 MCP 工具直接返回）。
type SequenceIssue struct {
	Kind       SequenceIssueKind `json:"kind"`
	VoucherNum int               `json:"voucherNum,omitempty"`
	Files      []string          `json:"files,omitempty"`
	Message    string            `json:"message"`
}

// SequenceReport 检查结果。
type SequenceReport struct {
	Dir     string          `json:"dir"`
	Month   string          `json:"month,omitempty"`
	Count   int             `json:"count"`   // 正式凭证文件数
	MaxNum  int             `json:"maxNum"`  // 最大凭证号
	Numbers []int           `json:"numbers"` // 已有序号（升序）
	Issues  []SequenceIssue `json:"issues"`
}

// OK 是否无任何问题。
func (r *SequenceReport) OK() bool { return len(r.Issues) == 0 }

// Summary 人类可读摘要（供 CLI 直接打印与 agent 复述）。
func (r *SequenceReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "凭证目录 %s：正式凭证 %d 张，凭证号 1..%d\n", r.Dir, r.Count, r.MaxNum)
	if r.OK() {
		b.WriteString("序列检查通过：无缺号、无重号、无非正式文件名\n")
		return b.String()
	}
	fmt.Fprintf(&b, "发现 %d 项问题：\n", len(r.Issues))
	for _, is := range r.Issues {
		fmt.Fprintf(&b, "  - %s\n", is.Message)
	}
	return b.String()
}

// maxMissingListed 缺号明细的输出上限（防止因异常大号产生巨量输出）。
const maxMissingListed = 100

// CheckSequence 检查月目录的凭证序列完整性与目录卫生。
//
// 遍历方式与凭证收集路径一致（filepath.WalkDir 递归，见 cmd/common.go CollectEntries）：
// 月目录内任何 .md 都会被解析记账，因此子目录里的 .md 也计入"非正式"问题——
// 若改用非递归读取，检查本身就会带有它要堵的那个盲区。
//
// 纯读：不修改任何文件。
func CheckSequence(dir string) (*SequenceReport, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("凭证目录不可访问：%w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s 不是目录", dir)
	}

	report := &SequenceReport{Dir: dir, Month: filepath.Base(dir)}
	byNum := map[int][]string{}
	var nonStandard []string

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			rel = path
		}
		base := filepath.Base(path)
		m := voucherFileWhitelist.FindStringSubmatch(base)
		// 必须同时满足：命名规范 且 直接位于月目录根层
		if m == nil || filepath.Dir(rel) != "." {
			nonStandard = append(nonStandard, filepath.ToSlash(rel))
			return nil
		}
		num, convErr := strconv.Atoi(m[1])
		if convErr != nil || num <= 0 {
			nonStandard = append(nonStandard, filepath.ToSlash(rel))
			return nil
		}
		byNum[num] = append(byNum[num], filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(nonStandard)
	for _, f := range nonStandard {
		report.Issues = append(report.Issues, SequenceIssue{
			Kind:  IssueNonStandard,
			Files: []string{f},
			Message: fmt.Sprintf("非正式凭证文件 %s：月目录内任何 .md 都会被递归解析计入账本（重号检测只认「记字第」命名的文件），"+
				"请移出月目录或改名为 记字第XXXX号.md", f),
		})
	}

	nums := make([]int, 0, len(byNum))
	for n := range byNum {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	report.Numbers = nums
	report.Count = len(nums)
	if len(nums) > 0 {
		report.MaxNum = nums[len(nums)-1]
	}

	for _, n := range nums {
		files := byNum[n]
		if len(files) > 1 {
			sort.Strings(files)
			report.Issues = append(report.Issues, SequenceIssue{
				Kind:       IssueDuplicate,
				VoucherNum: n,
				Files:      files,
				Message:    fmt.Sprintf("凭证号 %d 重复，出现在 %d 个文件：%s", n, len(files), strings.Join(files, "、")),
			})
		}
	}

	present := make(map[int]bool, len(nums))
	for _, n := range nums {
		present[n] = true
	}
	var missing []int
	for n := 1; n <= report.MaxNum; n++ {
		if !present[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		shown := missing
		suffix := ""
		if len(shown) > maxMissingListed {
			shown = shown[:maxMissingListed]
			suffix = fmt.Sprintf(" 等共 %d 个", len(missing))
		}
		report.Issues = append(report.Issues, SequenceIssue{
			Kind:    IssueMissingNum,
			Message: fmt.Sprintf("凭证号不连续，缺号：%s%s（请确认是漏录还是作废留空）", joinInts(shown), suffix),
		})
	}

	sortSequenceIssues(report.Issues)
	return report, nil
}

func joinInts(v []int) string {
	parts := make([]string, 0, len(v))
	for _, n := range v {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}

// sortSequenceIssues 输出确定性：类别 → 凭证号 → 首文件名。
func sortSequenceIssues(issues []SequenceIssue) {
	kindRank := map[SequenceIssueKind]int{IssueNonStandard: 0, IssueDuplicate: 1, IssueMissingNum: 2}
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if kindRank[a.Kind] != kindRank[b.Kind] {
			return kindRank[a.Kind] < kindRank[b.Kind]
		}
		if a.VoucherNum != b.VoucherNum {
			return a.VoucherNum < b.VoucherNum
		}
		af, bf := "", ""
		if len(a.Files) > 0 {
			af = a.Files[0]
		}
		if len(b.Files) > 0 {
			bf = b.Files[0]
		}
		return af < bf
	})
}
