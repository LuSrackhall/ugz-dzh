# 设计：明细科目独立账页

## D1 配置与判定

- 字段：`GlobalSettings.DetailStandaloneAccounts []string`，json tag `明细账独立科目,omitempty`，紧跟 `多科目明细账忽略科目` 之后（配置文件可读性）。
- 匹配：**叶子全路径精确相等**（`wb.detailStandalone(path)`）。不做祖先段匹配——明细科目是叶子无子树，前缀匹配会引入"收益分配-未分配"式歧义（路径本身含连字符）。
- 默认空 = 全部路径短路返回，现行为零变化。

## D2 页面结构：完整复用 GL 第 2 层

用户需求"明细列自然为空"的替代方案是三栏式——GL 布局正是三栏式（借/贷/余），且布局宪法第 2 层（20+1 行、过次页/承前页、月结四行）与横向双面装订第 1 层全部免费获得。

- 页名：`sheetPrefixDetail = "明细账-"`，`sheetNameDetail(path)`。
- 标题：`writeGLTitle` 参数化为 `writeLedgerTitle(sheet, title)`（现从 sheet 名剥 `总分类账-` 前缀取科目名处改为按前缀剥离）；GL 传"总  分  类  账"、明细传"明  细  账"。
- 页头：`writePageHeader` 内硬编码的"总  分  类  账"按 sheet 前缀切换（一处三元）。

## D3 追加本体共享（回归面最小化）

`appendToGLSheet` 拆为薄入口 + 共享本体：

```go
// 现签名保持，GL 入口
func (wb *Workbook) appendToGLSheet(account string, entries []voucher.Entry, initial int64) error
// 新：独立明细页入口
func (wb *Workbook) appendToDetailSheet(account string, entries []voucher.Entry, initial int64) error
// 共享本体（期初行 / 翻页 / 分录写入 / 样式全在这）
func (wb *Workbook) appendToLedgerSheetBody(sheet, account string, isNew bool, entries []voucher.Entry, initial int64) error
```

入口只差 `ensureGLSheet` vs `ensureDetailSheet`（后者调 `writeLedgerTitle`）。期初行逻辑（isNew→insertCarryForward；1 月→"上年结转"；调整额→"期初余额"）与续写余额（`lastPageBalance`）对独立页语义完全正确（initials[path] 来自 JSON 权威链，免费获得双源保护）。

新步骤 6.2 `AppendDetailLedgerEntries`（generate.go）：
- 分录按 path 分组，命中名单 → `appendToDetailSheet`；
- 期初≠0 且无当月分录的名单科目 → 同样建页写期初行（对仗 appendCarryForwardOnly；仅 1 月或调整额生效时写行）；
- initial==0 且无分录 → 不建页。

## D4 路由矩阵（加法而非搬移）

| 产物 | 名单命中明细 | 依据 |
|---|---|---|
| GL 叶子页 | 照常全套（期初/分录/月结/finalize） | 用户拍板"互不影响、不提示不联动" |
| 合并 ML 页 | 明细列排除（列收缩）；余额列不变（仍含该明细发生额） | "明细列自然为空"的等价物是列不出现 |
| 独立明细页 | 新建全套 | 本变更主体 |
| JSON/报表/试算/回写 | 零改动 | 拆的是页不是账 |

ML 排除实现点：`AppendMLEntries` 构建 `g.details` 时跳过 `detailStandalone(General+"-"+Detail)` 的明细 → `ensureMLSheet` 不会为其建列（列自然收缩，`明细列顺序` 配置求差集发生在 resolve 之前）。`WriteMLMonthClosings` 零改动（detailIdx 从存量表头读，`-f` 重建后无该列；余额列/本月合计口径本来就是总账全口径）。

## D5 存量收敛

- **孤儿独立页**：generate 存量清理段（GL 清理之后）遍历 `明细账-` 前缀 sheet，key 不在当前名单 → `DeleteSheet`（对仗"总分类账忽略科目"存量清理，名单变更对存量账本收敛）。
- **合并 ML 残留列**：`AppendMLEntries` 检测 `readMLDetailHeaders` 结果含名单内明细 → 报错"明细账独立科目 %s 已配置，但存量多科目明细账页仍含其列，请使用 -f 从首月重新生成"（对仗 checkMLDetailOrderConflict 先例——不做运行时删列，列收缩只发生在生成期，历史文件铁律一不动）。

## D6 月结：循环体抽出共享

`WriteMonthClosings` 循环体（本月合计/本季合计/本年累计/期末余额 ~230 行）抽为 `writeLedgerClosings(sheet, account, act, ...)`；GL 循环照旧调用（`sheetNameGL`），新增独立页循环（`sheetNameDetail`，changedSheets 键为明细页名）。`CollectChangedSheets` 为命中名单的 path 追加独立页键。

禁止方案：在 GL 月结循环里改 sheet 名选择——那会把 GL 月结路由到独立页，违反 D4。

## D7 打印与排序

- `TransformToPrint` 加 case `明细账-` 前缀 → `transformGLSheet`（该函数纯 sheet 参数化 + glLayout，无前缀假设，12 列拆位直接适用；独立页金额量级远小于 GL，位数上限只多不少）。
- `isLedgerSheet`（print_font.go）与 `setAllSheetPageLayout` GL 分支（workbook.go，B5 横向 74%）扩前缀。
- `reorderSubjectSheets`：`sectionDetail = 2`，ML 顺延为 3（GL→合并→明细账→ML；明细账页与 GL 同构，排在 ML 前；`科目顺序` 配置对独立页照常生效，key=叶子全路径）。

## D8 交叉语义

- 名单科目同时被 `总分类账忽略科目` 忽略 → GL 叶子页无（既有语义），独立页照建（独立名单独立判断，不联动）。
- 名单明细所属总账科目被 `多科目明细账忽略科目` 忽略 → 合并 ML 页整体不存在（无列冲突），独立页照建。
- 合并总账父级不可进名单（它是总账非明细；D1a 语义一致性，generate 时对名单命中合并科目报错）。
- `ExtractLastMonthFinals` 只扫 `总分类账-` 前缀，独立页不干扰期末提取（天然隔离，无需改）。

## D9 验证设计

- 单测：`detailStandalone` 精确匹配（含连字符路径不误伤）；ML 列排除与冲突报错；`ensureDetailSheet` 页名/标题；共享本体期初行标签；存量孤儿清理；合并父级进名单报错。
- e2e：多名单/非名单明细混跑数月 → 断言 GL 叶子页在、独立页期初/分录/月结/余额链连续、合并 ML 页无名单列且余额不变、JSON 链与报表逐科目不变、排序区块、名单移除 + `-f` 收敛。
- 全量：`go test ./... -count=1` + `bash scripts/test-e2e.sh`；技能双目录 diff 为空（embed_test 守护）。
