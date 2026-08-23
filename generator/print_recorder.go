// Package generator — 打印版位格输出：页面记录器。
//
// 在 GenerateWorkbook 运行期间，各写入函数通知 recorder 记录结构化数据。
// 生成结束后，打印渲染器消费 recorder 产出打印版工作簿。
// 数据来源是 Go 内存变量，不经过 xlsx 格式。
package generator

// SheetKind 账页类型。
type SheetKind int

const (
	SheetGL SheetKind = iota // 总分类账（含合并总账）
	SheetML                  // 多科目明细账
)

// RowKind 行类型。
type RowKind int

const (
	RowEntry         RowKind = iota // 分录行
	RowCarryForward                 // 承前页 / 上年结转
	RowPageBreak                    // 过次页
	RowMonthlyClose                 // 本月合计
	RowQuarterlyClose               // 本季合计
	RowYtdClose                     // 本年累计
	RowPeriodEnd                    // 期末余额
)

// PageRecorder 累积所有账页的结构化数据。
type PageRecorder struct {
	sheets map[string]*SheetRecord
}

// NewPageRecorder 创建空记录器。
func NewPageRecorder() *PageRecorder {
	return &PageRecorder{sheets: make(map[string]*SheetRecord)}
}

// SheetRecord 单个 Sheet 的全部页面记录。
type SheetRecord struct {
	Name  string
	Kind  SheetKind
	Pages []PageRecord
}

// PageRecord 单页记录。
type PageRecord struct {
	PageNum int
	Account string // 科目名
	Year    string // 年份（如 "2026"）
	Rows    []RowRecord
}

// RowRecord 单行结构化数据。
type RowRecord struct {
	Kind    RowKind
	Date    string // "MM-DD" 或 ""
	Voucher string // 凭证号或 ""
	Summary string
	Dir     string // 借/贷/平 或 ""
	Debit   int64  // 分
	Credit  int64  // 分
	Balance int64  // 分（显示余额，非负）
	Details []int64 // ML 明细净额，长度 = mlMaxDetails；GL 为 nil
}

// getOrCreateSheet 获取或创建 SheetRecord。
func (r *PageRecorder) getOrCreateSheet(name string, kind SheetKind) *SheetRecord {
	if s, ok := r.sheets[name]; ok {
		return s
	}
	s := &SheetRecord{Name: name, Kind: kind}
	r.sheets[name] = s
	return s
}

// getOrCreatePage 获取或创建 PageRecord（按 pageNum 归入）。
func (s *SheetRecord) getOrCreatePage(pageNum int, account, year string) *PageRecord {
	for i := range s.Pages {
		if s.Pages[i].PageNum == pageNum {
			return &s.Pages[i]
		}
	}
	s.Pages = append(s.Pages, PageRecord{PageNum: pageNum, Account: account, Year: year})
	return &s.Pages[len(s.Pages)-1]
}

// RecordGLRow 记录 GL 一行数据。
func (r *PageRecorder) RecordGLRow(sheet string, pageNum int, account, year string, row RowRecord) {
	if r == nil {
		return
	}
	s := r.getOrCreateSheet(sheet, SheetGL)
	p := s.getOrCreatePage(pageNum, account, year)
	p.Rows = append(p.Rows, row)
}

// RecordMLRow 记录 ML 一行数据。
func (r *PageRecorder) RecordMLRow(sheet string, pageNum int, account, year string, row RowRecord) {
	if r == nil {
		return
	}
	s := r.getOrCreateSheet(sheet, SheetML)
	p := s.getOrCreatePage(pageNum, account, year)
	p.Rows = append(p.Rows, row)
}

// Sheets 返回所有记录的 Sheet（按名称排序，便于稳定输出）。
func (r *PageRecorder) Sheets() []*SheetRecord {
	if r == nil {
		return nil
	}
	out := make([]*SheetRecord, 0, len(r.sheets))
	for _, s := range r.sheets {
		out = append(out, s)
	}
	// 按名称排序
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
