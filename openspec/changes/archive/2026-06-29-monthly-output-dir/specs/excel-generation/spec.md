## MODIFIED Requirements

### Requirement: 生成流程编排

`GenerateWorkbook` 函数 SHALL 作为唯一入口，按序执行：加载配置 → 解析凭证 → 复制/新建工作薄 → 提取上月期末 → 生成期初表 → 追加分录 → 月结 → 标记打印 → 回写余额 → 保存。输出文件 SHALL 存放在月度目录中。

#### Scenario: 首次生成（无上月 xlsx）
- **WHEN** 不存在上月 xlsx 文件
- **THEN** 新建空白工作薄，期初从 JSON 配置的 firstRecord 获取

#### Scenario: 有上月 xlsx
- **WHEN** 存在上月 xlsx 文件
- **THEN** 从上月月度目录复制上月 xlsx 作为基础，提取各 Sheet 过次页行余额作为本月期初
