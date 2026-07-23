## Phase 1: ML 列区映射

- [x] 1.1 在 `ml_sheet.go` 中新增 `mlLayout`、`mlBackBaseCol`、`mlFrontDetailCol` 函数
- [x] 1.2 编译通过

## Phase 2: 空白占位表

- [ ] 2.1 重写 `writeMLTitle`：Front 区写入空白占位表（标题4行+列标题+20行间距）
- [ ] 2.2 编译通过

## Phase 3: 数据写入

- [ ] 3.1 重写 `appendToMLSheet`：同一行同时写 Back 区 + Front 区
- [ ] 3.2 逻辑页切换：过次页后→双区标题→承前页
- [ ] 3.3 `writeMLPageBreakRow` / `writeMLCarryForwardRow` 双区
- [ ] 3.4 编译通过 + 测试

## Phase 4: 月结 + 收尾

- [ ] 4.1 `WriteMLMonthClosings` 双区适配
- [ ] 4.2 最后一个逻辑页 Back 区空白收尾
- [ ] 4.3 全部测试
