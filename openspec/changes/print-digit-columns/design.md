# 打印版位格输出 技术设计（v2 — 记录器架构）

> 高层设计见 [brainstorm-spec.md](brainstorm-spec.md)（需求/范围/拆位规则/边框规范不变）。
> v1 尝试了「生成后转换副本 + InsertCols」方案，因 excelize 多次插列行为不可控（表头与数据错乱 5~27 列）而放弃。
> v2 改为「生成时同步记录结构化数据 + 独立渲染器」，数据零次经过 xlsx 格式。

## Context

查看版生成管线：`cmd/generate.go` → `generator.GenerateWorkbook` → `wb.Save()` 落盘 `{output}/{year}/{year}-MM.xlsx`。
账页 Sheet 三类：`总分类账-*`（GL/MergeGL）、`多科目明细账-*`（ML）、其他（期初/期末表不动）。

核心约束：**查看版和打印版的数据必须强一致**——来自同一行 Go 代码里的同一个变量，不经过 xlsx 中转。
用户只验证查看版（Excel 里求和/核对余额链），打印版自动继承正确性。

## Goals / Non-Goals

**Goals:**
- 打印版 xlsx 输出到 `{output}/{year}/print/{year}-MM.xlsx`
- 数据与查看版强一致：同一 Go 变量 → 同时写 view + 记录 IR → 打印渲染器消费 IR
- 查看版输出字节级不受影响
- 打印版样式代码独立成新文件

**Non-Goals:**
- 不改期初/期末表
- 不做打印预览
- 打印版不跨月累积（每月从同月查看版的生成过程中直接产出）

## Decisions

### D1: 生成时同步记录 + 独立渲染器

```
GenerateWorkbook 运行中：
  appendToGLSheet / writePageBreakRow / WriteMonthClosings / ...
    → 写 view xlsx（现有逻辑不变）
    → 同时通知 pageRecorder.Record(sheet, row, {debit, credit, dir, balance, ...})

GenerateWorkbook 结束后：
  wb.Save()  → 落盘查看版
  RenderPrintVersion(wb.recorder, printPath) → 全新打印版工作簿
```

- **数据来源**：Go 内存变量（e.DebitCents、balance、pageDebit 等），不读 xlsx
- **一致性保证**：view 和 print 的数据来自同一个变量，由构造保证
- **失败策略**：打印版生成失败仅告警，不影响主流程

### D2: 记录器（PageRecorder）

```go
type PageRecorder struct {
    sheets map[string]*SheetRecord  // sheet名 → 记录
}
type SheetRecord struct {
    Name  string
    Kind  SheetKind  // SheetGL, SheetML
    Pages []PageRecord
}
type PageRecord struct {
    PageNum int
    Rows    []RowRecord
}
type RowRecord struct {
    Kind    RowKind  // RowEntry, RowCarryForward, RowPageBreak, RowMonthlyClose, RowPeriodEnd, ...
    Date, Voucher, Summary, Dir string
    Debit, Credit, Balance int64  // 分
    Details []int64  // ML 明细净额，长度 = mlMaxDetails
}
```

集成点（~15 处，每处加一行 `wb.recorder.Record(...)` 调用）：
- GL: appendToGLSheet（分录行）、writePageBreakRow、writeCarryForwardRow、insertCarryForward、WriteMonthClosings（本月合计/本季合计/本年累计/期末余额）
- ML: appendToMLSheet、writeMLPageBreakRow、writeMLCarryForwardRow、WriteMLMonthClosings

### D3: 打印渲染器

全新工作簿，从零构建每个 Sheet：
1. **列布局计算**：基于 glLayout()/mlLayout() 的列比例，将金额列展开为 12 小列（纯算术，不插列）
2. **逐页渲染**：遍历 SheetRecord.Pages，按页写标题区/表头/数据行/过次页/承前页
3. **金额拆位**：splitCNY(cents) → 12 个字符串写入小格
4. **边框**：分组常量表 dividerStyles 控制竖线；上下边框从查看版同位置的样式语义复制（每5行加粗、过次页红双线等——这些信息在 RowRecord.Kind 中可推导）
5. **分页符**：按计算出的列位置直接写入，无需重建

### D4: splitCNY 拆位（不变）

```go
func splitCNY(cents int64) [12]string
// 负数取绝对值；0 全空；前导零留空；个位对齐「元」列
```

### D5: 分组边框常量表（不变）

```go
var dividerStyles = [11]int{...}
// 组界加粗：十|亿(0)、百|十(3)、千|百(6)
// 元|角(8) 红色单细线；其余绿细线
```

### D6: 文件划分

```
generator/print_recorder.go   PageRecorder + SheetRecord + RowRecord 类型定义
generator/print_render.go     GL/MergeGL 打印渲染器（消费 SheetRecord → 新工作簿）
generator/print_render_ml.go  ML 打印渲染器
generator/print_common.go     splitCNY + dividerStyles + 列布局计算工具 + 小列标签
generator/print_common_test.go  单测（splitCNY、dividerStyles、列布局计算）
cmd/generate.go               GenerateWorkbook 成功后调用 RenderPrintVersion + print/ 级联清理
```

现有写入函数（gl_sheet.go / ml_sheet.go / monthly_close*.go）每处加一行 recorder 通知，不改计算逻辑。

## Risks / Trade-offs

- [recorder 通知遗漏] → 每个通知点对应一个 RowKind，e2e 抽查打印版行数与查看版一致即可发现遗漏
- [列布局计算与查看版 Layout 偏差] → 打印渲染器复用 glLayout()/mlLayout() 的列比例，只改物理列数；行结构完全相同
- [小格字号可读性] → 常量化（初始 7pt），实机打印校准
- [打印版不跨月累积] → 每月独立生成，不涉及复制上月文件；页码从1开始（与查看版独立）

## Migration Plan

无数据迁移。部署即生效于下一次 `generate`。
