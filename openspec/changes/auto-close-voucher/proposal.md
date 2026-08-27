## Why

用户确认：损益结转凭证由系统自动生成，但**不得污染手工凭证目录**。推荐方案（用户选定）：输出目录下 `{year}/closing/` 专属子目录，generate 自动扫描并入。

## What Changes

1. **`ledger gen-close` 新命令**：读取 `{year}.json`，对仍有余额的损益类（收入/费用）科目生成结转凭证（借/贷 本年收益 按余额方向，借贷平衡），写入 `<output>/{year}/closing/记字第X号 年末损益结转.md`（标准凭证格式，可被解析器识别）；自动跳过已结转科目（扫描 closing/ 已有凭证）；编号自动递增避免覆盖。
2. **generate 自动并入 closing/**：生成账本时自动扫描 `<output>/{year}/closing/*.md` 并入凭证解析，并提示"已并入 N 张自动结转凭证"。
3. **`本年收益` 加入科目类别**（权益类），结转目标科目可识别。
4. **year-close 提示**：损益未结转告警处补充"可用 ledger gen-close 生成结转凭证"。

## Capabilities

### New Capabilities
- `auto-close-voucher`: 年末损益结转凭证自动生成（closing/ 目录、幂等跳过、generate 自动并入）

## Impact

- `cmd/gen_close.go`（新）、`cmd/generate.go`（并入逻辑）、`balance/balance.go`（科目类别）、`cmd/year_close.go`（提示）
- 测试：gen-close 生成文件、generate 并入后损益归零、幂等（不重复生成）
- 不变：手工凭证目录零写入
