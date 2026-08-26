## Context

audit-initial-balance（Change 1）实现了"生效月"语义（允许任意月声明期初）。会计专家评审（agent-5f3aab24）指出该语义不符合会计准则：期初余额是账套启用时点的存量，中途"插入期初"切断余额链。用户采纳修正：期初只落建账月（启动月）。

## Goals / Non-Goals

**Goals:**
- 期初调整额只在建账月（启动月）生效，其余月份续链
- 废弃"生效月"计算语义（字段保留为信息性）
- 删除期初回填（启动月为最早月，无需回填）
- add-manual 幂等更新（改期初值）

**Non-Goals:**
- 不改跨年结转（year-close 自动结转，新年期初=上年末）
- 不改凭证路径（中途补记业务 = 凭证，已有）
- 不做 PurgePhantomInitials 之外的历史数据清理

## Decisions

**D1 锚定建账月**：`GetInitBalanceForGenerate` 中调整额分支条件 `m.EffectiveMonth == month` / `a.FirstMonth == month` → 统一改为 `month == cfg.Settings.StartMonth`（启动月）。生成启动月时直取调整额；其余月份不读调整额。`HasInitialAdjustment` 同步：仅 `month == StartMonth` 且调整额≠0 → true（决定账页"期初余额"行与摘要）。

**D2 生效月字段降级为信息性**：JSON 结构保留 `EffectiveMonth`/`FirstMonth` 字段（兼容旧数据），不参与计算。`add-manual` 的 `-m` 参数保留，仅记录补录时点。

**D3 删除回填**：`ensureBackfillForAll` 删除（及两处调用）。启动月为最早月，期初表直接由 initials 构建（启动月生成时 initials 含调整额），无需回填历史月份。

**D4 add-manual 幂等更新**：遍历 `ManualItems`，同科目已有条目 → 更新 Adjustment/Note（及 EffectiveMonth 若传入）；无 → 追加。Tree 节点同步（FirstRecord.Amount=调整额、Property 按类别）。修正建账月期初值的命令行入口。

**D5 中途补期初 = 改调整额 + -f 从建账月重建**：建账月已生成时改期初，必须 `generate -f` 从启动月起级联重建（铁律一：历史不重写；会计上改期初须追溯）。README 明确该工作流。

## Risks / Trade-offs

- **行为变更**：Change 1 的"生效月"场景（-m 2026-03 生效）不再生效——符合会计，但旧 JSON 中 EffectiveMonth 非启动月的条目行为变化（不再按生效月触发），需在验收中确认。
- **全量重建成本**：改建账月期初需 -f 从启动月重建全部月份——会计追溯的正当代价。
- **自动识别科目调整额**：若用户给 AutoItems 设调整额且 FirstMonth 晚于启动月，修正后调整额在启动月生效（期初表列该科目）——尊重用户显式声明，语义一致。
