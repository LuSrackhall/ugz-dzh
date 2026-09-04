# 项目长期记忆（ugz-dzh 手工账电子化）

## 科目体系 v0.9.0：先定义后生成 + 官方 42 科目表（2026-09-04）

- **宪法级原则（openspec 铁律 3 已修订）**：generate 遇科目树未定义的凭证科目 → 输出清单并拒绝；`--allow-new` 为显式逃生。新工作流：scan（宽容吃 OCR md）→ 旧账期末余额表转录为权威基底双向 diff → subjects import（方向列对新登记科目同样生效）→ opening import（试算闸门）→ check --vs + 三数对平 → lock。
- **官方科目表（balance/official_accounts.go）**：财会〔2023〕14号 42 个一级科目，按"最终结构"定义（顺序号/编码/名称/属性/官方大类/损益段位/现行类别）——这是编码主键化延后的唯一留口子。**运行时只消费"现行类别"，AccountTypeOf 返回值冻结现行五类**（资产/负债/权益/收入/费用；引入官方大类进运行时会落 report_sheets switch 空档静默消失）。401 生产(劳务)成本 显式决策=资产（官方编制说明：存货按库存物资+生产(劳务)成本等合计填列；年末不清零）。**备抵科目 133/152/162 Property=贷**（官方"期末贷方余额"，财政部 PDF 逐字核对——按大类映射会错标借）。
- **科目编码主键化延后**（5 触发器见 docs/account-code-design.md §四）：镇级汇总/带码报送、真实改名事故、手工录入界面立项、跨年比较、配置名单静默失效。辩论结论：输入层与标识层正交；语音+agent 输入层优于人工背码；编码的标识价值=低频高害+agent 可兜+常量表零成本期权。
- **两处真静默已堵**：收支结余表未分类科目单列（此前无痕丢行）；gen_close/year_close 对未知类别余额≠0 告警。**告警/统计余额口径=FinalAtOrBefore（≤快照月最近记录回退）**——当月无发生的休眠科目必须可见，否则被期初回退静默带入新年。
- **gen-close 预登记"本年收益"**：收紧后结转凭证引用的结转目标科目会被自家闸门拦截（自锁，红队实测）——gen-close 写凭证前自动登记（属性=贷，系统自洽行为，非静默长科目）。
- **技能双目录同步铁律**：`.agents/skills/ledger-accounting/`（源）↔ `embedded/ledger-accounting/`（发布二进制内嵌副本）必须逐字一致；embed_test.go 双向守护测试锁死。**只改 .agents 等于发旧技能**（v0.9.0 前曾发现 embedded 停在 0.8.4、与收紧行为直接矛盾）。
- **验收教材**：发版前三重复核（数据核对联网比对官方原文 / 代码审查逐 commit 对照设计稿 / 红队对抗实验）——本轮抓出 1 blocker + 1 流程锁死 + 6 问题；红队确认收紧零错账路径（拒绝全部发生在写盘前）。

## 合并总账月结职责划分（2026-08-30 连续月结行 bug 修复）

- **架构教训**：合并总账父级 sheet 名 = `sheetNameGL(父级)` = "总分类账-{父级}"，与普通叶子 sheet 命名**同源**。`WriteMonthClosings` 的 M4 补月结逻辑（对"期初≠0 且当月无分录"科目补月结）**必须排除合并父级**（构建 mergeSet 跳过），否则与 `WriteMergeGLClosings` 对同一 sheet 双写月结 → 连续两套月结行，且 M4 那套期末=期初、未计入当月子科目发生额（**错误数据**）。修复点：monthly_close.go（M4+主循环）、generate.go appendCarryForwardOnly。
- **D1a 拦截不完整（根因）**：generate.go:31-49 的 D1a 只拦"建账月期初调整额"这条让 initials[父级]≠0 的路径，漏了"科目树 Tree[父级].Balances 历史期末余额"这条路径（旧版本回写/year-close 跨年结转/手动编辑写入父级）。`GetInitBalanceForGenerate` 路径3 读 Tree[父级].Balances → initials[父级]≠0 → M4 触发。**根治**：GenerateWorkbook 构建 initials 时 Tree 遍历也跳过 mergeSet，合并父级不进 initials → 不进期初表/期末表/试算平衡/UpdateBalancesAfterGenerate 回写（合并父级是汇总视图，期初=子科目之和，列了重复）。
- **e2e 覆盖缺口**：test/e2e 用例 `合并总账科目: []`（空），完全不覆盖合并路径，导致 bug 长期潜伏。已补 `test/e2e/merge_close_test.go` `TestMergeGLNoDoubleClose` 固化（构造合并科目+历史余额，断言合并 sheet 月结各1次）。
- **P1-1 翻页保护（已修，2026-08-30）**：`writeMergeGLClosings`（merge_gl_sheet.go）原连写4行月结缺 checkBreak 翻页保护。已照搬 WriteMonthClosings 的 checkBreak 闭包：每行月结前检查页容量，满了过次页+承前页，保证余额链连续（铁律二）。writeMergeGLClosingRows 签名加 account 参数（用于 writePageHeader/writeCarryForwardRow）。
- **isNew 判断修复（2026-08-30，合并账页首次新建期初丢失）**：`appendToMergeGLSheet` 原用 `len(rows)<=2` 判 isNew，但 `ensureMergeGLSheet` 新建时已写标题（行1-6有值）→ isNew 恒 false → 合并页**首次新建**（建账月配合并科目/普通科目转合并）时**不写期初行、余额链从0起算**，与期末余额表不一致（复现：固定资产分录余额25000 vs 期末25678，丢678期初）。修复：`ensureMergeGLSheet` 返回是否新建标志，`appendToMergeGLSheet` 用它替换 len(rows)<=2。
- **insertCarryForward 行位修复（2026-08-30）**：`insertCarryForward` 原 `row := DataStartRow+1`=6 落在子表头行（SubHeaderRow+1），期初行与表头重叠（标签经 G5:G6 合并显示在表头行）。修复：`row := DataStartRow+1+TopMarginRows`（=7，数据首行，与 nextDataRow 空 sheet 分支一致）。影响普通 GL 全新 sheet 期初行位置（行6→行7，修正）。
- **合并账页月结行边框修复（2026-08-30，打印版边框错乱）**：`writeMergeGLClosingRows` 的月结样式原为灰(808080)部分框（monthlyStyle 仅 top、qtStyle 无边框、cumStyle/endStyle 仅 bottom）。有分录月靠 appendToMergeGLSheet 的账页预置绿框(006100)打底掩盖；**无分录月**（appendToMergeGLSheet 不执行、无预置边框）月结行 L/R/T 缺失 → 打印版列线断裂/边框错乱。修复：4 个月结样式改为完整绿框（top/right/bottom/left #006100 thin），endStyle bottom #000000 medium，与普通 GL 月结样式一致。
- **职责划分原则**：合并总账父级的月结/期初完全由 `WriteMergeGLClosings` 专职（汇总子科目），普通 GL 流程（WriteMonthClosings/appendCarryForwardOnly/initials）一律排除 mergeSet。新增任何对 GL sheet 写月结/期初的代码，必须检查是否排除合并父级。

## 打印版位格输出（2026-08-24）

- 架构：**同文件复制 + 几何变换**（非记录器、非 InsertCols）。复制查看版 xlsx 为打印版，对 GL/ML Sheet delete+NewSheet+MoveSheet 重建，逐格复制（非金额复用 styleID，金额列展开 12 小列）。查看版生成代码 0 改动。
- 入口 `generator.TransformToPrint(viewPath, printPath)`，cmd/generate.go 在 GenerateWorkbook 后调用（失败仅告警）；`-f` 级联清理 `print/`。
- GL/ML 均为**多页块**结构，isLabelRow 须按块周期判定（非固定首行）：GL blockRows=(SubHeaderRow+1)+pageSize+1+BottomMarginRows；ML blockRows=DataStartRow+pageSize+1+BottomMarginRows。
- **元|角红细线在 dividerStyles 索引 9**（元=标签索引9、角=索引10）。旧 worktree-print-digit-columns 错放在索引8（未采用）。
- **excelize 字节级输出非确定**（map 迭代影响 sharedStrings/样式注册顺序）——main 自比也字节不同。验证查看版"不变"须用语义对比（单元格值多重集），勿用 cmp/字节 diff。
- **金额列空值格必须走拆位**（按 0 生成 12 空格 + 分组竖线），不能铺原样式——否则原金额格红双线左右边框污染 12 小格（双线溢出、无分组竖线、"整体金额"视觉）。判别：金额列 val=="" 但 sid!=0 → 拆位；val=="" 且 sid==0 → 跳过（未渲染侧）。
- **拆位（分组竖线）仅限数据区**（isDataRow：GL (r-7)%28<21 / ML (r-9)%30<21，=数据20行+过次页1行）。表头/标题区/下边距区的空值金额格必须铺 **amountEdgeStyle（仅继承边界：top/bottom 继承铺满、k=0/k=11 继承左右、中间格无边框）**——不能拆位（分组竖线溢出标题区）、不能整体铺原样式（红双线复制 12 条）。
- **坑：`yuanStrToCents("")` 返回 (0,nil)**——空串被当数值！用 `isNumericAmount` 判数值必须带 `val != ""`。
- 装订侧红双线在**数据列边缘**（GL col3 左/col28 右；ML 明细3右/明细4左），不在装订列本身（装订列 sid=0）。验证红双线延伸须查数据列边缘，勿查装订列。
- **坑：excelize GetRows 会裁剪末尾"仅有样式、无值"的行**（末块下边距行只有红双线无文字）→ readSheetMeta 必须向上探测尾部样式行（maxRow+1..+5 GetCellStyle!=0），否则打印版末块下边距行整行丢失（"红双线未延伸至下边距行"，只在最后一张逻辑表出现）。行数对比用 openpyxl max_row（含样式行），勿用 excelize GetRows。
- **ML 第 1 块（Paper1 r1-30）Back 侧查看版未渲染**（无边框无数据）→ 打印版镜像一致即可，**勿擅自补线**（d8e5c3a 因此被撤回）。
- **h4 标签行"装订边看不到"的真正根因（用户亲自定位，e0199c5 解决）**：12 列每列仅 ~1.08 字符宽，7pt 字写入即贴边 → 边框被字体"视觉挤掉"（实际已设置，删字即见）。**解法=减少列数**而非补线/合并区（前两个假设 d8e5c3a/755db46 均被撤回）。教训：数据存在≠渲染可见，但也要考虑**列宽 vs 字体**的物理挤压。
- **ML 金额列数参数化（e0199c5）**：借/贷/余 **11 列**（去十亿位）、明细 **10 列**（去十亿/亿，最高千万）。数据上限验证：9 个月最大金额 24,180,554.26 元（2418 万）< 明细 10 列上限 9999 万 < 借/贷/余 11 列上限 99.9 亿。**GL 仍 12 列**。API：`digitColLabels(n)`（12:十亿…分 / 11:亿…分 / 10:千万…分）、`splitCNY(cents,n)`（元对齐 n-3）；colMap 支持每列独立展开数 `splitCols`。
- **分组边框规则（用户定义，58ba4b5）**：单红线（CC0000/thin）**永远在元|角之间**（分隔符索引=元所在索引=n-4）；从元开始每三位一组（元十百|千万十万|百万千万亿），**组界加粗绿线**（006100/medium）= 分隔符索引 **yuanIdx-3/-6/-9**（12列→6,3,0；11列→5,2=千|百、百|十；10列→4,1）。**dividerStyles(n) 勿硬编码 0/3/6**（仅 12 列碰巧对）。
- **坑：样式缓存 key 必须含 n**——amountSubStyle/amountEdgeStyle 的 cache key=(styleID,k,n)，否则同一 styleID 在 GL(12列)/ML(11,10列) 下 k 相同但元位置不同，缓存串用 → 红细线错位、列内部出现不该有的红线。
- **ML 打印版新列坐标**（借/贷/余=11列、明细=10列）：明细4末格=c80（装订边左界右框）、明细5首格=c85（装订边右界左框）；装订区 c81-84。**旧 12 列坐标 c91/c96 已废弃**。
- **查看版 cols 列宽是范围化的**（excelize GetColWidth 读取正确）；**openpyxl 读列宽不可靠**（范围列返回默认 13.0）——判断列宽用 excelize。

## 工作树约定
- 打印版相关工作树：`worktree-print-fresh`（本次全新实现，已提交未 push）。旧 `worktree-print-digit-columns`（locked，记录器架构，未采用）。
- 审计修复：`change/audit-fix` 分支经用户批准**分批合并 main 并 push**（2026-08-27，main = 56d56fd）：Change 1-5（期初/平衡/幂等/锚定建账月）+ Change 6-7（红字/期初回退/合并累计/合并父级冲突/跨年校验）+ Change 8（投产门槛）+ Change 9（日记账/试算平衡/year-close JSON/结转草稿）+ Change 10（结转凭证自动生成 gen-close→closing/，不进 git）+ **Change 11（报表套件：资产负债表/收支结余表/科目汇总表/凭证序时簿/结账标记 lock/未分类）+ Change 12（rebuild.sh 全量重建/check 漂移比对/重号检测/红字文档）**。**12 个 change 全部 archive**。`.worktrees/change/audit-fix` **保留未清理**（用户要求）。
- **资产负债表口径（会计专家二次复核）**：左列只列资产类、右列只列负债/权益类；收入/费用科目不进列，以"本年收益（未结转损益）"单行汇总入权益侧；未分类科目表底列出标注。结转后"本年收益"科目入权益列。

## 审计修复历程（2026-08-26 ~ 08-27，8 个 change，四轮专家审查收敛）
- 会计专家（agent-6ad11b15）+ 审计专家（agent-612211a3）四轮审查，每轮发现问题→修复→子 agent 验收→用户批准合并。
- 关键修复：期初锚定建账月（Change 5，会计语义修正）；期初回退含 0（H1）；红字四格式+打印红色字体（H2）；合并累计并集（H3）；合并父级冲突双拦截（D1）；跨年结转三告警（F1）；投产 5 门槛（Change 8）。
- **投产结论（2026-08-27）**：有条件投产——5 条必须门槛已全部补齐（Change 8），系统达到可投产条件；全量重建脚本/check xlsx 漂移比对为可选后续项。

## 项目原则：CLI 是绝对安全对象（2026-08-26 用户明确）
- **CLI 产物（ledger 各子命令，尤其 generate）才是需要"绝对安全"的核心**；`scripts/test-e2e.sh` 只是开发测试工具，其便利（如 --keep-json 续跑）不能替代 CLI 内建的安全机制。
- 安全必须内建于 CLI 强制执行，不能靠脚本传参/使用习惯约定：generate 幂等（检测"本月合计"行）、借贷平衡校验、期初机制正确（调整额生效/无幻影期初/属性不推断）、科目映射合并、跨年结转干净。
- 用户工作流宗旨：json+凭证.md → 账本；JSON 唯一权威源；git 管变更（新增科目/期初调整/余额回写都要在 JSON diff 真实可见且真实生效）。
- **合并账页最终修复（2026-08-30 第六轮，main=06ca20b）**：用户撤回 fix-merge-20260830 分支后，在 4b696b4 基线重新根治三缺陷——D1 有余额无分录合并账页整体消失（跨年 1 月主诉）、D2 新建页期初行丢失（isNew 恒 false）、D3 月结边框不自包含（打印列线断裂）。提交 1e50155 + 06ca20b；36 项全账本验证通过；详见 2026-08-30.md。
