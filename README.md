# 手工账电子化生成系统

将手工记账凭证（Markdown 文件）自动转为每月独立、完整的累计 Excel 工作薄，支持总分类账和多科目明细账两种格式，适配活页打印与 git 版本追踪。

## 快速开始

```bash
# 编译
go build -o ledger .

# 初始化（创建 output/2026/2026.json）
./ledger init -s 2026-01 -o ./output

# 生成单月 xlsx（年份和月份自动从凭证推导，JSON 路径自动推导为 output/{year}/{year}.json）
./ledger generate -v ./vouchers/2026_01 -o ./output

# 管理科目名称映射（OCR 纠错）
./ledger map add -j ./output/2026/2026.json -f "管埋费用" -t "管理费用"
./ledger map list -j ./output/2026/2026.json

# 手动添加调整科目
./ledger add-manual -a "银行存款-工商银行" -m 2026-03 -n 100000.00 -t "补记上年余额" -j ./output/2026/2026.json

# 检测科目树完整性
./ledger check -j ./output/2026/2026.json

# 跨年结转
./ledger year-close -j ./output/2026/2026.json -o ./output

# 重置打印标记
./ledger reset -m 2026-01 -o ./output
```

生成产物（全部在 `output/{年份}/` 下）：
- `output/2026/2026-01.xlsx` — 完整累计工作薄（总分类账 + 多科目明细账 + 期初表）
- `output/2026/ledger.csv` — 凭证分录汇总（CSV）
- `output/2026/ledger.xlsx` — 凭证分录汇总（Excel）
- `output/2026/balance.csv` — 科目余额表（CSV）
- `output/2026/balance.xlsx` — 科目余额表（Excel）
- `output/2026/2026.json` — 年份配置文件

## 凭证格式

凭证为 Markdown 文件，内含 HTML 表格。每行 5 列：

| 摘要 | 总账科目 | 明细科目 | 借方金额 | 贷方金额 |
|---|---|---|---|---|
| 购买办公用品 | 管理费用 | 办公费 | 500.00 | |
| 支付现金 | 库存现金 | | | 500.00 |

要求：
- 文件中包含日期（`YYYY年MM月DD日` 或 `YYYY-MM-DD`）和凭证号（`记字第XX号`）
- 借方和贷方金额填在对应列，另一方留空
- 合计行和表头行会被自动跳过
- **一次 generate 的凭证目录中所有凭证必须来自同一年同一月**，否则报错

## 输出目录约定

指定 `-o ./output` 后，系统自动按年份和月份创建子目录：

```
output/
└── 2026/
    ├── 2026.json          # 年份配置（科目树、余额历史、映射表）
    ├── 2026-01/           # 1 月目录
    │   ├── 2026-01.xlsx       # 查看版累计工作薄
    │   ├── 2026-01-print.xlsx # 打印版累计工作薄
    │   ├── ledger.csv         # 当月分录汇总（CSV）
    │   ├── ledger.xlsx        # 当月分录汇总（Excel）
    │   ├── balance.csv        # 当月科目余额表（CSV）
    │   ├── balance.xlsx       # 当月科目余额表（Excel）
    │   └── html/              # HTML 打印版
    │       └── ...
    ├── 2026-02/           # 2 月目录
    │   └── ...
    └── ...
```

- `init -s 2026-01` → 在 `{output}/2026/` 下创建 `2026.json`
- `generate -v <dir>` → 年份和月份从凭证自动推导，JSON 路径 = `{output}/{year}/{year}.json`

## 年份配置（{year}.json）

全局配置文件，管理所有科目的期初调整和余额历史。格式：

```json
{
  "全局设置": {
    "启动月": "2026-01",
    "科目顺序": [],
    "科目映射表": {},
    "合并总账科目": [],
    "总分类账忽略科目": [],
    "多科目明细账忽略科目": []
  },
  "科目树": {},
  "自动识别科目": [],
  "手动调整科目": []
}
```

`启动月` 决定手动补科目的期初回溯起点（从该月起计算余额），默认应为年度 1 月。后续凭证中出现的新科目会自动加入 `自动识别科目`，期初默认 0。

### 合并总分类账

指定父级科目（如 `固定资产`）生成合并 GL Sheet，其下所有子科目分录按发生时间序归入同一帐页。三个可选的独立配置项：

| 字段 | 作用 |
|------|------|
| `合并总账科目` | 为父级科目生成合并 GL Sheet（纯增量，不影响原有叶子 GL） |
| `总分类账忽略科目` | 父级下子科目不生成叶子 GL Sheet |
| `多科目明细账忽略科目` | 父级下子科目不生成 ML Sheet |

三者完全解耦，可按需组合。例如：三项全配 `["固定资产"]` → 只出合并帐页，叶子 GL 和 ML 均不生成。

### 科目名称映射表

OCR 识别凭证时可能产生错字（如"管埋费用"、"银杭存款"），导致同一科目被识别为多个不同名称。映射表用于将 OCR 原始名统一映射到标准科目名，确保科目唯一性。

**方式一：直接编辑 JSON**

在 `全局设置.科目映射表` 中添加键值对，格式为 `"OCR原始名": "标准科目名"`：

```json
"科目映射表": {
  "管埋费用": "管理费用",
  "银杭存款": "银行存款"
}
```

保存后，下次运行 `generate` 即自动应用。

**方式二：命令行管理**

```bash
# 添加映射
./ledger map add -j ./output/2026/2026.json -f "管埋费用" -t "管理费用"

# 查看所有映射
./ledger map list -j ./output/2026/2026.json

# 删除映射
./ledger map delete -j ./output/2026/2026.json -f "管埋费用"
```

映射在生成时自动生效 — 总账科目和明细科目都会匹配替换。使用 `-V` 可查看替换条数。

### 期初调整

如需调整某科目的期初余额，编辑 `自动识别科目` 中的 `期初调整额`（元），或添加 `手动调整科目` 条目：

```json
{
  "科目": "银行存款-工商银行",
  "生效月": "2026-03",
  "期初调整额": 100000.00,
  "说明": "补记上年余额"
}
```

> 期初调整的唯一入口是 JSON。xlsx 中的 `YYYY-MM期初` 工作表只是生成品，不应手动修改。

## 命令参考

```
子命令：
  generate     生成月度账本（年份/月份自动推导，凭证需同年同月）
  init         系统初始化 — 创建 {year}/{year}.json
  map          管理科目名称映射表（add / delete / list）
  check        检测 JSON 科目树与余额完整性
  add-manual   手动添加调整科目
  reset        重置打印标记
  year-close   跨年结转

运行 ledger --help 或 ledger <command> --help 查看各子命令的详细参数。
```

## 打印操作

每月 xlsx 仅标记有变化的账页为"需打印"。用户按标记打印后替换活页即可，无需重印整本账簿。

### 打印版本

系统支持三套输出，满足不同场景需求：

1. **查看版 Excel**（默认）：普通数字格式，用于计算验证
2. **打印版 Excel**：金额分栏展示（十亿千百十万千百十元角分），用于快速预览
3. **HTML 打印版**：精美样式，支持正反面打印布局

```bash
# 生成所有版本（默认）
./ledger generate -v ./vouchers/2026_01 -o ./output

# 仅生成查看版
./ledger generate -v ./vouchers/2026_01 -o ./output -view-only

# 仅生成打印版 Excel
./ledger generate -v ./vouchers/2026_01 -o ./output -print-only

# 仅生成 HTML 打印版
./ledger generate -v ./vouchers/2026_01 -o ./output -html-only
```

**输出文件**：
- `output/2026/2026-01.xlsx` — 查看版 Excel
- `output/2026/2026-01-print.xlsx` — 打印版 Excel
- `output/2026/2026-01-print.html` — HTML 打印版（用浏览器打开后 Ctrl+P 打印）

**打印建议**：
- 打印版 Excel：建议使用 A3 纸张横向打印，或缩小字号到 8-9pt
- HTML 打印版：支持 A4 横向打印，可调整浏览器打印设置优化效果

## 跨年处理

年末最后一月生成后，新年度首月生成时系统自动：
- 将各科目期末余额结转为新年度的期初余额
- 在新 xlsx 中插入"上年结转"行

## 项目结构

```
main.go                 入口（调用 cmd.Execute()）
cmd/                    CLI 子命令包（generate/init/map/check/add-manual/reset/year-close）
voucher/                凭证解析器（Markdown HTML 表格 → []Entry）
balance/                余额管理器（JSON 配置、期初计算、余额回写）
generator/              Excel 生成器（总分类账、多科目明细账、月结、打印标记）
test/e2e/               端到端测试
openspec/               项目规范与变更记录
```

## 技术栈

Go 1.21+ · excelize/v2 · 纯标准库 JSON

## 开发

```bash
go test ./...          # 运行所有测试
go build -o ledger .   # 编译
```

开发流程遵循 [OpenSpec](https://github.com/Fission-AI/OpenSpec) + [Superpowers](https://github.com/obra/superpowers) 桥接工作流。详见 `CLAUDE.md` 和 `openspec/project.md`。

## License

MIT
