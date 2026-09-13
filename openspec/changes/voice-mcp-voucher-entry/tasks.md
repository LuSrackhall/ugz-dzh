# 任务：语音/MCP 记账录入

> 本轮范围：第 1–4 组（voucher 命令链路）+ 相关的第 7/8 组条目。第 5–6 组（科目候选、MCP 服务）未开工。

## 1. 凭证写入器（`voucher/writer.go`）

- [x] 1.1 定义入参结构（日期/摘要/附件数/凭证号/分录数组），金额用字符串接收并转 `int64` 分（复用既有转换，禁止浮点）。`WriteRequest`/`WriteEntry`/`Signers`；JSON 侧 `jsonAmount` 同时接受字符串与数字字面量（保留原文，避免 float64 精度损失）。
- [x] 1.2 按基线格式（`test/e2e/test_data/2026_04/记字第0022号.md`）生成 md 正文：标题块、凭证号行、日期、附件、HTML 表格（总账/明细两列由首个 `-` 拆分）、合计行、签章行。**订正：不做表格空行填充**——抽样既有凭证填充数为 0–3 不等，属 OCR 原件产物而非约定（design.md D2 已同步订正）。
- [x] 1.3 写盘后 round-trip 自检：`voucher.ParseFile` 解析并逐项断言日期/凭证号/科目/方向/金额/摘要；失败删文件 + 非零退出。
- [x] 1.4 单测：单借多贷、含红字、摘要回落、明细空白归一、分厘精度、零填充行数、篡改检测、自检失败删文件。
- [x] 1.5 **新增防护（往返自检发现）**：解析器会把"任意单元格含 `摘要/总帐科目/总账科目/明细科目/借方/贷方`"的行当表头丢弃、把摘要为 `合计` 的行当合计行丢弃（均不报错）——写入器提前拦截并给出改写建议。

## 2. `ledger voucher add`（`cmd/voucher.go` + `cmd/voucher_add.go`）

- [x] 2.1 子命令组骨架 `cmd/voucher.go`；`add` 的 flag：`--date`、`--summary`、`--debit`/`--credit`（StringArray）、`--json`、`--attachment`、`--num`、`--idempotency-key`、`--unit`、`-o`。
- [x] 2.2 入参校验：必填项、重复 flag 与 `--json` 互斥、日期可解析、金额可解析；`side` 接受 `debit/credit` 与 `借/贷/借方/贷方`。
- [x] 2.3 落盘前借贷平衡校验（复用 `voucher.ValidateVoucherBalance`，净额口径）；不平衡拒绝且零写盘。
- [x] 2.4 落盘前科目定义校验（复用 `undefinedVoucherSubjects`，与 generate 同一套）；未定义科目输出清单并拒绝，**不提供 `--allow-new`**。
- [x] 2.5 凭证号分配：扫月目录取最大号 +1（`ParseVoucherFileNum` 走白名单）；`--num` 冲突拒绝。
- [x] 2.6 幂等索引 `vouchers/.voucher-keys/YYYY_MM.json`：key 命中且文件仍在 → 返回既有结果；文件被人工删除 → 索引失效并重建。
- [x] 2.7 写前置检查：调用序列/白名单检查（默认告警，打印问题清单后继续）；已结账月份提示需 `-f` 级联重建。
- [x] 2.8 确认 `add` 不触发 generate、不写 `{year}.json`、不动 xlsx/print（单测 `TestVoucherAddDoesNotGenerate` 断言）。
- [x] 2.9 输出凭证号、路径、借贷合计，并提示"账本未生成"。

## 3. 序列完整性与目录卫生检查（`voucher/sequence.go`）

- [x] 3.1 白名单正则 `^记字第\d+号\.md$`（严于 `voucherNumRe`）与三类问题：缺号 / 重号 / 非正式文件名（含"位于子目录"）。
- [x] 3.2 默认告警零退出、`--strict` 非零退出；`--json` 结构化输出（类别/文件名/凭证号）。
- [x] 3.3 单测：连续 / 空目录 / 缺号 / 重号 / 白名单违规 / 子目录 / 缺号明细截断 / 确定性排序 / 目录错误。
- [x] 3.4 兼容口径确认：`记字第0005号 更正.md` 类文件名在默认模式只告警不阻断（`TestVoucherAddWarnsButDoesNotBlockOnDirtyDir`、`TestVoucherCheckDefaultWarnsStrictBlocks`）。
- [x] 3.5 遍历方式与 `CollectEntries` 一致（`filepath.WalkDir` **递归**）——否则检查本身会带有它要堵的盲区。

## 4. `voucher list` / `voucher check`（`cmd/voucher_check.go`）

- [x] 4.1 `voucher list`：凭证号、日期、摘要、借贷合计、行数 + 合计行；`--json` 输出。
- [x] 4.2 `voucher check`：包装 3.x 检查，输出可直接供 agent 复述与请求决策。
- [x] 4.3 两者均为只读（只读 `os.ReadDir` + `ParseFile`），无写盘路径。

## 5. 科目名归一化与候选（`voucher/subject_match.go`）

- [ ] 5.1 归一化：TrimSpace + 移除内部空白 + 全角转半角。
- [ ] 5.2 候选顺序：归一化精确 > 前缀 > 包含 > 编辑距离（阈值内）；返回 `matchKind` 字段（为后续同音字预留）。
- [ ] 5.3 **只返回候选，禁止自动替换或自动登记**。
- [ ] 5.4 单测：内部空格、全角、错字（`管埋费用` → `管理费用`）、空候选。

## 6. MCP 服务（`cmd/mcp.go`、`mcp/`）

- [ ] 6.1 实现选型决策：官方 Go SDK vs 手写 Streamable HTTP JSON-RPC；记录在 design.md 的 Open Questions 收口。
- [ ] 6.2 `ledger mcp serve`：`-o` 绑定单一账套根目录，非法账套拒绝启动。
- [ ] 6.3 传输：Streamable HTTP，`/mcp` 端点；实现 `initialize` / `tools/list` / `tools/call`。
- [ ] 6.4 鉴权：token 来源 `--token` > `LEDGER_MCP_TOKEN` > 账套配置；**未配 token 时绑定非回环地址 → 自动生成随机 token + 写入账套配置 + 启动打印**（禁止任何固定默认值，已配置则不覆盖）；默认只绑回环（免 token）；无鉴权仅 `--insecure` 显式开启（默认关闭）。
- [ ] 6.5 启动打印接入信息：账套路径、MCP 端点 URL、token（+ 可选二维码）。
- [ ] 6.6 工具面白名单硬编码：`subjects.list`、`subjects.match`、`voucher.add`、`voucher.list`、`voucher.check`、`ledger.check`、`ledger.query`；**不暴露** generate/lock/gen-close/year-close/subjects import/opening import/add-manual/map/任何 `-f`/任何 JSON 写。
- [ ] 6.7 路径隔离：所有工具 schema 无路径/目录/文件名参数；月份以 `YYYY-MM` 传入、服务端拼路径；自由文本校验不含路径分隔与 `..`。
- [ ] 6.8 写操作串行：`voucher.add` 跨会话互斥锁，保证"读最大号 → 写文件"原子。
- [ ] 6.9 硬规则不上移：MCP 层只做协议编解码 + 参数校验 + 调用 CLI 侧函数。
- [ ] 6.10 单测：未配 token 绑非回环自动生成；token 错误不执行读写；工具清单恰为七个；schema 无路径参数；并发 add 不发号冲突。

## 7. 测试

- [x] 7.1 `go test ./... -count=1` 全绿（含新增 `voucher/` 21 项、`cmd/` 14 项单测）。
- [x] 7.2 端到端：`init` → 同月连续 `voucher add` 多张 → `generate` 成功 → `check` 校验 xlsx 期末与 JSON 余额链一致（`TestVoucherAddThenGenerate`）；真实二进制冒烟同样通过。
- [x] 7.3 兼容回归：既有 e2e 包与 `scripts/test-e2e.sh` 账套行为不变（本变更零改动 generator/parser/JSON schema）。
- [x] 7.4 `bash scripts/test-e2e.sh` 通过（`E2E_EXIT=0`，日志无 FAIL；含 balance/cmd/embedded/generator/layout/test-e2e/voucher 全包 ok）。

## 8. 文档与技能

- [x] 8.1 `.agents/skills/ledger-accounting/` ↔ `embedded/ledger-accounting/` 逐字同步；`SKILL.md` 命令总览补 `voucher add/list/check`（`mcp serve` 待第 6 组）。
- [ ] 8.2 新增 `references/mcp.md`：MCP 客户端配置、局域网接入、token 配置、排障（AP 隔离 / macOS 防火墙 / DHCP 换 IP）。
- [x] 8.3 技能内写明 A→B 边界纪律：录完由**用户显式**要求才 generate；已生成月份默认只冲销（SKILL.md「每月记账」节）。
- [x] 8.4 `embed_test.go` 双目录守护通过。
- [x] 8.5 `commands.md` 新增 `## voucher` 节，含三道闸门、两种入参形态、幂等索引位置、默认告警口径与"递归扫 .md"的静默伤害说明。
- [x] 8.6 `SKILL.md` 常见错误表新增两行（多余 `.md` 被静默记账、摘要含表头关键字被丢弃）。

## 9. 验证与发版准备

- [ ] 9.1 `openspec validate voice-mcp-voucher-entry --json` → `valid: true`（改完 spec 后复验）。
- [ ] 9.2 三重复核（对照本变更 specs/design 与实现，红队对抗实验）。
- [ ] 9.3 `CHANGELOG.md` 增人话节（🚀新增 / ⚠行为变更 / 📦对账套影响）。
- [x] 9.4 `ledger doctor` 通过（3 项正常 / 2 项警告 / 0 项失败；警告为仓库根非账套所必然——无 print-config.json 与年度 JSON）。`VERSION` 联动：doctor 报 "CLI dev = skill dev" 一致。
- [ ] 9.5 发版六步执行（push/tag 前须用户显式批准）。

## 10. 实施期顺手修复（连带发现）

- [x] 10.1 **测试夹具缺陷**：`resetFlagsInProcess` 对切片型 flag 用 `Set(DefValue)`，切片 flag 的 DefValue 是字面量 `"[]"`，会写入一个内容为 `"[]"` 的元素——`--debit/--credit` 是本仓库第一对切片 flag，由此暴露。改用 `pflag.SliceValue.Replace(nil)`。
- [x] 10.2 项目根二进制 `go build -o ledger .` 已更新（用户跑的是它）。
