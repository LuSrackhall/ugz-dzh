## Phase 1: LayoutSpec 适配

- [ ] 1.1 分析当前 LayoutSpec 中 ML 的列分配，确定是否需要新增 ML 专用 LayoutSpec
- [ ] 1.2 定义 ML 正面（10 明细列）和反面（7 基础列+4 明细列）的列映射
- [ ] 1.3 编译通过

## Phase 2: 标题区实现

- [ ] 2.1 实现 `writeMLCrossoverTitle`（左右双区标题写入）
- [ ] 2.2 编译通过

## Phase 3: 数据写入

- [ ] 3.1 重写 `appendToMLSheet`：同一行同时写入 Back 区 + Front 区
- [ ] 3.2 `writeMLPageBreakRow` / `writeMLCarryForwardRow` 双区写入
- [ ] 3.3 编译通过

## Phase 4: 过次页/承前页 + 页码

- [ ] 4.1 过次页后左侧写入承前页，右侧空白
- [ ] 4.2 页码：分第 N 页(左)(右)
- [ ] 4.3 20 行数据/逻辑页
- [ ] 4.4 测试全绿

## Phase 5: 验证

- [ ] 5.1 e2e 测试通过
- [ ] 5.2 跨年生成验证
