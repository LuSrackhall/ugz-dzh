# 手工账电子化生成系统

将手工记账凭证（Markdown 文件）自动转为每月独立、完整的累计 Excel 工作薄，支持总分类账和多科目明细账两种格式，适配活页打印与 git 版本追踪。

## 快速开始

```bash
# 编译
go build -o ledger .

# 初始化（创建 科目余额总览.json）
./ledger init -s 2026-01 -o .

# 生成单月 xlsx
./ledger generate -v ./vouchers -m 2026-01 -j ./科目余额总览.json -o ./output

# 手动添加调整科目
./ledger add-manual -a "银行存款-工商银行" -m 2026-03 -n 100000.00 -t "补记上年余额" -j ./科目余额总览.json

# 检测科目树完整性
./ledger check -j ./科目余额总览.json

# 跨年结转
./ledger year-close -j ./科目余额总览.json -o ./output

# 重置打印标记
./ledger reset -m 2026-01 -o ./output
```

生成产物：
- `output/2026-01.xlsx` — 完整累计工作薄（总分类账 + 多科目明细账 + 期初表）
- `output/ledger.csv` — 凭证分录汇总
- `output/balance.csv` / `output/balance.xlsx` — 科目余额表

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

## 科目余额总览.json

全局配置文件，管理所有科目的期初调整和余额历史。格式：

```json
{
  "全局设置": {
    "启动月": "2026-01",
    "科目顺序": []
  },
  "科目树": {},
  "自动识别科目": [],
  "手动调整科目": []
}
```

系统首次运行前需要手动创建此文件（至少提供 `启动月`）。后续凭证中出现的新科目会自动加入 `自动识别科目`，期初默认 0。

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
  generate     生成月度账本
  init         系统初始化 — 创建 科目余额总览.json
  check        检测 JSON 科目树与余额完整性
  add-manual   手动添加调整科目
  reset        重置打印标记
  year-close   跨年结转

运行 ledger --help 或 ledger <command> --help 查看各子命令的详细参数。
```

## 打印操作

每月 xlsx 仅标记有变化的账页为"需打印"。用户按标记打印后替换活页即可，无需重印整本账簿。

## 跨年处理

年末最后一月生成后，新年度首月生成时系统自动：
- 将各科目期末余额结转为新年度的期初余额
- 在新 xlsx 中插入"上年结转"行

## 项目结构

```
main.go                 入口（调用 cmd.Execute()）
cmd/                    CLI 子命令包（generate/init/check/add-manual/reset/year-close）
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
