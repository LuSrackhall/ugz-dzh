# ML 多科目明细账滑动窗口布局设计

## 概述

将 ML 的 Sheet 编排从对称布局（正/反页相同列）改为**滑动窗口非对称布局**：
每一「逻辑页」跨越两张物理纸的半页，左半（Paper Back）放基础列+前 4 个明细列，右半（Paper Front）放剩余明细列。

## 宪法层归属

| 逻辑 | 归属层 | 负责 |
|---|---|---|
| Back/Front 各自的列定义 | Layer 2 | `MLSpec` |
| 每页 20 行 + 过次页 | Layer 2 | 宪法第 2 章 |
| 标题/表头样式 | Layer 2 | 页内标准 |
| 滑动窗口配对 | Layer 1 | Sheet 编排 |
| 首尾占位 | Layer 1 | Sheet 编排 |
| 装订列、列间隙 | Layer 1 | Sheet 编排 |

## MLSpec 改动（Layer 2）

### 修改前

```go
type MLSpec struct {
    ColProportions []MLColProportion  // 单一套，正反共用
}
```

### 修改后

```go
type MLSpec struct {
    // 物理约束（不变）
    PaperWidthMM, PaperHeightMM float64
    LeftMarginMM, RightMarginMM float64
    PageGapMM         float64
    TitleRowCount      int
    ColHeaderRowCount  int
    DataRowsPerPage    int

    // 两套独立列比例
    BackColProportions  []MLColProportion   // 左半：7基础 + 明细1~4
    FrontColProportions []MLColProportion   // 右半：明细5~14
}
```

### 默认比例

**Back（左半）**— 7 基础列 + 4 明细列，共 11 列：

| 列名 | 比例 |
|---|---|
| 日期 | 8 |
| 凭证号 | 7 |
| 摘要 | 15 |
| 借方金额 | 10 |
| 贷方金额 | 10 |
| 方向 | 4 |
| 余额 | 8 |
| 明细1 | 6 |
| 明细2 | 6 |
| 明细3 | 6 |
| 明细4 | 6 |

**Front（右半）**— 最多 10 明细列，等宽：

| 列名 | 比例 |
|---|---|
| 明细5~14 | 各 10（等宽，动态） |

## MLLayout 改动（Layer 2 输出）

```go
type MLLayout struct {
    // 物理位置（不变）
    FrontLeftMM, FrontWidthMM float64
    PageGapLeftMM, PageGapWidthMM float64
    BackLeftMM, BackWidthMM float64

    // ⬇ 拆成两套
    BackColumns  []MLColumnPos   // 左半列位置（11 列基准）
    FrontColumns []MLColumnPos   // 右半列位置（最多 10 列，动态）

    // Excel 列映射（改意义）
    BindingLeftCols  int   // 2
    BackStartCol     int   // Front area → now means Back columns start
    PageGapStartCol  int   // 1 col gap
    FrontStartCol    int   // Back area → now means Front columns start
    BindingRightCols int   // 2
    TotalCols        int

    // 行号（不变，两侧共享）
    TitleRow, PageNumRow, AccountRow, HeaderRow, DataStartRow int
}
```

## Sheet 编排（Layer 1）— Spread 滑动窗口

### Spread 结构

```go
type Spread struct {
    StartRow     int    // 块起始行号
    HasBack      bool   // Back 区激活
    HasFront     bool   // Front 区激活
    BackPageNum  int    // Back 侧页码
    FrontPageNum int    // Front 侧页码
}
```

### 编排规则

| 块序号 | Back | Front | 说明 |
|---|---|---|---|
| 0 | 空 | Paper1 Front 标题 | 占位，无数据 |
| 1 | Paper1 Back + 数据 | Paper2 Front + 数据 | 逻辑页 1 |
| 2 | Paper2 Back + 数据 | Paper3 Front + 数据 | 逻辑页 2 |
| N-1 | Paper(N-1) Back + 数据 | PaperN Front + 数据 | 逻辑页 N-1 |
| N | PaperN Back + 数据 | 空 | 末页，无配对 |

## 列映射（Layer 2 → Excel）

### 基础列（只写 Back 侧）

| 数据 | Back 偏移 |
|---|---|
| 日期 | `BackStartCol + 0` |
| 凭证号 | `BackStartCol + 1` |
| 摘要 | `BackStartCol + 2` |
| 借方金额 | `BackStartCol + 3` |
| 贷方金额 | `BackStartCol + 4` |
| 方向 | `BackStartCol + 5` |
| 余额 | `BackStartCol + 6` |

### 明细列（按索引分流）

| 明细索引 | 侧 | 偏移 |
|---|---|---|
| i=0~3 | Back | `BackStartCol + 7 + i` |
| i=4~13 | Front | `FrontStartCol + (i-4)` |

## 过次页/承前页

贯穿两侧。Back 侧写基础列合计 + 明细1~4 净额，Front 侧写明细5~14 净额。

## 读取适配

| 函数 | 改动 |
|---|---|
| `mlHasPageBreakAt` | 检查 Back 摘要列 + Front 摘要列 |
| `mlLastPageBalance` | 只从 Back 余额列读 |
| `mlLastBreakTotals` | 只从 Back 借方/贷方列读 |
| `lastBreakDetailTotals` | Back 列读明细1~4，Front 列读明细5~14 净额 |
| `mlNextDataRow` | 按 Spread 行范围计算 |

## 月结适配

Back 侧写基础列合计明细，Front 侧写明细5~14 净额。期末余额只在 Back 侧。

## 测试策略

1. `go test ./generator/layout/...` — 新 Layout 单元测试
2. `go test ./generator/...` — AppendMLEntries 集成测试
3. `bash scripts/test-e2e.sh` — 全流程穿透
4. 目视 xlsx 确认列分拆和滚动窗口正确
