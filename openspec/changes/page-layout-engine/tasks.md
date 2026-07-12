## 1. LayoutEngine 包

- [ ] 1.1 创建 `generator/layout/` 包，实现 `LayoutSpec` 结构体（纸张尺寸、装订边、页间隙、每页行数、金额位数、列比例）
- [ ] 1.2 实现 `ComputeLayout(spec) -> Layout` 纯函数，计算正面/反面区域 mm 坐标、列坐标、行坐标
- [ ] 1.3 实现 mm → Excel 列宽/行高的映射函数
- [ ] 1.4 编写 LayoutEngine 单元测试（各种 spec 参数的坐标计算验证）

## 2. 总分类账标题样式实现

- [ ] 2.1 在正面区标题行写"总    分    类    账"（深绿 #006100，14pt 加粗，字体双下划线，居中，列范围由 ComputeLayout 计算）
- [ ] 2.2 在标题行右侧写"分第 n 页"（绿色，n 印章红 #CC0000，虚线下划线）和"科目名称"（印章红），使用 RichText 多色，位置和宽度由 ComputeLayout 计算
- [ ] 2.3 从 `ComputeLayout` 获取所有标题行列坐标，替换硬编码

## 3. 总分类账每页写入重写

- [ ] 3.1 重写 `appendToGLSheet`：去掉打印重复行依赖，过次页后代码写标题行 + 列标题行 + 20 行数据 + 过次页
- [ ] 3.2 过次页/承前页与数据行同高，过次页为每页数据区的最后一行（行号由 ComputeLayout 计算）
- [ ] 3.3 实现页码计数器，每新一页 `pageNum++`
- [ ] 3.4 调整 `pageStartRow` / `rowIsPageBreak` 等辅助函数适应新布局

## 4. 正面/反面并排

- [ ] 4.1 正面内容写入左半区域（Columns 中 front 列范围）
- [ ] 4.2 反面内容写入右半区域（Columns 中 back 列范围）
- [ ] 4.3 保留页间隙空列（正面和反面之间的空白列）
- [ ] 4.4 左/右各保留装订边空列（最左和最右）

## 5. 验证

- [ ] 5.1 编译通过，所有单元测试通过
- [ ] 5.2 用 test_data 生成 1-6 月，且 e2e 测试通过
- [ ] 5.3 打开 xlsx 在打印预览中确认 A4 横向排布正确

---

## Post-Implementation Workflow

<!-- DO NOT MODIFY THIS SECTION — it defines the required workflow after all tasks are complete -->

After completing ALL tasks above, follow this sequence strictly:

1. **Verify**: Run `/opsx:verify` to produce verify.md
2. **User Acceptance**: Present change summary, ask user to confirm the problem is solved
3. **Merge**: After user accepts, go to main branch and merge (must ask user)
4. **Archive**: Run `/opsx:archive` on main
5. **Cleanup**: `git worktree remove .worktrees/change/<name>`

**Iteration**: If user does not accept, analyze the issue and recommend:
fix in place / new change / git reset + stash / git reset / abandon.
