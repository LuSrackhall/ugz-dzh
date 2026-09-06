# 项目长期记忆（ugz-dzh 手工账电子化）

> 详细过程都在 `.workbuddy/memory/` 当天日志（git 已跟踪，30 天以上按主题蒸馏后删除）；本文件只留跨会话仍需直接可用的结论与红线。

## 年末两段结转（v0.9.4，2026-09-07）

- gen-close 默认两段齐发：① 损益→本年收益（原有一段）；② 本年收益→**收益分配-未分配收益**（制度 322 使用说明要求设明细科目；净收益 借本年收益/贷收益分配，净亏损方向相反；`--no-transfer` 跳过，重跑自动补发）。
- postFinal = Tree[本年收益].Balances[月].Final + Σ全部损益科目 Final（**不排除已结转科目**）——干净首跑/一段已生成未合并/一段已合并三态同值，老账套跑一次新版 gen-close 自愈，历史月铁律一不动；防重= closing/ 凭证含"收益分配"分录（hasTransfer；不能用"本年收益出现过"判定，一段凭证本身含它），closing/ 按年隔离不串年。
- 结转目标科目（本年收益、收益分配-未分配收益）自动登记（属性=贷）；资产负债表权益列**零改动即正确**：结转月显 收益分配-未分配收益（官方"未分配收益"项目）、未结转月显"本年收益（未结转损益）"行、混合月态两行并存（e2e 固化）。year_close 从不清零科目（AGENTS.md 1.4 已更正为现行行为）。

## 期初双源冲突：采信 JSON（铁律三落地，2026-09-06）

- 触发态 = xlsx 账页链陈旧（冲平凭证未级联重建）+ JSON 被手修 → -f 从陈旧账页链读回旧余额污染 Balances。修复 = 双源比对，不一致**采信 JSON + 告警**，-f 重建自愈；仅单源时行为不变。
- 教训：干净账本复现不出的 bug，触发态多在"外部手修"——要构造的是**状态**（双源分歧），不是更多操作序列。

## 科目体系 v0.9.0（先定义后生成）

- generate 遇未定义科目拒绝（`--allow-new` 逃生）；工作流 scan（宽容吃 OCR）→ 旧账期末余额表转录为权威基底双向 diff → subjects import → opening import（试算闸门）→ check --vs 三数对平 → lock。
- 官方 42 科目表（balance/official_accounts.go）按"最终结构"定义；**运行时只消费"现行类别"，AccountTypeOf 冻结现行五类**（引官方大类进运行时会落报表 switch 空档静默消失）；401 生产(劳务)成本=资产（年末有意不清零，勿"顺手"改）；备抵 133/152/162 Property=贷（官方"期末贷方余额"）。
- 告警/统计余额口径 = FinalAtOrBefore（当月无发生的休眠科目必须可见）；gen-close 预登记结转目标科目 = 系统自洽行为，非静默长科目。
- 科目编码主键化延后（5 触发器见 docs/account-code-design.md §四）。

## 合并总账职责划分（2026-08-30）

- 合并父级 sheet 名与普通叶子同源。父级月结/期初**完全由 WriteMergeGLClosings 专职**（汇总子科目）；普通 GL 流程（WriteMonthClosings/appendCarryForwardOnly/initials）一律**排除 mergeSet**——新增任何对 GL sheet 写月结/期初的代码必须检查排除合并父级；mergeSet 不进 initials（期初表/试算/回写全链）。

## 打印版位格（关键坑精选，详见 2026-08-24/25 日志）

- 架构 = 同文件复制 + 几何变换；GL/ML 多页块结构，isLabelRow 按块周期判定（非固定首行）。
- 拆位仅数据区；金额列空值格判别：val=="" && sid!=0 → 拆位、sid==0 → 跳过；表头/边距区空值格铺 amountEdgeStyle。
- 样式缓存 key 必须含列数 n；dividerStyles(n) 勿硬编码 0/3/6（分组边框规则用户定义：元|角单红线 + 三位分组粗绿线）；ML 金额列参数化 11/10 列、GL 12 列。
- excelize：字节级输出非确定（验证"不变"用语义对比）；GetRows 裁剪尾部纯样式行（readSheetMeta 向上探测）；openpyxl 读列宽不可靠（用 excelize）；yuanStrToCents("") 返回 0（判数值必须带 val != ""）。
- **红线（用户锁定）**：colScale/rowScale 与金额子列数 12/11/10 不开放区域级自定义，复杂度请求一律拒绝；配置必须显式存在（缺失自动补默认 print-config.json）。

## 发版纪律与工作流

- 六步见 CHANGELOG.md 文末；push v* tag 触发 CI goreleaser，**严禁手动 gh release create**；push/tag 前必须用户显式批准（user 红线，合并/push 类操作不委托）。
- **技能双目录逐字同步铁律**：`.agents/skills/ledger-accounting/`（源）↔ `embedded/ledger-accounting/`（内嵌），embed_test.go 守护；只改 .agents = 发旧技能。
- 发版前三重复核：数据核对联网比对官方原文 / 代码审查逐 commit 对照设计稿 / 红队对抗实验。
- 测试坑：test/e2e 包级缓存必须 `-count=1`；测试改 JSON 必须读-改-写只动目标字段（整文件覆写抹掉余额链，两次教训）；e2e 夹具凭证必须镜像真实结构（总帐=段名、明细=叶子分列两格）；cmd 进程内 runCmd 的 pflag 值跨 Execute 持久（--no-transfer/-f 会泄漏进下一用例，已在 runCmd 统一重置修复，2026-09-07）。
- 跨 agent 记忆入口：AGENTS.md（CLAUDE.md 是一行 `@AGENTS.md` 垫片，勿"修复"删掉）；记忆本体 `.workbuddy/memory/`，完工后更新 MEMORY.md + 当天日志，禁止另存副本分叉。
- 审计修复 Change 1-12 全部 archive（`.worktrees/change/audit-fix` 按用户要求保留）；资产负债表口径：左列资产、右列负债/权益、损益不入列（结转后权益列 = 收益分配，见年末两段结转节）。
