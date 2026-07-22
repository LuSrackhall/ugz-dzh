## Context

brainstorm-spec.md 覆盖了高层设计。本文补充实现细节。

## Goals / Non-Goals

**Goals:**
- 定义 writePageHeader 的具体实现方式
- 明确 pageStartRow 的新偏移计算公式

**Non-Goals:**
- 不重复 brainstorm-spec 的设计决策

## Decisions

### 决策 1：writePageHeader 实现

提取 `writeGLTitle` 中标题区的样式代码为独立函数 `writePageHeader(sheet, pageNum, account)`。复用现有 Layout 坐标：

```
Row N+0: pageNum 通过 RichText 写入（绿色+数字红色）
Row N+1: "总分类账" 居中 + 科目名称右侧
Row N+2: 科目名称独立行
Row N+3: [空行]
Row N+4: 列标题
```

行号计算：过次页行 row → 承前页 row+1 → writePageHeader(row+2, account, pageNum)。

### 决策 2：pageStartRow 新偏移

当前：`return i + 3`（过次页行号 + 3 = 数据起始行）
新偏移：承前页 1 行 + 标题 TitleRowCount 行 + 列标题 1 行

```go
// 过次页在 row i，承前页在 i+1，标题行从 i+2 开始
// pageStartRow = i + 2 + TitleRowCount + 1 (列标题)
return i + 2 + lay.TitleRowCount + 1
```

### 决策 3：页码初始化

```go
pageNum := 1
for _, row := range rows {
    if len(row) > lay.BindingLeftCols+2 && row[lay.BindingLeftCols+2] == pageBreakLabel {
        pageNum++
    }
}
```

### 决策 4：MergeGL 同步

MergeGL 调用同一个 `pageStartRow` 函数，偏移更新后自动适配。但 MergeGL 不写入标题行（页码非必须）。

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| pageStartRow 偏移计算错误导致分页边界变化 | 通过测试验证 1 页/2 页/20 行/21 行边界 |
| writePageHeader 行号与数据行重叠 | 每次写入后 row++ 确保不重叠 |
