## Context

两位专家二审发现 3 个高危（H1 期初回退翻旧账 / H2 红字解析与打印失真 / H3 合并累计漏计），均已复核确认。用户决策：全部修复。

## Goals / Non-Goals

**Goals:**
- 期初回退语义正确（最近期末含 0）
- 红字四格式解析归一 + 打印版红色字体标记（手工账红笔惯例）
- 合并总账累计与 GL/ML 三路口径一致
- 每项带回归测试

**Non-Goals:**
- 不改红字"明确负数列示"数据语义（净额平衡口径保持）
- 不改查看版负数显示（已正确）
- 不处理中低危项（红字单侧误报 B2、凭证号未解析分组 D1、损益结转检测 F1 等）——记入已知限制
- 不改打印版红字"整行红色"（仅金额格红色，最小侵入）

## Decisions

**D1 期初回退（H1）**：`GetInitBalanceForGenerate` 第 4 步去掉 `mb.Final != 0` 条件，取 `m < month` 中**最新月份的 Final**（可为 0）。结平科目跨年取 0；无记录取 0。与 PurgePhantomInitials 无冲突（幻影清理针对"无发生额"记录，与本条件独立）。

**D2 红字解析（H2a）**：`parseAmountToCents` 顺序：去空白/千分位 → 检测并剥括号（`(…)` 且内容含数字 → 记 neg）→ 全角减号 `－`/Unicode `−` 归一为 ASCII `-` → 白名单清理 → 解析 → `neg && val > 0` 时取反。括号内已有 `-`（如 `(-500)`）自然为负，不再重复取反。

**D3 打印版红字（H2b）**：`amountSubStyle` 增加 `red bool` 参数；`red` 时字体颜色强制 `#CC0000`（覆盖继承色）；缓存 key 由 `[3]int{styleID,k,n}` 扩为 `[4]int{styleID,k,n,redFlag}`（防红/黑样式串用）。金额格写入处保存 `wasNegative`，负数拆位（绝对值）后样式传 red。仅**金额格**红色（最小侵入），不染整行。

**D4 合并累计（H3）**：`WriteMergeGLClosings` 本年/本季累计循环从 `for k := range activity` 改为 `for k := range wb.Config.Tree`（`isChildOf(k, general)`），取 `ytdDebit[k] + activity[k].Debit`（activity 无 key 时为 0，Go map 零值安全）。父级自身分录（activity[general]）逻辑保留。

## Risks / Trade-offs

- **缓存 key 扩大**：digitCache/labelCache 类型 map[[3]int]int → map[[4]int]int，改动集中 print_common.go。
- **红字打印视觉**：红色字体在黑白打印机上不可见——手工账红字本就依赖彩色；如需黑白可辨，后续可加"红色下划线/斜杠"变体（记入已知限制）。
- **H3 遍历 Tree**：Tree 含全部科目（叶子+父级），`isChildOf` 已过滤叶子归属，性能与 activity 遍历同级。
- **B2（红字单侧误报）**不处理：净额口径严格校验是 CLI 安全底线；"借红+贷蓝"若出现，报错信息已含差额，用户可自查。
