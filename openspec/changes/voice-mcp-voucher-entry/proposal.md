# 语音/MCP 记账录入

## Why

用户要通过与 agent 交流来记账（移动端语音 → LLM → 落凭证），取代"给科目编码以便人工快速录凭证"的做法。系统当前**只能读凭证、不能写凭证**（`cmd/` 无任何创建凭证 md 的命令），因此这条链路缺的不是 AI 能力，而是**一个受硬闸门保护的凭证写入入口 + 一个可远程调用的接入层**。

同时，`skill + CLI` 的形态假设 agent 就在账套所在机器上、能读文档敲命令；语音/移动端 agent 摸不到用户磁盘，该形态在这一场景下失效。

## What Changes

1. **新增 `ledger voucher` 子命令组**：`voucher add`（创建凭证 md）、`voucher list`（当月清单）、`voucher check`（序列完整性与目录卫生）。
   - `voucher add` 落盘前强制：借贷平衡净额校验、科目必须在科目树已定义（复用"先定义后生成"同一套检查）、月目录白名单、凭证号分配。
   - `voucher add` **不触发 generate**——账本生成必须是用户显式动作。
   - `--idempotency-key` 支持重试幂等（索引存月目录外，避免污染凭证目录）。
2. **新增凭证序列完整性检查**：当月凭证号连续、无重、无断码、张数自洽；**默认告警、`--strict` 阻断**（避免让存在作废空号/带后缀文件名的真实历史账套无法 generate）。
3. **新增月目录白名单检查**：月目录内**只准** `记字第XXXX号.md`；其余 `.md` 会因 `CollectEntries` 递归（`cmd/common.go:34`）被静默记账。默认告警、`--strict` 阻断。
4. **新增 `ledger mcp serve`**：MCP Streamable HTTP 服务，**局域网优先**。
   - 工具面白名单：`subjects.list` / `subjects.match`（只读）、`voucher.add`（写）、`voucher.list` / `voucher.check`（只读）、`ledger.check` / `ledger.query`（只读）。
   - **不暴露** generate / lock / gen-close / year-close / 任何 `-f` / 任何路径参数。
   - 实例绑定单一账套根目录；所有工具不接受路径入参。
   - **fail-closed**：未配 token 时禁止绑定非本机地址。
5. **科目名归一化与候选提示**（落实 `docs/account-code-design.md` §四"语音立项工作项"）：全角半角、内部空格归一化 + 编辑距离/包含候选；**只提示候选、不自动替换**。同音字（拼音）候选列为首版 Non-Goal。

## Capabilities

### New Capabilities

- `voucher-authoring`: 从结构化输入创建凭证 md（含平衡校验、科目定义校验、凭证号分配、重试幂等、round-trip 自检）
- `voucher-sequence-integrity`: 凭证序列完整性（连续/无重/无断码/张数）与月目录卫生（只准正式凭证 md），默认告警 `--strict` 阻断
- `mcp-server`: MCP Streamable HTTP 接入层（工具面白名单、账套绑定与路径隔离、局域网绑定、fail-closed token、科目名归一化与候选提示）

### Modified Capabilities

<!-- 无：本变更为纯新增命令与新增子命令，不改动已有 spec 的任何需求；generate/解析/月结/打印零改动 -->

## Impact

- **新增代码**：`cmd/voucher.go`（子命令组）、`cmd/voucher_add.go`、`cmd/voucher_check.go`、`cmd/mcp.go`、`cmd/mcp_serve.go`；`voucher/writer.go`（md 生成器）、`voucher/sequence.go`（序列与白名单检查）、`voucher/subject_match.go`（归一化与候选）；`mcp/`（server + 工具绑定）。
- **复用而非改动的现有逻辑**：`voucher.ValidateVoucherBalance`（平衡）、科目树定义检查（`subjects` 路径）、`voucher.ParseFile`（round-trip 自检）、`cmd/check.go`（`ledger.check` 工具）。
- **不改动**：`generator/` 全部、`voucher/parser.go` 解析逻辑、`balance/` JSON schema、三条生成路径、打印版。
- **依赖**：优先手写 Streamable HTTP JSON-RPC（零新直接依赖）；若采用官方 MCP Go SDK 则为新增直接依赖，实施时评估。
- **文档**：`.agents/skills/ledger-accounting/` ↔ `embedded/ledger-accounting/` 双目录逐字同步（`embed_test.go` 守护）；新增 `references/mcp.md`；SKILL.md 命令总览补三个命令；`CHANGELOG.md` 人话节（发版纪律）。
- **兼容**：纯新增；不改 JSON schema、不改现有生成逻辑，老账套零影响；回滚只需回退二进制。
- **不改动的红线**（明示）：铁律一（历史文件永不修改）、铁律二（余额链连续）、铁律三（JSON 为余额唯一权威源）。
