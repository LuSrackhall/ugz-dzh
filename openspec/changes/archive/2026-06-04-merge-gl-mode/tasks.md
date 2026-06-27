## 1. 配置数据结构

- [x] 1.1 在 GlobalSettings 中新增 MergeGLAccounts、GLSuppressAccounts、MLSuppressAccounts 字段

## 2. 合并 GL 生成

- [x] 2.1 实现 AppendMergeEntries 和 appendToMergeGLSheet
- [x] 2.2 实现 ensureMergeGLSheet
- [x] 2.3 实现 isChildOf 和 sortEntries 辅助函数

## 3. 过滤逻辑

- [x] 3.1 AppendEntries 中实现 GLSuppressAccounts 过滤
- [x] 3.2 AppendMLEntries 中实现 MLSuppressAccounts 过滤

## 4. 月结

- [x] 4.1 实现 WriteMergeGLClosings 和 writeMergeGLClosingRows

## 5. 流程接入

- [x] 5.1 GenerateWorkbook 中插入 AppendMergeEntries 调用
- [x] 5.2 GenerateWorkbook 中插入 WriteMergeGLClosings 调用

## 6. 测试

- [x] 6.1 合并 GL 基础测试（NoConfig, Basic, SummaryFormat, NoDetail, MultipleDetails）
- [x] 6.2 过滤测试（GLSuppress, MLSuppress）
- [x] 6.3 回归测试 — 全部 PASS

## 7. 验证

- [x] 7.1 新增 7 个测试 + 回归 14+ 测试 — 全部 PASS
- [x] 7.2 `go build ./...` 编译通过
