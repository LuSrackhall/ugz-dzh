## Why

当前输出目录结构将所有月份的文件混合在同一个年份目录下（如 `output/2026/`），导致文件混乱、难以管理、无法按月备份或归档。需要改为按月分目录，符合传统手工账的按月归档习惯。

## What Changes

**BREAKING**: 修改输出目录结构，从扁平结构改为按月分目录结构。

**输出结构变化**：
```
# 之前
output/2026/
├── 2026.json
├── 2026-01.xlsx
├── 2026-01-print.xlsx
├── ledger.csv
├── ledger.xlsx
├── balance.csv
├── balance.xlsx
└── html/
    └── ...

# 之后
output/2026/
├── 2026.json
├── 2026-01/
│   ├── 2026-01.xlsx
│   ├── 2026-01-print.xlsx
│   ├── ledger.csv
│   ├── ledger.xlsx
│   ├── balance.csv
│   ├── balance.xlsx
│   └── html/
│       └── ...
└── 2026-02/
    ├── 2026-02.xlsx
    ├── 2026-02-print.xlsx
    ├── ledger.csv
    ├── ledger.xlsx
    ├── balance.csv
    ├── balance.xlsx
    └── html/
        └── ...
```

## Capabilities

### New Capabilities

- `monthly-output-directory`: 按月分目录输出能力，包括目录创建、文件路径处理、上月 xlsx 查找

### Modified Capabilities

- `excel-generation`: 修改输出路径，从 `yearDir` 改为 `monthDir`
- `html-print-generation`: 修改输出路径，从 `yearDir/html` 改为 `monthDir/html`
- `cli-commands`: 修改 `generate` 命令的输出路径逻辑

## Impact

### 代码影响

- **generator/workbook.go**: 修改 `prevMonthPath` 函数，适配新目录结构
- **generator/html_print.go**: 修改输出路径，适配新目录结构
- **cmd/generate.go**: 修改输出路径逻辑、force 级联删除逻辑

### 依赖影响

- 无新依赖
- 无 API 变更

### 风险

- 核心变更，影响范围广 → 全面的端到端测试
- 用户脚本可能引用旧路径 → 提示用户更新
