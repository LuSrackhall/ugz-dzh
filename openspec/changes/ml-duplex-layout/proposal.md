## Why

多科目明细账的跨页布局与 GL 不同。当前 ML 是左对齐连续布局，不支持物理纸的正反面并排（反面 4 明细+基础列、正面 10 明细+标题）。需要实现非对称跨页布局以满足打印需求。

## What Changes

- ML 数据行同时写入 Back 区（7 基础列 + 4 明细）和 Front 区（10 明细）
- ML 标题区左半（Back 区）：科目名称、分第 N 页(左)、列标题
- ML 标题区右半（Front 区）：明细帐、科目名称、分第 N 页(右)、列标题
- 首张正面空白占位
- 过次页/承前页同时写入两区
- 20 行数据/逻辑页

## Capabilities

- `ml-duplex-layout`: 多科目明细账跨页布局

## Impact

- generator/ml_sheet.go: appendToMLSheet 重写，writeMLTitle 重写
- generator/layout/layout.go: 可能新增 ML 列分配常量
