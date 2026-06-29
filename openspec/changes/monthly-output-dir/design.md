## Context

当前输出目录结构将所有月份的文件混合在同一个年份目录下。需要改为按月分目录，涉及输出路径逻辑、上月 xlsx 查找逻辑、force 级联删除逻辑等核心变更。

### 技术栈
- Go 1.21+、excelize/v2、标准库
- 现有架构：generator 包负责输出，cmd 包负责 CLI 逻辑

## Goals / Non-Goals

**Goals:**
- 修改输出路径逻辑，从扁平结构改为按月分目录
- 修改上月 xlsx 查找逻辑，适配新目录结构
- 修改 force 级联删除逻辑，删除月度目录
- 保持文件名格式不变

**Non-Goals:**
- 不改变文件命名格式
- 不改变 CLI 参数
- 不改变配置文件结构

## Decisions

### 决策 1：目录结构

**选择**：按月份分目录，年份配置保持在年份目录

**新结构**：
```
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
    └── ...
```

**理由**：
- 年份配置文件是全局的，保持在年份目录
- 每月文件完全隔离，便于管理和备份

### 决策 2：输出路径修改

**选择**：在 `cmd/generate.go` 中创建月度目录，修改所有文件写入路径

**实现细节**：
```go
// 创建月度目录
monthDir := filepath.Join(yearDir, month)
os.MkdirAll(monthDir, 0o755)

// 修改文件写入路径
xlsxPath := filepath.Join(monthDir, month+".xlsx")
printXlsxPath := filepath.Join(monthDir, month+"-print.xlsx")
csvPath := filepath.Join(monthDir, "ledger.csv")
// ...
```

### 决策 3：上月 xlsx 查找逻辑

**选择**：修改 `prevMonthPath` 函数，从上月目录查找

**实现细节**：
```go
func (wb *Workbook) prevMonthPath() string {
    prev := prevMonth(wb.Month)
    if prev == "" {
        return ""
    }
    prevDir := filepath.Join(wb.OutputDir, prev)
    return filepath.Join(prevDir, prev+".xlsx")
}
```

### 决策 4：force 级联删除逻辑

**选择**：删除当月及之后月份的目录

**实现细节**：
```go
if force {
    // 删除当月及之后月份的目录
    entries, _ := os.ReadDir(yearDir)
    for _, entry := range entries {
        if entry.IsDir() && entry.Name() >= month {
            os.RemoveAll(filepath.Join(yearDir, entry.Name()))
        }
    }
}
```

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
