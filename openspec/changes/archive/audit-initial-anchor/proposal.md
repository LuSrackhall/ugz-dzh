## Why

会计专家评审（agent-5f3aab24，对照《会计基础工作规范》与主流财务软件实务）确认：**期初余额是账套启用时点的存量，是时点概念**；当前实现（audit-initial-balance 的"生效月"语义——允许任意月声明"从某月起期初=某金额"）**不符合会计**：会切断"上期期末=本期期初"余额链，产生与当月发生额冲突的畸形账页。用户主张（采纳）：期初只属于建账月（`init` 启动月），中途补记业务一律走凭证路径（有凭证号）。

## What Changes

1. **期初调整只落建账月**：`GetInitBalanceForGenerate` 调整额生效条件从"生效月==当月"改为"**启动月==当月**"——仅在生成启动月（建账月）时直取调整额；其余月份一律走上月期末续链。`HasInitialAdjustment` 同步。
2. **废弃"生效月"计算语义**：`ManualItems[].EffectiveMonth` 字段保留（JSON 兼容、信息性：记录补录时点），**不再参与期初计算**；`AutoItems[].FirstMonth` 同理不再作为调整额生效条件。
3. **删除期初回填**：`ensureBackfillForAll` 及其调用删除（启动月为最早月，期初表直接由 initials 构建，无需回填）。
4. **add-manual 改为"设置/修改建账月期初值"**：同科目已有手动调整条目 → 更新调整额/说明（不再报"已存在"），便于修正期初；无则追加。
5. **中途补记业务走凭证**：README 明确"漏记业务补凭证（带凭证号），期初调整仅作用于建账月"。

## Capabilities

### New Capabilities
- `initial-anchor`: 期初余额锚定建账月——调整额仅在建账月生效、生效月字段废弃（信息性）、无回填、add-manual 幂等更新

### Modified Capabilities
- `initial-balance`: 期初调整额生效条件从"生效月==当月"改为"启动月==当月"；回填机制删除（audit-initial-balance 的 D1/D3/回填设计被本 change 修正）
- `cli-commands`: `add-manual` 行为变化——同科目更新而非报错

## Impact

- `balance/balance.go`：`GetInitBalanceForGenerate` / `HasInitialAdjustment` 生效条件、`AddManualAdjustment` 更新语义、删除 `ensureBackfillForAll`
- `balance/balance_test.go`：更新生效月→建账月用例；删除 TestBackfill；新增建账月/非建账月/add-manual 更新用例
- `cmd/add_manual.go`：`-m` 参数保留（信息性），帮助文案更新
- README：期初调整章节（锚定建账月、中途补记走凭证）
- 不变：跨年结转（year-close 自动结转）、PurgePhantomInitials、借贷平衡、幂等、e2e（Change 1-4）
