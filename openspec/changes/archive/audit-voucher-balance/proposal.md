## Why

审计（docs/audit-2026-08-26.md）H2：**全库无"每张凭证借方合计=贷方合计"校验**。凭证解析接受负数（红字），借贷不平衡的凭证会静默通过，导致账本失衡而无人报警（e2e 测试数据实测存在 -15,760.35 的累积差额，期初平衡校验（Change 1）才第一次暴露）。CLI 是绝对安全对象，凭证错误必须在 generate 前拦截。

## What Changes

1. **新增 `voucher.ValidateVoucherBalance(entries) (warnings, error)`**：
   - **逐张凭证平衡校验**：按（日期, 凭证号）分组，校验 `Σ|借方| == Σ|贷方|`（绝对值口径，兼容红字）；不平衡 → 返回 error（含凭证号与差额），generate 拒绝生成。
   - **红字提示**：借方或贷方为负数 → warning"该行为红字，将显示为负金额"；**不自动折入对侧**（折入会破坏绝对值平衡，且改变用户数据语义），采用审计 H2 的"明确负数列示"口径。
   - **凭证号未解析保护**：VoucherNum<=0 的条目无法可靠分组，跳过平衡校验并 warning 提示（解析警告已在 parser 中给出）。
2. **generate 内建校验**：`cmd/generate.go` 在 `CollectEntries` 后立即调用；error → 拒绝生成（CLI 内建安全，不靠使用习惯）；warnings → 打印提示。
3. **精度**：金额解析已由 `parseAmountToCents` 统一处理（Round 到分），本 change 不重复校验。

## Capabilities

### New Capabilities
- `voucher-balance`: 凭证借贷平衡校验——逐张凭证绝对值平衡、红字提示、凭证号未解析保护

### Modified Capabilities
- `cli-commands`: `generate` 新增凭证借贷平衡前置校验（不平拒绝生成）

## Impact

- 新增 `voucher/balance_check.go`：`ValidateVoucherBalance`
- 新增 `voucher/balance_check_test.go`
- `cmd/generate.go`：CollectEntries 后调用校验
- 不变：凭证解析、金额换算、生成流程、期初机制（Change 1）
