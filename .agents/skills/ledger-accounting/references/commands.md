# 命令详细用法（ledger CLI）

二进制：发布包提供的 `ledger.exe` / `ledger`。

## init — 建账
```
./ledger init -s 2025-10 -o output
```
- `-s` 建账月（启动月）：决定期初调整锚定月、跨年结转基准
- **一次性建立完整账本管理体系**（输出根目录）：
  - `vouchers/` — 凭证输入目录（+ README 命名说明），每月凭证放 `vouchers/YYYY_MM/*.md`
  - `print-config.json` — 打印版配置模板（generate **自动发现**，放输出根目录即生效；已存在则保留用户修改）
  - `README.md` — 体系使用说明
  - `output/<年份>/<年份>.json` — 年度配置（含全局设置/科目树/科目映射/合并总账科目等）
- 幂等：print-config.json / README / vouchers 已存在不覆盖（仅补缺失项）；年份 JSON 已存在则报错拒绝

## generate — 生成月度账本
```
./ledger generate -v <凭证目录> -o <输出目录> [-f] [-V] [-p <平台>] [--config <print-config.json>]
```
- `-v` 凭证目录：**该目录下所有凭证必须同一年同一月**（报错则检查是否混入其他月）
- 无 `-f`：幂等保护——已生成月份（含"本月合计"）拒绝重跑
- `-f`：覆盖重建当月（从当月删除重建，-f 级联需逐月或使用 rebuild.sh）
- `-p/--platform <auto|mac|windows>`：打印版目标平台（默认 auto=当前系统；在任一平台都可用 `--platform windows` 生成 Windows 版打印版（跨平台生成，便于在一台机器上产出两端账本））
- `--config <print-config.json>`（可选）：打印版配置文件——平台补偿系数（colScale/rowScale）与分区域字体（fonts.normal/digit/title/default），解决 WPS 各平台/机器渲染尺寸不一致；不传用默认值（Windows 列宽×1.1075/行高×0.992，Mac 恒 1.0）。字段说明见 references/print-config.md，模板见 examples/print-config.example.json
  - **自动发现 + 自动创建**：未传 `--config` 时，依次查 **当前目录 → 输出根目录** 的 print-config.json；**都没有则自动创建默认模板**（输出根目录，含全部默认字段）——**不存在"无配置"状态**，删除文件下次生成自动重建
  - **必须是英文键**：`platforms.{windows,mac}.{colScale,rowScale,fonts.{normal,digit,title,default}}`（中文键会报错）
  - 文件须 **UTF-8 无 BOM** 编码（JSON 不能有注释/尾逗号）
  - **只作用于打印版** `输出目录/<年份>/print/*.xlsx`（查看版不变），须重新 generate 后看新文件
  - 自检：generate 启动会打印 `打印版配置: 已加载/已自动发现当前目录/默认 → 平台=.. 列宽系数=..` 行，`默认` = 配置文件没被读取
- 自动并入 `<输出目录>/<年份>/closing/*.md`（系统生成的结转凭证）
- 凭证号未解析 → 阻断报错；文件名凭证号重复 → 告警

生成的账本 xlsx 包含：
- 总分类账-<科目>（每科目一页，三栏式 + 本月合计/本年累计/期末余额）
- 多科目明细账-<总账科目>（明细多栏）
- 总分类账-<父级>（若配置"合并总账科目"）
- 现金日记账 / 银行存款日记账（逐日逐笔 + 本日合计 + 本月合计/本年累计/期末结存）
- <月>期初 / <月>期末（借贷分列试算平衡）
- 资产负债表 / 收支结余表 / 科目汇总表 / 凭证序时簿
- print/ 子目录：打印版位格（GL/ML 拆位 + 红字红色字体）

## map — 科目映射纠错
```
./ledger map add -f <凭证中的错名> -t <正确科目名> -j <json>
./ledger map delete -f <错名> -j <json>
./ledger map list -j <json>
```
合并科目（凭证写错名时统一纠正），映射存 JSON 全局设置.科目映射表。

## add-manual — 期初调整
```
./ledger add-manual -a <科目> -m <月> -n <金额> -t <备注> -j <json>
```
- **只作用于建账月（启动月）**；`-m` 仅记录不参与计算
- 建账月账本已生成时：修改后须 `generate -f` 从建账月重建才生效
- 合并总账科目（父级）禁止设期初（须设到子科目）

## subjects — 科目批量登记（list / export / import）
```
./ledger subjects import -f <建账审核表.csv> -j <json> [--dry-run]
./ledger subjects list   -j <json>
./ledger subjects export -j <json> [-o <导出.csv>]
```
- 审核表格式（列序无关）：`科目,方向,期初余额,备注`——「方向」= 借/贷（即科目属性）；期初余额列在 subjects import 中忽略
- `import`：批量登记科目并设置属性。新科目以 0 期初登记；**已存在科目只更新属性，绝不动期初调整额**
- 未分类科目指定属性的唯一入口（属性=方向）
- `export`：导出科目树为审核表格式，供与旧账科目余额表做双向 diff（盘点）
- 合并总账科目（父级）禁止登记

## opening — 期初余额批量导入
```
./ledger opening import -f <建账审核表.csv> -j <json> [--dry-run]
```
- 读取审核表「期初余额」列非空的行，批量写入期初调整额（**与 add-manual 同语义**：锚定建账月，借→+金额，贷→-金额）
- 写入前三重校验，任一失败即拒绝：① 科目已登记且非合并父级；② 科目属性与方向一致；③ **试算平衡强制闸门**（含已有调整合并计算，不平拒绝并列出各科目差额——不平衡说明旧账有问题，先对账）
- `--dry-run` 预演：打印拟导入明细与校验结果，不落盘——先迭代到全通过再正式导入
- 同科目重复导入 = 更新（修正用）；无余额科目行自动跳过（仅登记）
- 建账月账本已生成时：导入后须 `generate -f` 从建账月重建才生效

## year-close — 跨年结转
```
./ledger year-close -j output/2025/2025.json -o output
```
- 自动生成新年 JSON（output/2026/2026.json）+ 空账本
- 三告警（不阻断）：最近月非 12 月 / 期初借贷不平衡 / 损益类科目漏结转
- 输出损益结转草稿（借 本年收益 / 贷 科目 按余额方向）
- 提示期初锚定建账月（新年度期初 = 上年末自动结转）

## gen-close — 生成结转凭证
```
./ledger gen-close -j output/2025/2025.json -o output
```
- 对仍有余额的收入/费用科目生成年末结转凭证 → `output/2025/closing/记字第X号 年末损益结转.md`
- 幂等：已结转科目（closing/ 已有凭证覆盖）跳过，不重复生成
- 凭证日期 = 余额月最后一天；借贷恒平衡
- closing/ 是派生产物（不进 git）；重新 generate 该月即完成损益结转

## lock — 结账标记
```
./ledger lock -j <json> -m 2025-10     # 设置结账月
./ledger lock -j <json> -m ''          # 解锁
```
- `<= 结账月` 的月份：generate 无 `-f` 时拒绝（防误改）
- 格式校验 YYYY-MM

## check — 账本检查
```
./ledger check -j <json> [--vs <建账审核表.csv>]
```
- 科目树一致性验证
- 期初试算平衡（最新月份快照，借正贷负，0=平衡）
- xlsx 漂移比对：最新月 xlsx 期末表 vs JSON Balances（手改账本可检出，-f 重建修复）
- 未知类别科目提示（属性=未分类）
- `--vs`：迁移验收用——逐项比对 JSON 期初调整额与老板批准的建账审核表（防录入漂移；不一致即报错并列出明细）

## rebuild.sh — 全量重建
```
bash scripts/rebuild.sh <凭证根目录> <输出目录>
```
- 脚本随本 skill 分发（scripts/rebuild.sh，生产环境可从 skill 包提取到本地执行）
- 读 JSON 启动月，逐月 `generate -f` 重建
- 凭证目录支持 `<根>/2025_10/` 或 `<根>/2025/10/` 两种形态
- 灾难恢复：xlsx 丢失/漂移时一键从 JSON + 凭证重建

## 其他
- `reset`：重置相关状态（按需）
- 帮助：`./ledger <命令> --help`

## config — 打印版配置模板库（list 列出 / apply 应用）

```bash
./ledger config list                                          # 列出内置模板 + 适用说明
./ledger config apply mac-win-common -o <输出根目录>           # 应用模板（-f 覆盖已有）
./ledger config apply win-standard -o <输出根目录> -f          # 覆盖式应用
```

- 模板库：技能包内 `templates/*.json`（win-standard / mac-standard / mac-win-common，随版本发布）
- 模板名支持：简称（`mac-win-common`）/ 带后缀（`mac-win-common.json`）/ 全名（`print-config.mac-win-common.json`）
- 已有 `print-config.json` 时需 `-f` 覆盖（防误毁已标定配置）；应用后重新 `generate` 即生效（自动发现，无需 `--config`）
- 贡献：机器上标定完美后把 print-config.json 交开发方入库，下版 `config list` 可见

## doctor — 环境自检（agent 排障先跑这个）
```
./ledger doctor [-o <输出根目录>]
```
- 一次查清：程序版本、skill 是否安装且自包含（references/print-config.md 在=新版）、**CLI 与 skill 版本是否匹配（VERSION 文件）**、print-config.json 能否被自动发现、输出目录是否有 {year}/{year}.json
- 输出 `[OK]/[WARN]/[FAIL]` 逐项 + 结论统计 + 修复指引
- 生产环境遇到"配置不生效/命令报错/agent 行为异常"时，先跑 doctor 定位，再按 [FAIL] 项修复（多为重跑 install-skill 或 init）

## install-skill — 安装本技能
```
./ledger install-skill                          # 交互选择接入 agent（1=WorkBuddy/2=Claude Code/3=Cursor）
./ledger install-skill --select 1,2             # 非交互指定
./ledger install-skill --real-workbuddy         # WorkBuddy 用真实复制（软链接不被加载时）
```
- **版本联动**：安装时把当前 ledger 版本写入 `.agents/skills/ledger-accounting/VERSION`——**更新 ledger 后必须重跑 install-skill**，否则技能与程序不匹配；用 `ledger doctor` 校验（版本不一致会 [FAIL] 并提示重装）
- **仓库级安装（唯一推荐）**：技能只装到 **CLI 所在目录**的 `.agents/skills/`（仓库级），配合 CLI 使用
- **纳入 git 管理**：账本目录若为 git 仓库，安装/升级技能后执行 `git add .agents/skills/ && git commit`（`.agents/skills/` 真实源已纳入 git），技能版本随仓库可追踪、可回滚、可随仓库分发（换机 clone 后重跑 install-skill 即恢复）
- 安装后，你的 agent 工具（WorkBuddy / Claude Code / Cursor / dsh 等）在对话中遇到记账、建账、结转等操作时会自动加载本技能来指导你
- 标准源 `.agents/skills/` 始终安装（支持 Agent Skills 开放标准的工具原生读取）

### 用户级残留自查（agent 负责，CLI 不代劳）
技能应仅存在于仓库级 `.agents/skills/`。历史版本（≤0.7.2）或误操作可能把技能装到 **home 下各 agent 用户级目录**，造成版本不一致与维护混乱。agent 在安装/升级技能后应自查并清理（CLI 不硬编码 agent 清单——agent 自己知道本机有哪些 agent 工具目录，天然不过时）：

```
# 自查：枚举本机各 agent 的用户级技能目录（示例：~ 下常见位置）
ls -d ~/.workbuddy/skills/ledger-accounting \
     ~/.claude/skills/ledger-accounting \
     ~/.cursor/skills/ledger-accounting \
     ~/.codex/skills/ledger-accounting \
     ~/.gemini/skills/ledger-accounting \
     ~/.windsurf/skills/ledger-accounting \
     ~/.zed/skills/ledger-accounting 2>/dev/null
# 也检查本机实际安装的其它 agent 工具目录（Agent Skills 标准：<agent-home>/skills/）
# 发现残留 → 移除（推荐仅保留仓库级）：
rm -rf <上述列出的残留目录>
```
清理后 `ledger doctor` 确认 skill 安装位置为仓库级 `.agents/skills/` 且版本匹配。
