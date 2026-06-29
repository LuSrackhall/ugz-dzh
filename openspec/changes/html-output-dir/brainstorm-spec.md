## Context

当前 HTML 打印版文件与 Excel 文件混合输出在同一个目录（如 `output/2026/`），导致目录结构混乱，不便于管理和查找。

## Goals / Non-Goals

**Goals:**
- 将 HTML 打印版输出到单独的 `html/` 子目录
- 保持现有文件命名格式不变
- 自动创建子目录（如果不存在）

**Non-Goals:**
- 不改变 Excel 文件的输出位置
- 不改变文件命名格式
- 不改变 CLI 参数

## Decisions

### 决策 1：HTML 输出到 `output/{year}/html/` 子目录

**选择**：修改 `generateAccountHTML` 函数，将输出路径从 `outputDir` 改为 `filepath.Join(outputDir, "html")`

**理由**：
- 与 Excel 文件分离，目录结构更清晰
- 保持在年份目录下，便于按年份管理
- 自动创建子目录，用户无需手动创建

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

## Risks / Trade-offs

### [路径变更] → 用户需要适应新的文件位置

如果用户有脚本引用旧路径，需要更新。但这是必要的改进，长期收益大于短期成本。
