# 任务：语音/MCP 记账录入

## 1. 凭证写入器（`voucher/writer.go`）

- [ ] 1.1 定义入参结构（日期/摘要/附件数/凭证号/分录数组），金额用字符串接收并转 `int64` 分（复用既有转换，禁止浮点）。
- [ ] 1.2 按基线格式（`test/e2e/test_data/2026_04/记字第0022号.md`）生成 md 正文：标题块、凭证号行、日期、附件、HTML 表格（`总账科目`/`明细科目` 两列由 `splitPath` 同规则拆分）、合计行、签章行、表格空行填充。
- [ ] 1.3 写盘后 round-trip 自检：`voucher.ParseFile` 解析并逐项断言日期/凭证号/科目/方向/金额；失败删文件 + 非零退出。
- [ ] 1.4 单测：单借多贷、含红字、空行填充、多行摘要回落，均 round-trip 等价。

## 2. `ledger voucher add`（`cmd/voucher.go` + `cmd/voucher_add.go`）

- [ ] 2.1 子命令组骨架 `cmd/voucher.go`；`add` 的 flag：`--date`（必填）、`--summary`（必填）、`--debit`/`--credit`（StringArray）、`--json`、`--attachment`、`--num`、`--idempotency-key`。
- [ ] 2.2 入参校验：必填项、重复 flag 与 `--json` 互斥、日期可解析、金额可解析且为正。
- [ ] 2.3 落盘前借贷平衡校验（复用 `voucher.ValidateVoucherBalance`，净额口径）；不平衡拒绝且零写盘。
- [ ] 2.4 落盘前科目定义校验（复用"先定义后生成"同一套检查）；未定义科目输出清单并拒绝，**不提供 `--allow-new`**。
- [ ] 2.5 凭证号分配：扫月目录取最大号 +1；`--num` 冲突拒绝。
- [ ] 2.6 幂等索引 `vouchers/.voucher-keys/YYYY_MM.json`：key 命中返回既有结果；索引存在但文件已被删则重建。
- [ ] 2.7 写前置检查：调用序列/白名单检查（默认告警，打印问题清单后继续）。
- [ ] 2.8 确认 `add` 不触发 generate、不写 `{year}.json`、不动 xlsx/print。
- [ ] 2.9 输出凭证号、路径、借贷合计。

## 3. 序列完整性与目录卫生检查（`voucher/sequence.go`）

- [ ] 3.1 白名单正则 `^记字第\d+号\.md$`（严于 `voucherNumRe`）与三类问题：缺号 / 重号 / 非正式文件名。
- [ ] 3.2 默认告警零退出、`--strict` 非零退出；`--json` 结构化输出（类别/文件名/凭证号）。
- [ ] 3.3 单测：连续 / 缺号 / 重号 / 白名单违规四类 + 两种模式的退出码。
- [ ] 3.4 兼容回归：对含 `记字第0005号 更正.md` 类文件名的既有账套，确认默认模式不阻断。

## 4. `voucher list` / `voucher check`（`cmd/voucher_check.go`）

- [ ] 4.1 `voucher list`：当月凭证号、日期、摘要、借贷合计。
- [ ] 4.2 `voucher check`：包装 3.x 检查，输出可直接供 agent 复述与请求决策。
- [ ] 4.3 两者均为只读，单测断言账套文件内容不变。

## 5. 科目名归一化与候选（`voucher/subject_match.go`）

- [ ] 5.1 归一化：TrimSpace + 移除内部空白 + 全角转半角。
- [ ] 5.2 候选顺序：归一化精确 > 前缀 > 包含 > 编辑距离（阈值内）；返回 `matchKind` 字段（为后续同音字预留）。
- [ ] 5.3 **只返回候选，禁止自动替换或自动登记**。
- [ ] 5.4 单测：内部空格、全角、错字（`管埋费用` → `管理费用`）、空候选。

## 6. MCP 服务（`cmd/mcp.go`、`mcp/`）

- [ ] 6.1 实现选型决策：官方 Go SDK vs 手写 Streamable HTTP JSON-RPC；记录在 design.md 的 Open Questions 收口。
- [ ] 6.2 `ledger mcp serve`：`-o` 绑定单一账套根目录，非法账套拒绝启动。
- [ ] 6.3 传输：Streamable HTTP，`/mcp` 端点；实现 `initialize` / `tools/list` / `tools/call`。
- [ ] 6.4 token：`--token` > `LEDGER_MCP_TOKEN` > 配置文件；**未配 token 时拒绝绑定非回环地址**并给出生成指引；默认只绑回环。另提供 `ledger mcp token`：生成随机 token 并写入账套配置，使"一条命令 + 一个局域网"即可接入。
- [ ] 6.5 启动打印接入信息：账套路径、MCP 端点 URL、token（+ 可选二维码）。
- [ ] 6.6 工具面白名单硬编码：`subjects.list`、`subjects.match`、`voucher.add`、`voucher.list`、`voucher.check`、`ledger.check`、`ledger.query`；**不暴露** generate/lock/gen-close/year-close/subjects import/opening import/add-manual/map/任何 `-f`/任何 JSON 写。
- [ ] 6.7 路径隔离：所有工具 schema 无路径/目录/文件名参数；月份以 `YYYY-MM` 传入、服务端拼路径；自由文本校验不含路径分隔与 `..`。
- [ ] 6.8 写操作串行：`voucher.add` 跨会话互斥锁，保证"读最大号 → 写文件"原子。
- [ ] 6.9 硬规则不上移：MCP 层只做协议编解码 + 参数校验 + 调用 CLI 侧函数。
- [ ] 6.10 单测：未配 token 绑非回环拒绝；token 错误不执行读写；工具清单恰为七个；schema 无路径参数；并发 add 不发号冲突。

## 7. 测试

- [ ] 7.1 `go test ./... -count=1` 全绿（含新增 `voucher/`、`cmd/`、`mcp/` 单测）。
- [ ] 7.2 e2e：`init` → 同月连续 `voucher add` 多张 → `generate` 成功且余额链连续；与手写凭证账套结果一致。
- [ ] 7.3 兼容回归：既有 e2e 账套 `generate` 行为逐字节不变（本变更零改动 generator/parser）。
- [ ] 7.4 `bash scripts/test-e2e.sh` 通过。

## 8. 文档与技能

- [ ] 8.1 `.agents/skills/ledger-accounting/` ↔ `embedded/ledger-accounting/` 逐字同步；`SKILL.md` 命令总览补 `voucher add/list/check` 与 `mcp serve`。
- [ ] 8.2 新增 `references/mcp.md`：MCP 客户端配置、局域网接入、token 配置、排障（AP 隔离 / macOS 防火墙 / DHCP 换 IP）。
- [ ] 8.3 技能内写明 A→B 边界纪律：record 后由**用户显式**要求才 generate；已生成月份默认只冲销。
- [ ] 8.4 `embed_test.go` 双目录守护通过。

## 9. 验证与发版准备

- [ ] 9.1 `openspec validate voice-mcp-voucher-entry --json` → `valid: true`。
- [ ] 9.2 三重复核（对照本变更 specs/design 与实现，红队对抗实验）。
- [ ] 9.3 `CHANGELOG.md` 增人话节（🚀新增 / ⚠行为变更 / 📦对账套影响）。
- [ ] 9.4 `ledger doctor` 通过；`VERSION` 联动确认。
- [ ] 9.5 发版六步执行（push/tag 前须用户显式批准）。
