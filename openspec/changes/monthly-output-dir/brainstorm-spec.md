## Context

当前输出目录结构将所有月份的文件混合在同一个年份目录下（如 `output/2026/`），导致文件混乱、难以管理、无法按月备份或归档。

## Goals / Non-Goals

**Goals:**
- 每个月有独立的文件夹（如 `output/2026/2026-01/`）
- 保持文件名格式不变（如 `2026-01.xlsx`）
- 年份配置文件保持在年份目录（如 `output/2026/2026.json`）
- 上月 xlsx 查找逻辑适配新结构

**Non-Goals:**
- 不改变文件命名格式
- 不改变 CLI 参数
- 不改变配置文件结构

## Decisions

### 决策 1：目录结构调整

**选择**：按月份分目录，年份配置保持在年份目录

**新结构**：
```
output/2026/
├── 2026.json                    # 年份配置（保持在年份目录）
├── 2026-01/
│   ├── 2026-01.xlsx             # 查看版
│   ├── 2026-01-print.xlsx       # 打印版
│   ├── ledger.csv               # 当月分录汇总
│   ├── ledger.xlsx
│   ├── balance.csv              # 当月余额表
│   ├── balance.xlsx
│   └── html/
│       ├── 2026-01-银行存款-print.html
│       └── ...
├── 2026-02/
│   ├── 2026-02.xlsx
│   ├── 2026-02-print.xlsx
│   ├── ledger.csv
│   ├── ledger.xlsx
│   ├── balance.csv
│   ├── balance.xlsx
│   └── html/
│       └── ...
└── ...
```

**理由**：
- 年份配置文件是全局的，保持在年份目录
- 每月文件完全隔离，便于管理和备份
- 符合传统手工账的按月归档习惯

### 决策 2：上月 xlsx 查找逻辑

**选择**：修改 `prevMonthPath` 函数，适配新目录结构

**实现细节**：
```go
// 修改前
func (wb *Workbook) prevMonthPath() string {
    prev := prevMonth(wb.Month)
    if prev == "" {
        return ""
    }
    return filepath.Join(wb.OutputDir, prev+".xlsx")
}

// 修改后
func (wb *Workbook) prevMonthPath() string {
    prev := prevMonth(wb.Month)
    if prev == "" {
        return ""
    }
    // 上月 xlsx 在上月目录中
    prevDir := filepath.Join(wb.OutputDir, prev)
    return filepath.Join(prevDir, prev+".xlsx")
}
```

### 决策 3：force 参数的级联删除逻辑

**选择**：修改级联删除逻辑，适配新目录结构

**实现细节**：
- 删除当月目录及其所有内容
- 删除之后月份的目录及其所有内容

## Risks / Trade-offs

### [核心变更] → 影响范围广，需要全面测试

- 修改输出路径逻辑
- 修改上月 xlsx 查找逻辑
- 修改 force 级联删除逻辑
- 修改所有文件写入路径

**缓解措施**：
- 全面的端到端测试（4 个月份）
- 保持文件名格式不变，减少破坏性变更
- 逐步实施，先验证核心逻辑
