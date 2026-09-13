# 设计：语音/MCP 记账录入

## D1 命令形态与入参

新增子命令组 `cmd/voucher.go`（cobra `voucher`），下挂 `add` / `list` / `check`；MCP 为独立命令 `cmd/mcp.go` → `mcp serve`。

`voucher add` 入参形态（spec: 凭证创建命令）：

```
ledger voucher add --date 2026-03-05 --summary "付养老金" \
  --debit "公益支出-补助费用=9990.00" --credit "应付款-养老金=9990.00" \
  [--attachment N] [--num 5] [--idempotency-key K] [--json -]
```

- 重复 flag 用 cobra `StringArray`；`--json` 读文件或 stdin。二者**互斥**（同时给出报错）。
- 分录结构（JSON 形态）：`{summary?, account, side: "debit"|"credit", amount}` —— 金额用**字符串**接收（避免浮点），内部走既有 `int64` 分转换。
- `account` 即科目全路径（`总账科目` 或 `总账科目-明细科目`），与 `map`/科目树同一 key 空间。
- 行级 `summary` 可选，缺省回落凭证级摘要（原凭证格式每行都有摘要，故写盘时逐行填充）。

否决方案：只提供 `--json`（人手敲不便）；只提供重复 flag（agent 需处理 shell 转义，脆弱）。两者并存成本极低。

## D2 凭证 md 生成器

新文件 `voucher/writer.go`。格式基线 = `test/e2e/test_data/2026_04/记字第0022号.md`：

```
<单位名>

记字第XXXX号 1/1

记帐凭证

YYYY年MM月DD日

附件 N 张

<table><thead><tr><th>摘要</th><th>总帐科目</th><th>明细科目</th><th>借方</th><th>贷方</th></tr></thead><tbody>…省略空行填充…<tr><td>合计</td>…</tbody></table>

会计主管 
记帐 
审核 
制单 
```

- 单位名/签章人：首版从账套配置读取，缺省留空（打印后手签）。
- 表格填满固定行数（对齐基线样例的空行填充行为），保证打印形态一致。
- 科目列拆分：按 `splitPath` 同规则拆 `总账科目` / `明细科目` 两列。
- **round-trip 自检**（spec: 写入器 round-trip 自检）：写盘后立即 `voucher.ParseFile`，逐项断言日期/凭证号/科目/方向/金额；失败则删除该文件并非零退出。选择理由：写入器的正确性由**已成为事实标准的解析器**裁定，不靠人眼比对格式；这也是唯一能在 CI 里挡住格式漂移的机制。

## D3 凭证号分配与幂等索引

- 分配：`filepath.Glob(月目录/*.md)` → `voucherNumRe` 提取 → 取最大 +1（对齐既有非递归口径；正式命名由白名单保证）。
- 显式 `--num` 冲突 → 拒绝（spec 已规定）。
- 幂等索引文件：`vouchers/.voucher-keys/YYYY_MM.json`，内容 `{key: {num, path, createdAt}}`。
  - **为什么放在月目录外**：`CollectEntries`/`ParseDir` 是 `WalkDir` 递归；若有 `.md` 落进月目录会被静默记账（`.json` 扩展名虽被 `filepath.Ext(path) != ".md"` 过滤掉，但把索引放月目录外是双保险，也保持"月目录内只有正式凭证"这条不变量可被人眼与白名单直接验证）。
  - 幂等判定：key 命中 → 直接返回既有结果；文件已被人工删除 → 视为失效并重新创建（避免索引成为垃圾锁）。
- 并发：`voucher add` 内部对"读最大号 → 写文件"加进程内互斥；MCP 层再包一层写串行（D6）。

## D4 序列/白名单检查的口径

新文件 `voucher/sequence.go`，纯读。

- 白名单正则：`^记字第\d+号\.md$`（**比 `voucherNumRe` 更严**——后者刻意宽松，允许 `记字第0005号 更正.md`，那是历史兼容用的）。
- 三类问题：缺号（1..max 中缺失）、重号（同号多文件）、非正式文件名（不匹配白名单的 `.md`）。
- **默认告警、`--strict` 阻断**（spec 已规定）。理由是一致性：`generate` 的既有 `-v` 目录若存在历史遗留非规范文件名，新检查若默认阻断就等于**让老账套无法生成**，直接违反铁律一"历史文件永不修改"的立法意图。
- `voucher add` 的前置检查复用同一函数，默认告警（打印问题清单但继续），使 agent 能在复述中把问题带给用户。
- 结构化输出：`--json` 输出问题数组（类别/文件名/凭证号），供 MCP `voucher.check` 直接转成工具返回值。

## D5 MCP 传输与实现选型

- 传输：**Streamable HTTP**（`/mcp` 端点）。否决 stdio：手机/跨设备 agent 无法连本机进程；否决 SSE-only：MCP 现行规范以 Streamable HTTP 为准。
- 实现选型（实施时定，倾向后者）：
  - (a) 官方 MCP Go SDK → 新增直接依赖；协议跟进由上游保证。
  - (b) 手写 Streamable HTTP + JSON-RPC（`initialize` / `tools/list` / `tools/call` 三个方法已够用）→ **零新直接依赖**，符合本仓库依赖洁癖（`go.mod` 现仅 cobra + excelize + x/text 等 indirect）。
- 依赖无阻塞：`golang.org/x/crypto`、`x/net` 已在依赖树内，HTTP 与随机 token 生成不构成新负担。

## D6 工具绑定与路径隔离

`mcp/` 包只做三件事：协议编解码、参数校验、调用 `cmd/` 已有逻辑。**硬规则一律不进 MCP 层**——借贷平衡、科目定义、幂等、余额链全部由 CLI 侧函数承担（proposal 影响节已明示），否则会出现两套规则。

- 工具入参 schema **不含任何路径/目录/文件名参数**（spec 已规定）；月份以 `YYYY-MM` 字符串传入，服务端自行拼路径。
- 越界防护：科目名与摘要等自由文本在使用前校验不含路径分隔与 `..`，且**只**用于拼接进凭证正文，不参与路径构造。
- 写操作串行：`voucher.add` 走同一把互斥锁（跨所有会话），保证 D3 的"读最大号 → 写文件"原子。
- 工具面白名单硬编码在代码里（不是配置），新增工具必须改代码 + 过 review。

## D7 token 与 fail-closed

- token 来源优先级：`--token` flag > 环境变量 `LEDGER_MCP_TOKEN` > 账套配置文件。
- 未配置 token 时：允许绑定回环（本机 agent 场景），**拒绝绑定非回环**（spec 已规定）。
- token 自动生成：需要绑定局域网但未提供时，**不自动生成后静默放行**，而是报错并提示如何生成（`ledger mcp token` 或 `--token`），避免出现"用户以为有鉴权、实际是默认值"的隐性风险。
- 接入便利（降低"在一个局域网却接不上"的摩擦）：启动时打印局域网 URL + token，并可选打印二维码；后续可加 mDNS 发现（本期不做，列入 Open Questions）。

## D8 科目匹配算法与范围裁剪

新文件 `voucher/subject_match.go`。

1. 归一化：`TrimSpace`（既有）→ 移除内部空白 → 全角转半角。
2. 候选顺序：归一化精确 > 前缀 > 包含 > 编辑距离（阈值内）。
3. **只返回候选，绝不自动替换**（用户与设计稿 §四一致；自动替换会把"合法的错"变成"看起来对的错"）。
4. 同音字（拼音）不在首版：需要拼音表或第三方库。诚实裁剪而非半成品——`subjects.match` 的返回结构预留 `matchKind` 字段，后续加拼音候选时无需改协议。

## D9 与既有硬规则的关系

| 既有机制 | 本变更的处理 |
|---|---|
| 借贷平衡校验 `voucher.ValidateVoucherBalance` | **复用**，add 落盘前调用 |
| 先定义后生成 | **复用同一套科目定义检查**；add 不提供 `--allow-new` 逃生 |
| 凭证号未解析阻断（`voucher/balance_check.go:48`） | add 分配合法号，天然满足 |
| 重号检测（`cmd/generate.go:84`，非递归、只告警） | **不改**；新增白名单检查补其盲区（非凭证 .md 静默） |
| 幂等保护（已生成月份无 `-f` 拒绝重跑） | **不改**；A→B 边界的既有兜底 |
| `lock` | **不改**；MCP 不暴露 |
| 铁律一/二/三 | **不改**；本变更为纯新增，老账套零影响 |
| `generator/` 全部、`voucher/parser.go` 解析逻辑、`balance/` JSON schema | **零改动** |

## D10 验证设计

- **单测**（`voucher/`、`cmd/`）：
  - writer round-trip：多种分录组合（单借多贷、含红字、含空行填充）→ 解析回等价；
  - 号分配：空目录 / 已有 1..4 / 显式冲突 / 含非正式文件名时的行为；
  - 幂等：同 key 两次 → 一个文件；索引存在但文件被删 → 重建；
  - 平衡与科目定义：不平衡、未定义科目、红字净额各拒绝且**零写盘**；
  - 序列检查：连续 / 缺号 / 重号 / 白名单违规四类 + 默认告警 vs `--strict` 退出码；
  - subject match：归一化（内部空格、全角）、错字候选、空候选；
  - MCP：未配 token 绑非回环 → 拒绝启动；token 错误 → 不执行读写；工具清单恰为七个；schema 无路径参数；并发 add 不发号冲突。
- **e2e**（`scripts/test-e2e.sh` 扩展或新增脚本）：`init` → 同月连续 `voucher add` 多张 → `generate` 成功且余额链连续 → 与手写凭证账套结果一致。
- **兼容回归**：对既有 e2e 账套（含 `记字第0005号 更正.md` 类文件名）跑检查，确认**默认告警不阻断**、`generate` 行为逐字节不变。
- **全量**：`go test ./... -count=1` + `bash scripts/test-e2e.sh`；`embed_test.go` 双目录守护通过；`openspec validate --all` 中本变更 `valid: true`。

## Migration Plan

1. 纯新增命令与新增子命令，不新增 JSON 字段、不改 JSON schema → **无需账套迁移**。
2. 老账套：`generate` 路径零改动；新检查默认告警，历史遗留文件名与断码不会阻断既有工作流。
3. 技能同步：`.agents/skills/ledger-accounting/` ↔ `embedded/ledger-accounting/` 逐字同步（`embed_test.go` 守护）；`SKILL.md` 命令总览补三个命令；新增 `references/mcp.md`（MCP 客户端配置 + 局域网接入 + token）。
4. `VERSION` 联动：CLI 版本变更后须重跑 `ledger install-skill`（技能与版本一一对应，`ledger doctor` 校验）。
5. 回滚：无 schema 变更、无数据写入格式变更 → 直接回退二进制即可；已由 `voucher add` 创建的凭证 md 是合法凭证，无需清理。
6. 发版：按 `CHANGELOG.md` 文末六步；`CHANGELOG.md` 增人话节；push/tag 前需用户显式批准。

## Open Questions

1. **MCP 实现选型**：官方 Go SDK（新直接依赖）vs 手写 Streamable HTTP JSON-RPC（零依赖）。倾向手写；实施首日定。
2. **mDNS/Bonjour 自动发现**：让"同一局域网即可接入"免去抄 IP（DHCP 下 IP 会变）。本期不做，接口预留。
3. **单位名与签章人来源**：首版从账套配置读取、缺省留空。是否需要 `voucher add --operator` 或配置字段，待用户确认打印流程。
4. **`voucher add` 的前置检查在发现非正式文件名时**：默认告警继续写盘 vs 直接拒绝。当前取前者（一致性优先），若实践发现静默记账风险高可改 `--strict-add`。
5. **同音字候选**：需要拼音表/库；本期 Non-Goal，`matchKind` 字段已预留。
6. **局域网现实障碍**（非代码）：访客 Wi-Fi/AP 隔离导致同 SSID 设备互不可见、macOS 防火墙首次拦入站。需写入 `references/mcp.md` 的排障节。
