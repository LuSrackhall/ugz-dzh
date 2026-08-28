---
name: ledger-accounting
description: 手工账电子化工具 ledger（Go CLI）的会计操作技能。当用户涉及建账（init）、生成月度账本（generate）、科目管理（map/add-manual）、期初调整、月结、年末损益结转（gen-close/year-close）、结账标记（lock）、账本检查（check/漂移/重建）、打印版、凭证格式与红字处理、git 管理 JSON 等一切会计记账操作时使用。技能指导 agent 全程协助用户完成村级/个人手工账电子化：凭证 Markdown → Excel 账本（总账/明细账/合并总账/现金银行日记账/期初期末表/资产负债表等）→ 打印装订 → 跨年结转。
agent_created: true
version: 0.7.11
---

# ledger 会计记账操作

> **版本联动**：本技能与 ledger CLI 版本一一对应（install-skill 会把当前 ledger 版本写入安装目录 VERSION 文件）。
> **CLI 更新后必须重跑 `ledger install-skill`**，否则技能与程序不匹配；用 `ledger doctor` 校验（版本不匹配会 [FAIL]）。
> **用户画像**：最终用户是"老板"（可能完全不懂会计，只看结果），本技能是给 agent（专属会计）看的操作手册——agent 负责把老板的问题翻译成 CLI 操作，把结果翻译回老板能懂的语言。

# ledger 会计记账操作

## 概述

`ledger` 是 Go 编写的 CLI（项目根编译产物，`go build -o ledger .`）。核心流程：
**JSON 配置 + 凭证 Markdown → 每月 Excel 账本**。JSON 是唯一权威源，git 管理变更。

- **本 skill 自包含**：所有文档（commands/print-config/workflows）、配置模板（examples/）、脚本（scripts/）都在本包内，生产环境（仅 ledger.exe + 本 skill）可直接查阅，不依赖源码项目
- 产物：`<输出目录>/<年份>/{年份}.json`（配置权威源）+ `{年份}-{月}.xlsx`（账本）+ `print/`（打印版位格）
- 二进制：项目根 `./ledger`（或 `go build -o ledger .` 重建）

### 按需查阅（渐进式，不要一次全读）

只读 SKILL.md 即可完成多数操作；**遇到下列场景才读对应文件**：

| 场景 | 读哪个文件 |
|---|---|
| 需要某命令的完整参数/行为 | `references/commands.md` |
| **调打印版尺寸/字体/正反面/微调** | `references/print-config.md`（全部可配字段 + 默认值 + 示例） |
| 建账→生成→月结→结转的完整流程、常见问题处理 | `references/workflows.md` |
| 需要配置模板原文 | `examples/print-config.example.json`（= `ledger init` / 无配置自动创建时生成的模板，含全部字段） |

## 命令总览

| 命令 | 用途 |
|---|---|
| `ledger init -s <YYYY-MM> -o <输出目录>` | 建账（设置建账月=启动月，生成 {year}.json，并建好 vouchers/ 凭证目录 + print-config.json 打印版配置模板 + README，完整管理体系一次到位） |
| `ledger generate -v <凭证目录> -o <输出> [-f] [-p <平台>] [--config <print-config.json>]` | 生成月度账本（所有凭证须同一年同月；-f 覆盖重建；-p 指定打印版目标平台 mac/windows；--config 加载打印版配置：平台补偿系数 + 分区域字体 + GL/ML 分账本 + 正反面独立 + ±px 微调，见 references/print-config.md） |
| `ledger map -a <错名> -b <对名> -j <json>` | 科目名称映射纠错 |
| `ledger add-manual -a <科目> -m <月> -n <金额> -t <备注> -j <json>` | 期初调整（**只作用于建账月**，-m 仅记录） |
| `ledger year-close -j <json> -o <输出>` | 跨年结转（生成新年 JSON + 空账本 + 损益结转草稿 + 三告警） |
| `ledger gen-close -j <json> -o <输出>` | 自动生成年末损益结转凭证到 `<输出>/<年份>/closing/`（不写手工凭证目录） |
| `ledger lock -j <json> -m <YYYY-MM>` | 设置结账月（<=该月默认拒绝无 -f 生成；`-m ''` 解锁） |
| `ledger check -j <json>` | 科目树 + 期初试算平衡 + xlsx 漂移比对 |
| `ledger doctor [-o <输出>]` | 环境自检（版本/skill 安装与自包含/print-config 发现/账本结构）；**生产排障先跑这个** |
| `scripts/rebuild.sh <凭证根目录> <输出目录>` | 全量重建（读启动月逐月 -f） |

## 核心工作流

### 1. 建账（首次）
```bash
./ledger init -s 2025-10 -o output
```
- 建账月决定期初调整的锚定月（后续 add-manual 只影响建账月）
- **一次建立完整管理体系**（输出根目录）：`vouchers/`（凭证输入目录，每月凭证放 `vouchers/YYYY_MM/*.md`）+ `print-config.json`（打印版配置模板，generate 自动发现）+ `README.md` + `output/2025/2025.json`
- 幂等：重复 init 不覆盖已有 print-config.json/README/vouchers（仅补缺失）；年份 JSON 已存在则报错

### 2. 每月记账
1. 凭证放 `vouchers/YYYY_MM/`（init 建好的目录），每张凭证一个 Markdown 文件（格式见 references/commands.md）
2. 生成当月账本：
```bash
./ledger generate -v vouchers/2025_10 -o output
```
- `print-config.json` 已在输出根目录，generate **自动发现**（无需 --config）；打印版尺寸不符时直接改它，免发版
3. 复核：打开 xlsx 检查（总账/明细账/日记账/期初期末表/报表），`check` 校验

### 3. 科目管理
- 新科目出现即自动入科目树；凭证科目名写错用 `map` 纠错（合并到正确科目）
- 未知科目（不在内置 17 类）属性显示"未分类"——如需进资产负债表/收支结余表，补充类别
- 期初调整：`add-manual`（只作用于建账月；建账月账本已生成时，改后须 `-f` 从建账月重建）

### 4. 年末损益结转（"清零"，每年 12 月）
```bash
./ledger gen-close -j output/2025/2025.json -o output   # ① 生成结转凭证到 closing/
# 打开 closing/ 里的凭证核对金额
./ledger generate -v 2025_12 -o output -f               # ② 重新生成 12 月（自动并入结转，损益归零）
./ledger year-close -j output/2025/2025.json -o output  # ③ 跨年（结转后无损益告警）
```
- 结转凭证是**派生产物**（closing/ 不进 git，可幂等重建）
- 不做结转：收入/费用带余额跨年，year-close 每次告警

### 5. 结账与保护
- 每月结账后 `ledger lock -j output/2025/2025.json -m 2025-10`——该月及之前默认拒绝无 -f 生成（防误改）
- 确需修改已结账月份：`generate -f`（从该月起级联重建）

### 6. 打印
- GL/ML 账页：打印版位格 xlsx（`print/` 子目录，金额拆位、红字红色字体）
- **打印版尺寸不符**时（WPS Mac/Windows 渲染差异）按需深入，用 `generate --platform <mac|windows>` 指定目标平台，改 `print-config.json` 调（**全部字段见 references/print-config.md，模板见 examples/print-config.example.json**）：
  1. **平台系数**（粗调）：`colScale`/`rowScale`（默认 Windows 1.1075/0.992、Mac 1.0）
  2. **分账本**：`gl`/`ml` 各自覆盖（默认 Windows GL 1.13595/0.99495）
  3. **正反面独立**：`{gl,ml}.frontColScale`/`backColScale`（ML 正反面结构不对称时用）
  4. **±px 精调**：`{front,back}{Summary,Binding,Outer}Delta` —— 最终 px 增量（`+1`=最终+1px），只动摘要列/装订边/非装订边（每边一列），**金额拆位列完全不参与**
  5. **字体**：`fonts.{normal,digit,title,default,labelFamily,digitSize,digitBold,labelSize,labelBold}`（默认 Win 表头=等线 Light、Mac=宋体）
  - 标定顺序：系数粗调 → px 微调逼近；Mac 上可配合 `scripts/gen-win-test.sh` 生成 Windows 版供对比
- **⚠️ 配置自动发现**：`print-config.json` 放在**运行 ledger 的当前目录**即自动生效（无需 --config 传参；显式传参优先）；配置用英文键（中文键会报错）；文件须 UTF-8 无 BOM；只作用于打印版（查看版不变），须重新 generate 后看 `print/` 新文件。自检：generate 启动打印的 `打印版配置: 已加载/已自动发现当前目录/默认 → 平台=.. 列宽系数=..` 行
- 日记账/期初期末表/报表：直接打印查看版 sheet

## 数据宪法（CLI 安全原则，必须遵守）

1. **JSON 是唯一权威源**：科目树/余额/配置都在 {year}.json；git 提交它
2. **git 管变更**：每次生成后提交 JSON；新增科目/期初调整/余额回写都在 git diff 可见
3. **借贷平衡净额口径**：每张凭证 Σ借 = Σ贷（红字为负值参与）；不平拒绝生成
4. **幂等保护**：已生成月份无 -f 拒绝重跑（检测"本月合计"行）
5. **期初锚定建账月**：调整额只落建账月；跨年自动结转（year-close 生成新年 JSON）
6. **凭证号防线**：未解析凭证号阻断生成；重号（文件名同号）告警
7. **派生产物不入 git**：xlsx 账本、closing/ 结转凭证（可重建）；只 git 管 JSON 和手工凭证

## 常见错误与处理

| 现象 | 原因与解法 |
|---|---|
| `凭证借贷平衡校验失败: N 条分录凭证号未解析` | 凭证正文无"记字第X号"且文件名无法解析 → 补凭证号 |
| `月份 X 已结账（结账月 Y）` | lock 生效 → 确认后用 `-f` |
| `X 已生成（本月合计）` 拒绝 | 幂等 → 确认后 `-f` 重建 |
| `上月账本不存在，疑似跳月/漏月` | 缺月告警 → 补生成上月或确认 |
| `现金日记账出现负余额` | 现金支出超收入 → 核对凭证 |
| `损益类科目年末余额非 0，疑似漏结转` | 未做年末结转 → `gen-close` |
| check 报 `⚠ 漂移: 科目` | xlsx 被手工改 → 以 JSON 为准，`-f` 重建 |
| `科目 X 配置为合并总账科目，不能直接记账` | 合并父级禁止直接记账/设期初 → 用子科目 |

## 参考文件

- `references/commands.md` — 命令详细用法与参数
- `references/workflows.md` — 凭证格式、红字表达、年结细节、git 操作约定、报表说明

> 凭证格式、红字、结转等详细内容见 references；操作前先读对应章节。
