# 打印版位格输出 技术设计

> 高层设计已获用户批准，见 [brainstorm-spec.md](brainstorm-spec.md)（Context/Goals/D1-D9 决策）。
> 本文档补充实现层细节：模块边界、数据流、错误处理、测试策略。

## Context

查看版生成管线：`cmd/generate.go` → `generator.GenerateWorkbook` → `wb.Save()` 落盘 `{output}/{year}/{year}-MM.xlsx`。账页 Sheet 三类：
- `总分类账-*`（GL 与 MergeGL 同前缀同格式）：两行表头（HeaderRow=4/SubHeaderRow=5），金额列借 glColDebit=5、贷 glColCredit=7、余额 glColBalance=10，Front(3-14 列)/Back(17-28 列) 两区
- `多科目明细账-*`（ML）：四行表头 h1-h4，Back 侧 借5/贷6/余额8 + 明细1-4，Front 侧明细5-14；首块为 Paper1 Front 占位页
- 其他（期初/期末表）：不动

布局常量由 `ledger/generator/layout` 包纯函数提供。打印版转换器只消费 Layout 公开坐标，不碰查看版写入路径。

## Goals / Non-Goals

**Goals:**
- 打印版 xlsx 输出到 `{output}/{year}/print/{year}-MM.xlsx`，与查看版唯一差异为金额栏位格化
- 查看版代码零改动（除 cmd 层调用点）；查看版文件字节级不受影响
- 打印版样式代码独立成新文件

**Non-Goals:**
- 不重写渲染器、不引入 HTML/PDF
- 不改期初/期末表
- 不做打印预览

## Decisions

### 架构：读副本 → 内存转换 → 另存（D1）

```
wb.Save() 落盘查看版
    └─ ConvertToPrint(viewPath, printPath):
         f := excelize.OpenFile(viewPath)      // 独立内存副本
         for each sheet:
             GL/MergeGL → convertGLSheet(f, sheet)
             ML        → convertMLSheet(f, sheet)
         f.SaveAs(printPath)
```

错误处理：ConvertToPrint 返回 error；cmd 层捕获后仅 `fmt.Fprintf(os.Stderr, ...)` 告警，主流程继续成功返回（铁律三：xlsx 是生成品）。磁盘上的查看版在 OpenFile 之后任何失败都不会被触碰。

### 插列编排（D2）

每个 Sheet 内**从右往左**对金额列依次插 11 列——右侧列先处理完，左侧插列不影响已处理的右侧坐标。坐标序列在插列前基于原始 Layout 常量预计算：

- GL Front 区：offset {5,7,10} → 原始 Excel 列 `FrontStartCol+offset`（即 9,11,14）
- GL Back 区：同样 offset → `BackStartCol+offset`
- ML Back 区：mlOffDebit/Credit/Balance + mlDetailCol(0..3)；ML Front 区：mlDetailCol(4..13)
- Paper1 占位块只有 Front 侧明细 5-14

每列插入后立即设置 12 个小列宽 = 原宽/12（先 GetColWidth 再 SetColWidth ×12）。

### splitCNY 拆位（D3）

```go
func splitCNY(cents int64) [12]string
// cents<0 取绝对值；cents==0 返回全空数组
// 位序：[十亿 亿 千 百 十 万 千 百 十 元 角 分]
// 实现：绝对值后按位取 digit := (abs / pow10[i]) % 10，i 从分(10^0)到十亿(10^11)
// 前导零格留空（""），首个有效位起填数字字符串
```

表驱动单元测试覆盖：0 / 1分 / 5元整 / 12345.67 / 99999999999.99（十亿上限）/ 负数。

### 表头改造（D4）

GL（每面区独立执行）：
1. Unmerge 原大标题纵向合并（如「借 方」占 HeaderRow 单行单列——实际为 headerStyle 区域内的单元格，需按实际合并清单处理）
2. 大标题文字写入 HeaderRow，跨 12 小列 MergeCell，样式复制原 header 样式
3. 小列标签「十 亿 千 百 十 万 千 百 十 元 角 分」逐格写入 SubHeaderRow 的 12 个小格，样式沿用 subHStyle（9 号绿字四边框）

ML：
1. 借/贷/余额原 h1:h3 纵向合并 → Unmerge 后重新 Merge 为 h1:h3 × 12 小列的矩形区域（12 个独立合并，每小列一个）
2. 明细科目名 h2:h3 合并同理扩展为 12 小列宽
3. 「( )方金 / 额 分析」h1 合并区随其下明细列拆分同步扩展
4. 小列标签写入 h4 行对应 12 小格
5. Paper1 占位页仅处理 Front 侧

### 分组边框常量表（D7）

```go
// dividerStyle[i] = 第 i 小列与其右邻之间的竖线样式（i: 1..11）
// 1:粗(组界 亿|十亿) 2:细 3:细 4:粗(万|十万...实为百万位|十万位) 
// 5:细 6:粗(千位|百位) 7:细 8:细 9:红细(元|角) 10:细 11:细
```

以 `[11]int` 常量表表达（值映射：1=绿粗 Style2，2=红细 CC0000 Style1，0=绿细 006100 Style1），单元测试断言表内容与需求一致。数据行边框派生算法见 brainstorm-spec D6：第 1 小格继承原左边框语义、第 12 小格继承原右边框语义、中间竖线替换、上下边框原样复制全部 12 格。

### 分页符重建（D5）

InsertCols 不平移 colBreaks（excelize 源码 TODO 已核实）。方案：插列完成后读取现有 RowBreaks/ColBreaks（GetRowBreaks/GetColBreaks），删除旧垂直分页符，按「原列号 + 该列左侧累计插入数」重插：
- GL：PageGapStartCol+1 → +33（Front 区 3 金额列 ×11）
- ML：PageGapStartCol+2 → +77（Back 区 7 金额列 ×11）
水平 RowBreaks 不受插列影响，保留即可。

### ML 标题区合并修复（D5 补）

ML 科目名合并横跨明细 12-13 列（位于插列区内）。不依赖 excelize 自动平移结果：插列前记录所有跨金额区的合并区坐标，插列后 Unmerge 全部再按「原坐标 + 累计偏移」重设。

## Risks / Trade-offs

- [InsertCols 对富文本单元格的行为未完全验证] → e2e 抽查年份富文本是否完好；异常则改为插列前摘除富文本、插列后回写
- [÷12 列宽亚毫米漂移] → 接受；补偿常量留待实机打印校准
- [小格 6~7pt 字号可读性] → 常量化，一次实机打印定稿
- [多次插列的性能]（每 Sheet 约 33~77 次 InsertCols，每次 O(cells)）→ 月度 Sheet 数量有限（数十个），实测若超 30s 再优化为一次性批量重建
- [转换器与未来 Layout 演进脱节] → 只 import layout 公开常量；Layout 变更编译期报错暴露

## Migration Plan

无数据迁移。部署即生效于下一次 `generate`。历史月份打印版可对已有查看版 xlsx 手动调用 ConvertToPrint 补生成（入口函数天然支持）。

## Open Questions

无——设计决策已全部由用户确认或授权 AI 决策。
