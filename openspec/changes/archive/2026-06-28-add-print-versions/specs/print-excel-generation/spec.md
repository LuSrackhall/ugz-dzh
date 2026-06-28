## ADDED Requirements

### Requirement: 打印版总分类账 Sheet 生成

系统 SHALL 为每个叶子科目生成独立的打印版总分类账 Sheet，使用金额分栏展示（十亿千百十万千百十元角分）。

#### Scenario: 打印版 Sheet 命名
- **WHEN** 生成银行存款的打印版总分类账
- **THEN** Sheet 名称为 `打印-总分类账-银行存款`

#### Scenario: 列布局
- **WHEN** 生成打印版总分类账
- **THEN** A 列为日期，B 列为凭证号，C 列为摘要，D-O 列为借方金额（12栏），P-AA 列为贷方金额（12栏），AB 列为方向，AC-AN 列为余额（12栏）

#### Scenario: 金额分栏展示
- **WHEN** 某行借方金额为 123456 分
- **THEN** D-O 列分别显示 0,0,0,0,0,0,1,2,3,4,5,6

### Requirement: 打印版多科目明细账 Sheet 生成

系统 SHALL 为存在明细科目的总账科目生成打印版多科目明细账 Sheet。

#### Scenario: 明细账 Sheet 命名
- **WHEN** 生成管理费用的打印版多科目明细账
- **THEN** Sheet 名称为 `打印-多科目明细账-管理费用`

#### Scenario: 左右页布局
- **WHEN** 生成打印版多科目明细账
- **THEN** 左页（A-G）为日期/凭证号/摘要/借方/贷方/方向/余额（普通格式），右页（H-U）为明细科目金额分栏

### Requirement: 打印版样式设置

系统 SHALL 为打印版 Sheet 设置美化样式，包括全边框、居中对齐、标题加粗、表头背景色。

#### Scenario: 表头样式
- **WHEN** 生成打印版 Sheet
- **THEN** 表头行背景色为 #D9E1F2，字体加粗，居中对齐

#### Scenario: 数据行样式
- **WHEN** 生成打印版 Sheet
- **THEN** 数据行字号为 8-9pt，居中对齐，四边边框

### Requirement: 打印版页面设置

系统 SHALL 为打印版 Sheet 设置打印参数，包括横向打印、FitToWidth、重复表头行。

#### Scenario: 页面布局
- **WHEN** 生成打印版 Sheet
- **THEN** 页面方向为横向，FitToWidth=1，页边距为 1cm

#### Scenario: 重复表头
- **WHEN** 生成打印版 Sheet
- **THEN** 前 3 行（标题 + 表头 + 表头）在每页重复显示

### Requirement: 打印版月结处理

系统 SHALL 在打印版 Sheet 末尾添加"本月合计"行和"本年累计"行，使用金额分栏展示。

#### Scenario: 本月合计行
- **WHEN** 当月所有分录追加完毕
- **THEN** 末尾添加"本月合计"行，借方/贷方/余额使用分栏展示

#### Scenario: 本年累计行
- **WHEN** 当月所有分录追加完毕
- **THEN** 末尾添加"本年累计"行，借方/贷方/余额使用分栏展示
