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

## 合并总账职责划分

- 合并父级月结/期初由 WriteMergeGLClosings **专职**；普通 GL 流程一律排除 mergeSet（不进 initials）。新增任何对 GL sheet 写月结/期初的代码必须检查排除合并父级；独立明细页月结同理归 WriteMLMonthClosings 专职。

## ML 打印区域（v0.10.1 修正 + 审计工具）

- **ML 物理结构铁律：块=页**——每 30 行一块，右半=正面、左半=反面、两侧同页码（writeMLPageHeader 两侧传同一 logicalPageNum）；块0=Paper1 Front 占位页（页码 0，只写右侧）。**旧"滑动窗口"理解是错的**（曾致末页正面整体漏出打印区域）。
- 打印区域序列：`[占位正, 页k正(右半), 页k反(左半), …]`（正先，对齐 GL）+ 尾部补空白保偶数（跨 sheet 配对）；补页矩形须取 `blocks*blockRows` 之后的空行区（否则与末块正面区域重叠）。
- **审计工具 `scripts/render-print-areas-pdf.py`**：区域 vs 内容覆盖校验（未覆盖/重叠/奇偶三项）+ 修复前后 PDF 对比渲染（★ 标注新增页）。改打印区域后必跑。
- 环境：本机无 LibreOffice/Excel，WPS 无法程序化导 PDF（无 sdef/CLI）；渲染验证走 reportlab（venv 已装 reportlab/pypdf/openpyxl）。

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
