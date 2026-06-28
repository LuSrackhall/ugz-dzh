## Why

当前系统只生成单一版本的 Excel 文件，金额使用普通数字格式 `#,##0.00` 展示，无法满足传统手工账的美观打印需求。用户需要：
- 金额分栏展示（十亿千百十万千百十元角分），符合手工账规范
- 支持正反面打印的布局，便于装订成册
- 同时保留计算验证能力

## What Changes

### 新增能力
1. **打印版 Excel**：金额分栏展示，用于快速预览和简单打印
2. **HTML 打印版**：精美样式，支持左右对开的正反面打印布局
3. **金额分栏转换**：`centsToDigits` 纯函数，将金额拆为 12 位数字

### 修改能力
1. **CLI 命令扩展**：`ledger generate` 支持 `-view-only`、`-print-only`、`-html-only` 参数
2. **查看版 Excel**：保持现有实现，无需修改

### 输出结构
```
output/
├── 2026-01.xlsx          # 查看版 Excel（现有）
├── 2026-01-print.xlsx    # 打印版 Excel（新增）
└── 2026-01-print.html    # HTML 打印版（新增）
```

## Capabilities

### New Capabilities
- `print-excel-generation`: 打印版 Excel 生成能力，包括金额分栏布局、美化样式、打印参数设置
- `html-print-generation`: HTML 打印版生成能力，包括模板设计、CSS 打印样式、正反面布局支持
- `amount-digit-formatting`: 金额分栏格式化能力，包括 `centsToDigits` 纯函数和相关单元测试

### Modified Capabilities
- `cli-commands`: 扩展 `ledger generate` 命令，支持 `-view-only`、`-print-only`、`-html-only` 参数
- `excel-generation`: 可能需要调整以支持打印版的列定义和样式应用

## Impact

### 代码影响
- **generator 包**：新增 `amount.go`（金额转换）、`print_sheet.go`（打印版 Excel）、`html_print.go`（HTML 模板）
- **cmd 包**：扩展 `generate.go` 命令，添加按需生成参数
- **embed.FS**：嵌入 HTML 模板和 CSS 样式文件

### 依赖影响
- **无新外部依赖**：HTML 版使用 Go 标准库 `html/template`
- **现有依赖不变**：继续使用 excelize/v2

### 风险
- Excel 列宽可能超限（缓解：缩小字号或使用 A3 纸张）
- HTML 浏览器兼容性（缓解：测试主流浏览器）
- 正反面对齐困难（缓解：提供打印指引）
