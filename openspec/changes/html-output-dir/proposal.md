## Why

当前 HTML 打印版文件与 Excel 文件混合输出在同一个目录（如 `output/2026/`），导致目录结构混乱，不便于管理和查找。需要将 HTML 输出分离到单独的子目录。

## What Changes

修改 `generateAccountHTML` 函数的输出路径，将 HTML 文件从 `output/{year}/` 移动到 `output/{year}/html/` 子目录。

**输出结构变化**：
```
# 之前
output/2026/
├── 2026-01.xlsx
├── 2026-01-print.xlsx
├── 2026-01-银行存款-print.html
└── 2026-01-管理费用-print.html

# 之后
output/2026/
├── 2026-01.xlsx
├── 2026-01-print.xlsx
└── html/
    ├── 2026-01-银行存款-print.html
    └── 2026-01-管理费用-print.html
```

## Capabilities

### New Capabilities

- `html-output-directory`: HTML 打印版输出目录管理能力，包括自动创建 `html/` 子目录和路径处理

### Modified Capabilities

- `html-print-generation`: 修改输出路径，从 `outputDir` 改为 `filepath.Join(outputDir, "html")`

## Impact

### 代码影响

- **generator/html_print.go**: 修改 `generateAccountHTML` 函数的输出路径
- **cmd/generate.go**: 可能需要更新 verbose 日志输出

### 依赖影响

- 无新依赖
- 无 API 变更

### 风险

- 用户脚本可能引用旧路径 → 提示用户更新
