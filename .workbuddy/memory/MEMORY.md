# 项目长期记忆（ugz-dzh 手工账电子化）

> 详细过程在 `.workbuddy/memory/` 当天日志（git 已跟踪，30 天以上按主题蒸馏后删除）；本文件只留跨会话仍需直接可用的结论与红线。

## 明细科目独立账页（2026-09-08 定稿，待发版）

- 账页模型（用户锁定）：**样式只有两种纸**——GL 三栏式（总分类账，默认分类/合并需配置）、ML 多栏式（明细账，默认合并/**分离需配置**）。明细账独立页 = **ML 家族分离形态**：页名 `明细账-{全路径}`、ML 样式多栏式、明细列自然为空；**不是** GL 引擎改名（首版返工教训）。
- 配置 `全局设置.明细账独立科目`（叶子全路径**精确匹配**——路径含连字符勿前缀匹配；默认空 = 不生效）。加法路由：GL 叶子页照常、命中明细从合并 ML 列排除（列收缩、余额口径不变）、JSON/报表/回写零改动——拆的是页不是账。
- 挂点：`appendToMLSheetBody` 共享本体（accountLabel 参数）；`WriteMLMonthClosings` targets 双轮（合并+独立，月结循环体零搬动）；setMLSheetPageLayout / transformMLSheet / FinalizeMLPages / 排序区块（GL→合并→ML→明细账）全按 `明细账-` 前缀扩展；存量收敛 = 孤儿页自动删 + ML 残留列报错 -f；合并父级进名单报错（对仗 D1a）。
- **ML BindingLeftCols=1**（非 GL 的 2；ml_layout.go 注释曾误写 2 已修正）——GetRows：摘要 idx5、余额 idx9、借方 idx6。ML 期初标签：isNew && initial≠0 →"期初余额"（无 GL 的 1 月"上年结转"区分）。excelize GetCellValue 数字格式返回不稳定，金额断言走数值解析。
- 教训：**改完必须 `go build -o ledger .` 更新项目根二进制**——用户跑的是它，测试内临时编译不更新交付物。

## 年末两段结转（v0.9.4）

- gen-close 默认两段齐发：①损益→本年收益；②本年收益→**收益分配-未分配收益**（净亏损方向相反；`--no-transfer` 跳过，重跑自愈）。postFinal = 本年收益 Final + Σ全部损益 Final（**不排除已结转**）三态同值，老账套跑一次自愈；防重 = closing/ 凭证含"收益分配"分录（hasTransfer）。结转目标自动登记（属性=贷）；资产负债表权益列零改动即正确（e2e 固化）。year_close 从不清零科目（清零 = gen-close 凭证；AGENTS.md 1.4 已更正）。

## 期初双源冲突（铁律三落地）

- 两源不一致**采信 JSON + 告警**，-f 重建自愈；仅单源行为不变。教训：干净账本复现不出的 bug 触发态多在"外部手修"——要构造的是**状态**（双源分歧），不是更多操作序列。

## 科目体系 v0.9.0

- 先定义后生成（`--allow-new` 逃生）；官方 42 科目表按"最终结构"定义，**运行时只消费"现行类别"**（AccountTypeOf 冻结现行五类）；401 生产(劳务)成本=资产（年末有意不清零）；备抵 133/152/162 Property=贷；告警口径 = FinalAtOrBefore（休眠科目可见）；编码主键化延后（docs/account-code-design.md §四 5 触发器）。

## 语音/MCP 记账方案（2026-09-13 定稿，未立项）

- **方案甲（用户拍板）**：**md 即草稿**——不建草稿区。凭证 md + 其 git commit 本身就是草稿态；改错 = 改/删 md 再重生成，零认知成本。用户原话："录入本月 5 号凭证不会影响前 4 号，git 历史一看便知。"
- **层次（用户纠正，我此前说反）**：git 只管**输入**（凭证 md + `{year}.json`）；**账本层不入 git，也不该入**——账本正确性由 CLI 代码保证（铁律三：JSON 权威、xlsx 可幂等重建）。限定词：生成读上月 xlsx 当期初，但无上月可纯 JSON 重建、双源冲突采信 JSON，**权威链始终在 JSON**。
- **A→B 边界**：边界不是"文件落盘"，是"**generate 跑过了**"。用户显式要求才 generate（**不允许 agent 私自生成**）；已生成月份的凭证默认**只冲销**，`-f` 改凭证需再三确认（不可怕——git 有历史，随时回退重决冲销 vs -f）。
- **时序 = 机器检查，不是事前约束**：脚本扫每月连续/无重/无断码，异常 → agent → 用户决策。
- **接入选型 = MCP（用户判断，我认同）**：skill 的前提是"agent 在账套那台机器上、能读文档敲命令"，语音/移动端 agent 摸不到磁盘 → 该模式失效；MCP 把能力变成可远程调用的语义化工具。**CLI 不消失**，是 MCP server 的实现底座。
- **传输**：**局域网优先**（用户："只要在一个局域网，我就能接上这个 mcp"）——正好是"数据不出本地"（2026-08-28 定的商业前提）最干净的形态。外网中继延后：同一 server 加传输层即可，不重构。
- **红线**：
  1. **generate 不进 MCP 工具面**——"不允许 agent 私自生成账本"只有不暴露才是结构性保证（暴露 = 提示词级软约束 + 事后可查）。
  2. **月目录白名单**：只准 `记字第XXXX号.md`。`CollectEntries`/`ParseDir` 是 `WalkDir` **递归**，多余 .md 被静默记账——`记字第0026号 副本.md` 仅重号告警（**不阻断**），`草稿.md`/`模板.md` **连告警都没有**（`voucherNumRe` 要求文件名含"记字第"）。
  3. MCP 实例绑定单一账套根目录，**路径不接受 LLM 传参**（"一次只碰一个账套"升级为工具层硬隔离）。
  4. 硬规则（借贷平衡/先定义后生成/幂等/余额链）只在 CLI，**禁止上移到 MCP 层**（否则两套规则）。
  5. **未配 token 时禁止非本机绑定**（fail-closed）；局域网阶段即要求 token——同一 Wi-Fi 下有其它设备。
- **缺口 4 项（待立项）**：① `ledger voucher add`——**CLI 目前只有读凭证，无任何创建凭证 md 的命令**；② 断码检查（**当前只有重号告警、无连续性检查**）+ 月目录白名单；③ 凭证号服务端串行分配 + 重试幂等；④ `ledger mcp serve`。
- **MCP 工具面（建议最终）**：`subjects.list`/`subjects.match`（只读，供 agent 对齐科目名与同音字候选——设计稿 §四"语音立项工作项"的落地处）、`voucher.add`、`voucher.list`、`check`、`query`（只读）。**不暴露** generate/lock/gen-close/year-close/任何 `-f`/任何路径参数。
- **确认颗粒度（我的设计建议）**：用户确认的对象必须是**磁盘上的 md 文件内容**（agent 先落盘、再把文件念回来），**不是 agent 凭记忆的转述**——否则"说的"和"写的"可能不一致，幻觉就藏在这里。

## 合并总账职责划分

- 合并父级月结/期初由 WriteMergeGLClosings **专职**；普通 GL 流程一律排除 mergeSet（不进 initials）。新增任何对 GL sheet 写月结/期初的代码必须检查排除合并父级；独立明细页月结同理归 WriteMLMonthClosings 专职。

## 打印版位格（红线与坑精选）

- **红线（用户锁定）**：colScale/rowScale 与金额子列数 12/11/10 不开放区域级自定义，复杂度请求一律拒绝；配置必须显式存在（缺失自动补默认 print-config.json）。
- 拆位仅数据区；样式缓存 key 必须含列数 n；dividerStyles(n) 勿硬编码 0/3/6；excelize 字节输出非确定（验证用语义对比）、GetRows 裁剪尾部纯样式行、openpyxl 读列宽不可靠、yuanStrToCents("") 返回 0。

## 发版纪律与工作流

- 六步见 CHANGELOG.md 文末；push v* tag 触发 CI goreleaser，**严禁手动 gh release create**；push/tag/合并前必须用户显式批准（user 红线，不委托）。
- **技能双目录逐字同步铁律**：`.agents/skills/ledger-accounting/` ↔ `embedded/ledger-accounting/`，embed_test.go 守护；只改 .agents = 发旧技能。
- 发版前三重复核：联网比对官方原文 / 逐 commit 对照设计稿 / 红队对抗实验。
- 测试坑：e2e 包级缓存必须 `-count=1`；测试改 JSON 必须读-改-写只动目标字段；cmd 进程内 runCmd 的 pflag 值跨 Execute 持久（已在 runCmd 统一重置）；文件编辑锚点必须含完整行（截断在行中会留残留文本）。
- 跨 agent 记忆入口 AGENTS.md（CLAUDE.md 是一行 `@AGENTS.md` 垫片，勿"修复"删掉）；记忆本体 `.workbuddy/memory/`，完工后更新 MEMORY.md + 当天日志，禁止另存副本分叉。
- 审计 Change 1-12 全部 archive（`.worktrees/change/audit-fix` 按用户要求保留）；资产负债表口径：左列资产、右列负债/权益、损益不入列（结转后权益列 = 收益分配）。
