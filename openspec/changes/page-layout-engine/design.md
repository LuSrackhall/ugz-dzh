## Context

Brainstorm-spec.md 和 CLAUDE.md 宪法章节已确定整体架构（LayoutEngine 三层、物理驱动、正/反面同 Sheet）。design.md 聚焦实现层面的技术决策。

当前 `generator/gl_sheet.go` 中：
- `writeGLTitle` 使用 Excel 字体双下划线写标题
- `appendToGLSheet` 依赖打印重复行（`SetDefinedName`），硬编码列号（1~7）
- 过次页/承前页通过 `GetRows` 动态判断行号
- 无物理单位概念，所有数值凭空定义

## Goals / Non-Goals

**Goals:**
- 实现 `generator/layout/` 包：`LayoutSpec` 定义 + `ComputeLayout` 计算
- 重写 `appendToGLSheet`：去掉打印重复行依赖，过次页后代码写标题行
- 正面/反面并排同 Sheet：正面在左（占正面区列），反面在右（占反面区列）
- 所有列号/行号通过 `ComputeLayout` 计算，渲染代码只消费坐标

**Non-Goals:**
- 不改变 `generate.go` 的流程
- 不改变 `monthly_close.go` 的月结逻辑
- 不改变多科目明细账、期初表等 Sheet

## Decisions

### 决策 1：LayoutEngine 入参定义

```go
// 物理约束（永不变）
type LayoutSpec struct {
    // 纸张
    PaperWidthMM     int // 297
    PaperHeightMM    int // 210

    // 边距
    BindingMarginMM  int // 20
    PageGapMM        int // 5

    // 内容参数
    TitleRows        int // 6（含空行 + 标题 + 列标题）
    DataRowsPerPage    int // 20
    AmountDigitCount int // 12（十亿~分）

    // 列比例（按需调整——日期拆两列时改这里）
    ColumnRatios     []ColumnRatio // 名称 + 百分比
}
```

### 决策 2：ComputeLayout 产出

```go
type Layout struct {
    // 正面区域 mm 坐标
    FrontLeftMM     int
    FrontWidthMM    int
    // 反面区域
    BackLeftMM      int
    BackWidthMM     int

    // 列坐标（相对于各自区域）
    Columns         []Column   // 起始 mm + 宽度 mm

    // 行坐标
    TitleRow        int        // 标题行号
    PageNumRow      int        // "分第 n 页"行号
    AccountRow      int        // "科目名称"行号
    ColHeaderRow    int        // 列标题行号
    DataStartRow    int        // 数据开始行号
    DataRowHeightMM int        // 每行数据高度

    // Excel 映射结果
    ExcelFrontStartCol int     // 正面第 1 列
    ExcelBackStartCol  int     // 反面第 1 列
    ExcelColumns       []ExcelColumn // 列号 + 宽度（Excel 单位）
}
```

### 决策 3：每页写入流程

```
appendToGLSheet 改写为：

for each account {
    pageNum = 1
    for each batch of 20 entries {
        // 第 1 页特殊处理（无需承前页）
        if pageNum > 1 {
            writeCarryForward(layout, row)
            row += 1
        }
        // 写标题区
        writePageTitle(layout, row, pageNum, account)
        row += titleRows
        // 写列标题
        writeColumnHeaders(layout, row)
        row += 1
        // 写 20 行数据
        for i = 0; i < 20; i++ {
            writeDataRow(layout, row, entry)
            row += 1
        }
        // 写过次页
        writePageBreak(layout, row)
        row += 1
        pageNum++
    }
    // 写月结...
}
```

### 决策 4：正面/反面渲染

为简化，第一版先只布局正面区，反面区留空。确认正面布局正确后再加入反面区内容。分两阶段：

- **阶段 1**（本次变更）：实现 LayoutEngine + 正面页布局重写
- **阶段 2**（后续变更）：添加反面页（偶数页）内容，完成正/反面并排

### 决策 5：过次页/承前页

它们与数据行同高，不特殊处理。过次页是第 21 行（最后一行数据），承前页是下一页的第 1 行（标题前）。

## Risks / Trade-offs

### [Layout 坐标与 Excel 单位转换偏差] → 测试验证
先按 mm → Excel 列宽/行高公式映射，生成后手动检查 Excel 打印预览，确认 A4 一页宽度内排布正确。

### [反面页留空，用户可能疑惑] → 文档说明
阶段 1 完成后在 README 或设计文档中说明反面页二期实现。

### [重写 appendToGLSheet 破坏月结/过次页逻辑] → 保留现有逻辑参照
现有 `monthly_close.go`、`monthly_close_ml.go` 的计算不变。过次页/承前页的数据格式与现有保持一致。
