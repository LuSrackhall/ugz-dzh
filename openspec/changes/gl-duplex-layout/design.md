## Context

brainstorm-spec.md 覆盖了高层设计。本文补充实现细节。

## Decisions

### 决策 1：dataCol 辅助函数

新增全局辅助函数，在所有需要根据奇偶页选择写入列的 Renderer 函数中使用：

```go
func dataCol(lay layout.Layout, pageNum, offset int) int {
    if pageNum%2 == 1 {
        return lay.FrontStartCol + offset
    }
    return lay.BackStartCol + offset
}
```

### 决策 2：偶数页标题写入

`writePageHeader` 接收 pageNum 参数判断奇偶。奇偶页的标题文字一致，仅写入列范围不同：

- 奇数页：标题占用 `FrontStartCol ~ FrontStartCol + nCol - 1`
- 偶数页：标题占用 `BackStartCol ~ BackStartCol + nCol - 1`

### 决策 3：月结和合并 GL 同步

`WriteMonthClosings` 和 `WriteMergeGLClosings` 写入关账行时，行位置已经在 Sheet 末尾，应根据 Sheet 实际状态判断（通过 `nextDataRowAfterBreak` 定位），无需关心奇偶页——它们总是在数据末尾追加。

月结行写入的列号需要通过 `dataCol` 计算。

### 决策 4：打印标记列

偶数页的打印标记列位置 = `BackStartCol + len(lay.ExcelColumns)`。

### 决策 5：pageNum 传递

`appendToGLSheet` 已有 pageNum 变量。需要将 pageNum 传递给所有需要奇偶感知的函数：
- `writePageHeader` — 已有
- `writePageBreakRow` / `writeCarryForwardRow` — 新增 pageNum 参数
- `insertCarryForward` — 新增 pageNum 参数
- `markRowForPrint` — 新增 pageNum 参数
