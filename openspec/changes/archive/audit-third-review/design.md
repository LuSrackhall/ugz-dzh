## Context

第三轮专家审查 + 实测复现 D1（合并父级直接记账 → 月结两遍 + 期初污染 + 期初虚增）；F1/N1/N2 为低成本输入防线。

## Goals / Non-Goals

**Goals:**
- D1：禁止合并父级直接记账；期初不受合并视图污染；合并期初只聚叶子
- F1：跨年结转三告警（非 12 月 / 期初不平 / 损益漏结转）
- N1 双填提示、N2 全角括号红字

**Non-Goals:**
- 不改合并视图 sheet 命名（隔离方案改动大；禁止+隔离期初已根治 D1）
- 不处理其余可忽略项（AlreadyGenerated 同词、sheet 名后缀等）

## Decisions

**D1a 禁止直接记账**：GenerateWorkbook 校验 `合并总账科目` 科目出现无明细分录 → 报错。冲突配置在源头拦截（CLI 内建安全）。

**D1b 期初隔离**：ExtractLastMonthFinals 跳过合并总账科目的 sheet（合并视图非账页）。父级期初不再被合并期末污染；父级自身 initials 由子科目聚合覆盖。

**D1c 聚合修正**：WriteMergeGLClosings `parentInitial` 去掉 `+= initials[general]`，仅 Σ 子科目 initials。

**F1 三告警不阻断**：year-close 非 12 月告警、期初不平告警（CheckInitialBalanceAt diff）、损益类（收入/费用）年末非 0 告警。不阻断原因：结转是用户主动操作，历史数据可能不平（e2e 实测 -15760.35），报错会卡死结转。

**N1/N2**：双填 warning（借红+贷蓝可能合法）；全角括号转半角后走既有括号逻辑。

## Risks / Trade-offs

- **D1a 行为变化**：配置了合并总账科目 + 直接记父级的历史数据将报错——正确行为（该配置本身错误）。
- **F1 告警 vs 报错**：告警不阻断，靠用户确认；符合"CLI 安全但不卡死历史数据"原则。
- **D1b 影响**：合并父级不再进入 prevFinals——若用户依赖父级期末做期初（错误用法），行为变为 0/子科目聚合（正确）。
