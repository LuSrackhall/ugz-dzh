## Context

审计发现期初机制五个缺陷：期初调整额死字段（改了不生效）、自动识别科目首月净额污染期初链路（M2 幻影期初）、科目属性按净额猜（M1）、期初行摘要不分来源、无期初试算平衡校验。项目宗旨：JSON 唯一权威源、git 管变更、余额链连续（铁律二）、历史 xlsx 永不修改（铁律一）。

> **Note（2026-08-26 被 audit-initial-anchor 修正）**：本 design 中"生效月==当前月/首次月==当前月"的调整额触发条件已改为"**启动月（建账月）==当前月**"（期初锚定建账月）；`生效月`/`首次月份` 字段降级为信息性；`ensureBackfillForAll` 回填机制已删除；`add-manual` 改为幂等更新。D1/D3 与"期初前推"小节已被 audit-initial-anchor 取代，详见 openspec/changes/audit-initial-anchor/。

## Goals / Non-Goals

**Goals:**
- 让"期初调整额"（手动调整科目 + 自动识别科目）真实生效，且不破坏余额链连续
- 根除幻影期初：自动识别科目首月净额不再进入期初链路；历史幻影数据可自愈清理
- 科目属性不再按净额推断
- 期初行摘要按来源区分（上年结转 / 期初余额），补齐中途建账期初行
- 期初试算平衡可校验（check 报错、generate 告警）

**Non-Goals:**
- 不改 year-close 跨年结转本身（2026-01 期初=2025-12 期末语义保持，H1 已修）
- 不改余额公式、运行余额、三路生成（GL/MergeGL/ML）骨架
- 不引入"期初设置"新数组结构（保持现有 AutoItems/ManualItems 字段，避免破坏已有 JSON）
- 不做打印版相关改动

## Decisions

**D1 调整额仅在"生效月==当前月"直取，其余月份续链（铁律二优先）**
`GetInitBalanceForGenerate` 新优先级：
1. 手动调整科目：存在 `生效月==当前月` 且 调整额≠0 → 期初=调整额（分）
2. 自动识别科目：存在 `首次月==当前月` 且 调整额≠0 → 期初=调整额
3. prevMonthEnd（上月 xlsx 期末）
4. 科目树 Balances 最近非零期末（m<month）
5. `FirstRecord.Month==month` → FirstRecord.Amount（自动科目已置 0，此步对自动科目恒 0）
6. 0
生效月之后的月份走 3/4 续链，期初=上月期末，余额链连续。

**D2 跨年安全**：自动识别科目 `首次月份` 为上年，2026 各月≠上年首次月 → 第 2 步不命中，2026-01 期初=prevFinals（2025-12 期末）。与 H1"跨年取期末做期初"一致，调整额不覆盖上年末。

**D3 中途补记工作流 = -f 级联重建**：生效月早于当前月时，历史 xlsx 不可改（铁律一），文档规定用 `-f` 自生效月起级联重建（`-f` 语义本就是"从指定月/当月开始重建"，符合铁律一）。重建时生效月==当前月命中调整额。

**D4 幻影期初迁移 `PurgePhantomInitials(cfg)`（幂等自愈）**
对 `首次记录.方式=="自动识别"` 的科目：`首次记录.金额` 置 0；删除 Balances 中 `月份<首次月` 且 `借方==0 && 贷方==0` 的记录（首次月之前不可能有发生额，此类记录只可能是 ensureBackfillForAll 回填产生）。generate 加载 JSON 后自动执行，重跑即清理历史幻影期初。

**D5 属性按类别推断**：`inferPropertyByType(general)`：资产/费用→借，负债/权益/收入→贷，未知→借。替代 `InferAccountProperty(净额)`。用户可 `SetAccountProperty` 覆盖；`check` 对未知类别科目提示。

**D6 期初行摘要 + initialSource**：生成期初映射时同步构建 `initialSource[account]`（命中 D1 第 1/2 步 → `adjustment`，否则 `carryover`）。摘要规则：`adjustment`→"期初余额"；`carryover` 且当月 1 月→"上年结转"；其余→"期初余额"。触发条件扩展：已有 Sheet 且期初≠0 且（当月 1 月 **或** initialSource==adjustment）→ 插期初行（原实现仅限 `-01`）。

**D7 期初平衡校验分层**：`check` 取全体科目**最新月份快照**（各科目 Balances 最大月，取全体最大月）校验期初 借合计=贷合计，不平报错；`generate` 对当月 initials 校验，不平告警不阻断（避免历史数据卡死）。

**D8 -f 级联重建基线正确（中途补记工作流的前提，实测发现并修复）**
原实现 `-f` 只删除"晚于当月"的 xlsx（`> month`），当月旧文件被 `NewWorkbook` 加载（其"当月存在→加载"分支本意是 year-close 预生成空文件），随后 `ExtractLastMonthFinals` 把**旧版当月的期末**当作"上月期末"→ 期初被旧数据污染（实测 -f 重建 2025-10 后，银行存款期初凭空出现 -118119.59）。修复：
1. `cmd/generate.go`：`-f` 删除条件 `> month` → `>= month`（当月及以后全部删除）；
2. `generator/workbook.go` `NewWorkbook`：复制上月前校验**同一年度**（`filepath.Base(prevPath)` 前缀 == `wb.Month[:5]`），跨年（新年首月）不复制旧年 12 月，从空工作薄开始、期初取自 JSON 上年末余额。
year-close 预生成空文件场景（当月文件存在 → 加载，不带 -f）不受影响。

## Risks / Trade-offs

- **迁移误删风险**：`月份<首次月 且 无发生额` 的删除条件已限定唯一可能是回填记录；真实余额记录必有发生额或首次月之后。低风险，且幂等可复核。
- **用户改调整额的生效时点**：首次月之后才改 AutoItems.Adjustment → 需 `-f` 从首次月重建才生效。文档（README）明示该工作流。
- **`-f` 重建范围**：仅重建当月及以后，不影响更早历史（铁律一允许）。
- **check 基准月**：取全体科目最新月份（可能部分科目无该月记录，按 0 处理），规则写明。
- **行为变化**：已有 JSON 中 AutoItems.Adjustment 若被用户手动改过非零值，修复后将开始生效（这正是修复目的），需在验收中覆盖。
- **已知限制（三路中途补记不一致）**：调整额生效月（非 1 月）的中途补记期初行仅 GL 插入；ML/MergeGL 对已有 Sheet 仍沿用 lastPageBalance 旧链（跨年 1 月场景三路均正确，因为旧链期末=新年期初）。GL 与 ML 在中途补记场景会显示不同的余额起点——spec 已明确该限制，三路一致化列入后续独立 change（涉及 ML 明细列聚合起算，复杂度高）。
- **-f 删除范围**：仅删除账本文件（YYYY-MM.xlsx），`isMonthXlsxName` 防误删 ledger.xlsx/balance.xlsx 汇总文件（子 agent 验收发现并修复）。

- **审计低优先级项（L1-L3）未处理**：L1 期末表合计口径、L2 期初/期末表零余额科目、L3 ML 月结行借贷/明细口径混用——审计定位"建议但不紧急"，本批修复（H2/M1/M2/M3/M4/死字段）未覆盖；L4 红字负数列示口径已在 audit-voucher-balance 明确。后续如需处理单独开 change。
