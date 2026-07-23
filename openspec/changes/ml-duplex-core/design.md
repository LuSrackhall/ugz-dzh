## Decisions

### 决策 1：列区映射

使用 DefaultMLSpec (nCol=12)：

```go
// Back 区（反面列）：7 基础列 + 明细 1~4
backBase(i)  = lay.BackStartCol + i      // i=0..6 基础列
backDetail(i)= lay.BackStartCol + 7 + i   // i=0..3 明细1~4

// Front 区（正面列）：明细 5~14
frontDetail(i)= lay.FrontStartCol + i     // i=0..9 明细5~14
```

### 决策 2：纸1 空白正面

`writeMLTitle` 首次写入时在 Front 区输出空白占位表（标题 4 行 + 列标题 1 行 + 20 行间距）。Back 区留空。

### 决策 3：逻辑页数据写入

同一 Row：
- Back 区：日期~余额 + 明细1~4
- Front 区：明细5~14
- PageGap 列跳过

### 决策 4：过次页

过次页行写两区：Back 区写页借贷合计/余额，Front 区写明细合计。
承前页同理。

### 决策 5：结束收尾

最后一个逻辑页后面，Back 区写空白占位表。
