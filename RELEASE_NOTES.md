## v0.2.0-rc1 - 打印版本功能预发布

这是一个预发布版本（Release Candidate），用于人工测试。

### 新增功能

- **金额分栏展示**：支持"十亿千百十万千百十元角分"的竖格展示，符合传统手工账规范
- **打印版 Excel**：生成金额分栏的 Excel 文件，用于快速预览
- **HTML 打印版**：生成精美的 HTML 文件，支持正反面打印布局
- **CLI 扩展**：新增 `-view-only`、`-print-only`、`-html-only` 参数，支持按需生成

### 输出文件

- `2026-01.xlsx` — 查看版 Excel（普通数字格式，用于计算验证）
- `2026-01-print.xlsx` — 打印版 Excel（金额分栏展示）
- `2026-01-print.html` — HTML 打印版（用浏览器打开后 Ctrl+P 打印）

### 使用示例

```bash
# 生成所有版本（默认）
./ledger generate -v ./vouchers/2026_01 -o ./output

# 仅生成查看版
./ledger generate -v ./vouchers/2026_01 -o ./output -view-only

# 仅生成打印版
./ledger generate -v ./vouchers/2026_01 -o ./output -print-only

# 仅生成 HTML 版
./ledger generate -v ./vouchers/2026_01 -o ./output -html-only
```

### 打印建议

- **打印版 Excel**：建议使用 A3 纸张横向打印，或缩小字号到 8-9pt
- **HTML 打印版**：支持 A4 横向打印，可调整浏览器打印设置优化效果

### 已知限制

- 打印版 Excel 列数较多（约 38 列），A4 纸张可能需要缩小字号
- HTML 打印版的正反面对齐依赖打印机驱动
