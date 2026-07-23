## Decisions

### 决策 1：同一行双区写入

```go
// 基础列（日期~余额 + 明细 1~4）→ Back 区
wb.File.SetCellValue(sheet, cellName(lay.BackStartCol+offset, row), value)
// 明细 5~14 → Front 区
wb.File.SetCellValue(sheet, cellName(lay.FrontStartCol+offset, row), value)
```

### 决策 2：标题区实现

`writeMLCrossoverTitle` 函数写入左右两侧标题区：

```
BackStartCol 区:          FrontStartCol 区:
  Row0: 科目名称             Row0: 明 细 帐
  Row1: 分第N页(左)          Row1: 科目名称(同Row)
  Row2: (gap)               Row2: 分第N页(右)
  Row3: 列标题               Row3: 列标题
```

### 决策 3：首张正面空白

`ensureMLSheet` 首次调用时创建空白 Sheet（writeGLTitle 或等效逻辑），首行标记占位。

### 决策 4：双下划线

使用底部双线边框绘制在过次页行上方。
