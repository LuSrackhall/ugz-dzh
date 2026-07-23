## Why

多科目明细账（ML）当前是左对齐连续布局，不支持物理纸正反面的跨页打印（反面 4 明细+基础列、正面 10 明细+标题）。DefaultMLSpec 已在变更 4 合入，本变更实施核心逻辑。

## What Changes

- writeMLTitle 重写为双区跨页标题（Front 区=纸正面，Back 区=纸反面）
- appendToMLSheet 重写：同一行同时写 Back 区（7基础+4明细）+ Front 区（10明细）
- 纸1正面空白占位表（标题+20行间距）
- 过次页/承前页双区写入
- 月结行双区适配

## Capabilities

- `ml-duplex-core`: 多科目明细账跨页布局核心实现

## Impact

- generator/ml_sheet.go: writeMLTitle、appendToMLSheet、writeMLPageBreakRow、writeMLCarryForwardRow 全面重写
- generator/monthly_close_ml.go: WriteMLMonthClosings 双区适配
