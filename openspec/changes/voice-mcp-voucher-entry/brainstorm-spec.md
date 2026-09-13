# 语音/MCP 记账录入（brainstorm-spec）

> 本文件产生于 2026-09-13 与用户的逐轮对话（用户对每一节都做过确认与纠正），单源承载设计阶段结论。过程记录见 `.workbuddy/memory/2026-09-13.md`。

## Context

用户构想：**MCP 连接器 + LLM agent + 语音/移动端**，让用户通过与 agent 交流来记账，取代"给科目编码以简化手工录凭证"的做法。用户在移动端用自带助手（小艺等）或通用 agent 说话，LLM 拿到字符串后落成凭证。

系统现状（已核实的代码事实）：

- **CLI 只有读凭证，没有任何创建凭证 md 的命令**。`cmd/` 全量为 init / add-manual / map / subjects / subjects-scan / opening / generate / check / lock / gen-close / year-close / doctor / config / reset / install-skill——凭证目前只由人或 OCR 产生。
- **硬闸门已就位**：借贷平衡净额校验（不平衡拒绝生成）、先定义后生成（未定义科目拒绝 + `--allow-new` 逃生）、幂等保护（已生成月份无 `-f` 拒绝重跑）、`lock` 结账月保护、凭证号未解析硬阻断（`voucher/balance_check.go:48`）、期初双源冲突采信 JSON 并告警。
- **两条**扫凭证目录的路径都是 `filepath.WalkDir`（**递归**）：`cmd/common.go:32 CollectEntries`、`voucher/parser.go:286 ParseDir`；`CollectEntries` 按 `(Date, VoucherNum)` 排序。
- **重号检测是另一套口径**：`cmd/generate.go:84` 用 `filepath.Glob(voucherDir/*.md)`（**非递归、只看文件名**），正则 `记字第\D*0*([0-9]{1,6})`（`cmd/generate.go:386`），且**只告警不阻断**。
- **断码/连续性检查完全不存在**：`grep 断码|连续性|缺号|gap` 在 `cmd/`、`voucher/` 零命中。
- 由此产生的不对称后果：`记字第0026号 副本.md` 会被递归解析记账且仅得一条重号告警；`草稿.md` / `模板.md` / `2026-03-05.md` 会被**静默**记账（文件名不含"记字第"，连告警都没有）。
- **技能形态的前提不成立**：现有 `skill + CLI` 假设 agent 就在账套那台机器上、能读文档、能敲命令；语音/移动端 agent 摸不到用户磁盘。
- 依赖现状：`go.mod` = cobra + excelize + `x/crypto`、`x/net`、`x/text`（均 indirect）。

关联既有结论：`docs/account-code-design.md` §四已把"语音输入立项工作项"挂触发器（总账列归一化、模糊匹配与同音字候选提示、复述确认规范）；`openspec/project.md` 的核心数据流把系统定位为「凭证 md → 解析器 → Excel」的**生成器**，输入侧假定凭证由外部产生。

## Goals / Non-Goals

**Goals**

1. 给系统补上**唯一的代码缺口**：创建凭证 md 的能力（`ledger voucher add`）。
2. 让"记账"这件事在**未生成账本的当月**可以零成本重做（改/删/重说），同时让"账已生效"成为一个**用户显式动作**。
3. 提供 MCP 接入层，使**同一局域网内**的手机/电脑 agent 能直接记账，且账本数据不出局域网。
4. 把凭证序列完整性（连续/无重/无断码）与目录卫生（只准正式凭证 md）做成**机器检查**，异常反馈 agent → 用户决策。
5. 落实设计稿 §四的"语音立项工作项"：科目名归一化 + 候选提示 + 复述确认规范。

**Non-Goals**

- **不做语音识别**（ASR）。语音→文本由通用 agent / 手机输入法/助手提供，本项目只接收文本。
- **不做外网中继 / 云端部署**。局域网优先；中继留接口不做实现。
- **首版不做同音字拼音库**。首版只做归一化（全角半角、内部空格）+ 编辑距离/包含候选；同音字候选需拼音表，列为后续变更。
- **不做科目编码主键化**。仍挂 `docs/account-code-design.md` §四的 5 个触发器。
- **不暴露、也不自动化 generate**。账本生成必须是用户显式动作。
- 不做多账套单实例（一实例一账套）。

## Decisions

### D1 方案甲：md 即草稿，不建草稿区（用户拍板）

凭证 md 及其 git commit **本身就是草稿态**。录入本月 5 号凭证不影响前 4 号；改错 = 改/删 md 再重生成；"是否影响了别的"由 `git log` 回答。不引入草稿区、不引入 draft/commit 两个概念。

否决方案：草稿区 + `voucher.commit`。被否理由：①多一层概念；②草稿目录若放在月目录内会被 `WalkDir` 静默吃进账本；③用户明确不要认知核算成本。

### D2 层次：git 只管输入，账本层不入 git（用户纠正）

git 管凭证 md + `{year}.json`；**账本层不入 git，也不该入**——账本正确性由 CLI 代码保证（铁律三：JSON 是余额唯一权威源、xlsx 可幂等重建）。限定词：生成读上月 xlsx 当期初，故严格说输入是"凭证 + JSON + 上月账本"；但无上月可纯 JSON 重建、双源冲突采信 JSON，**权威链始终在 JSON**。

### D3 A→B 边界 = "generate 跑过了"（用户拍板）

- 边界不是"文件落盘"，是"generate 跑过了"。
- **用户显式要求才 generate**；不允许 agent 私自生成账本。
- 已生成月份的凭证默认**只冲销**；`-f` 改凭证需再三确认（不可怕：git 有历史，随时可回退重决冲销 vs `-f`）。

### D4 接入选型 = MCP（用户判断，认同）

`skill + CLI` 的前提是 agent 在账套那台机器上；语音/移动端 agent 摸不到磁盘，手册模式失效。MCP 把能力变成可远程调用的语义化工具。**CLI 不消失**，它是 MCP server 的实现底座；所有硬规则留在 CLI，一处都不得上移到 MCP 层。

### D5 时序 = 机器检查，不是事前约束（用户修正）

不要求人按时序录入。脚本扫"每月连续、无重复、无断码"，不符合就反馈 agent → agent 告知用户 → 用户决策。git 粒度只需够用户决策后由 AI 执行。

### D6 传输 = 局域网优先（用户拍板）

"只要在一个局域网，我就能接上这个 MCP。"这正是"数据不出本地"（`.workbuddy/memory/2026-08-28.md` 定的 ugz-dzh 商业前提）最干净的实现——数据连局域网都不出。外网中继延后：同一 server 加传输层即可，不重构。

### D7 工具面窄 = 能力边界

MCP tool schema 即能力边界。只暴露：`subjects.list` / `subjects.match`（只读）、`voucher.add`（写）、`voucher.list` / `voucher.check`（只读）、`ledger.check` / `ledger.query`（只读）。**不暴露** generate / lock / gen-close / year-close / 任何 `-f` / 任何路径参数。

### D8 确认的对象是文件，不是 agent 的转述

agent 必须**先落盘成 md**，再把**文件内容**念回给用户确认；不得凭记忆在对话里复述后再写文件。否则"说的"与"写的"可能不一致，幻觉正藏于此。

### D9 校验默认告警、`--strict` 才阻断（兼容铁律一）

断码与目录白名单若默认阻断，会让**真实历史账套**（存在作废留空号、或文件名带后缀如 `记字第0005号 更正.md`）无法 generate。故默认告警列出问题清单，`--strict` 供主动校验/工具面使用。铁律一"历史文件永不修改"不因新检查而破。

### D10 首版范围裁剪

同音字候选需拼音表（新依赖），首版不做，只做归一化 + 编辑距离/包含候选；写入器正确性由 **round-trip 自检**（写完立即 `voucher.ParseFile` 解析回来断言与入参一致）保证，不靠人眼。

## Risks / Trade-offs

- [语音金额听错且借贷两边等额错，平衡校验查不出] → `voucher.add` 返回结构化摘要（含金额与借贷合计）供 agent 复述；`voucher.list` 供复核；用户显式 generate 前可再核。**复述确认的重点是金额**，不是整句。
- [agent 工具调用重试导致重复凭证] → `voucher.add` 支持 `--idempotency-key`，幂等索引存月目录**外**（`vouchers/.voucher-keys/YYYY_MM.json`）；现有重号检测只告警不阻断，不足以兜底。
- [严格白名单/断码阻断历史账套] → D9：默认告警、`--strict` 才阻断。
- [月目录内非凭证 .md 被静默记账] → 白名单检查 + `voucher add` 写入前置检查。
- [MCP 暴露 generate 导致 AI 擅自推进 A→B] → D7 不暴露（唯一的结构性保证；靠提示词约束是软的）。
- [局域网内其它设备可读取账本] → fail-closed：未配 token 时禁止绑定非本机地址。
- [写入器与解析器格式漂移] → round-trip 自检 + e2e；格式以 `test/e2e/test_data/2026_04/记字第0022号.md` 为准。
- [生成是月级：当月新增凭证会让当月已有分录的余额链/月结/页码整体重算] → 这是余额链是全月 fold 的必然结果，非缺陷；正确性由铁律三（JSON 权威）与历史月只读保证；`voucher.add` **不触发** generate。
- [引入 MCP SDK 与仓库依赖洁癖冲突] → 实施时在"官方 Go SDK（新直接依赖）"与"手写 Streamable HTTP JSON-RPC（零新依赖）"间评估，倾向后者。
