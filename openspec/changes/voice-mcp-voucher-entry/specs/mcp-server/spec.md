# mcp-server（新增）

## ADDED Requirements

### Requirement: MCP 服务命令与账套绑定

系统 SHALL 提供 `ledger mcp serve`，以 MCP Streamable HTTP 传输暴露一组工具。服务 MUST 在启动时解析并绑定**单一账套根目录**（输出根目录），并为整个进程生命周期固定该账套。

#### Scenario: 启动绑定账套并输出接入信息

- **WHEN** 用户执行 `ledger mcp serve -o <输出根目录>`
- **THEN** 服务启动并打印 MCP 端点 URL 与鉴权 token
- **AND** 打印的账套路径与 `-o` 一致

#### Scenario: 账套非法时拒绝启动

- **WHEN** `-o` 指向的目录不是有效账套（缺少 `{year}/{year}.json` 结构）
- **THEN** 服务拒绝启动并以非零码退出

### Requirement: 局域网绑定与鉴权

服务 SHALL 默认只绑定本机回环地址，此时 MUST 允许在无 token 情况下启动（只有本机能连，无外部暴露面）。

绑定非回环地址时，服务 MUST 具备有效 token。若未配置 token，服务 SHALL **自动生成随机 token、写入账套配置文件，并在启动时打印**，MUST NOT 拒绝启动，也 MUST NOT 使用任何固定默认值。

无鉴权绑定非回环地址 MUST 只能通过显式 `--insecure` 开启，默认关闭。

#### Scenario: 默认只绑定回环且免 token

- **WHEN** 未传 `--addr`
- **THEN** 服务仅监听回环地址
- **AND** 未配置 token 时服务正常启动

#### Scenario: 未配 token 绑局域网时自动生成并落盘

- **WHEN** 执行 `ledger mcp serve --addr 0.0.0.0:8765` 且此前未配置 token
- **THEN** 服务生成随机 token 并写入账套配置文件
- **AND** 启动输出打印该 token
- **AND** 服务正常监听，不退出
- **AND** 生成的 token MUST 不是任何固定默认值

#### Scenario: 已配置 token 时保持不变

- **WHEN** 执行 `ledger mcp serve --addr 0.0.0.0:8765` 且配置文件中已有 token
- **THEN** 服务使用既有 token 监听，MUST NOT 覆盖配置文件中的 token

#### Scenario: token 错误拒绝调用

- **WHEN** 客户端携带缺失或错误的 token 调用任一工具
- **THEN** 服务返回鉴权失败，且 MUST NOT 执行该工具的任何读写

#### Scenario: insecure 需显式开启

- **WHEN** 执行非回环绑定但未传 `--insecure` 且无法获得任何 token
- **THEN** 服务 MUST NOT 以无鉴权方式监听
- **AND** 只有在显式传入 `--insecure` 时才允许无鉴权绑定

### Requirement: 工具面白名单

服务 MUST 只暴露以下工具，且 MUST NOT 暴露账本生成、结账、年结、强制重建或任何其它写入命令：

- 只读：`subjects.list`、`subjects.match`、`voucher.list`、`voucher.check`、`ledger.check`、`ledger.query`
- 写：`voucher.add`

被禁止暴露的能力至少包括：`generate`（含 `-f`）、`lock`、`gen-close`、`year-close`、`subjects import`、`opening import`、`add-manual`、`map`，以及任何对 `{year}.json` 的写操作。

此约束是"不允许 agent 私自生成账本"的**结构性保证**：靠提示词约束为软约束，只有不暴露该能力才能使其不可能发生。

#### Scenario: 工具清单不含生成能力

- **WHEN** MCP 客户端请求工具清单
- **THEN** 返回的清单恰好为白名单中的七个工具
- **AND** 清单中不含任何生成/结账/年结/强制重建工具

#### Scenario: 调用未暴露的工具失败

- **WHEN** 客户端尝试调用 `generate`
- **THEN** 服务返回未知工具错误，且 MUST NOT 生成任何账本产物

### Requirement: 路径隔离

服务暴露的每个工具 MUST NOT 接受文件系统路径作为入参。所有路径 MUST 由服务端基于启动时绑定的账套根目录自行推导。

#### Scenario: 工具 schema 无路径参数

- **WHEN** 客户端请求任一工具的入参 schema
- **THEN** schema 中不含路径、目录或文件名类参数

#### Scenario: 越界路径不可达

- **WHEN** 客户端在参数中夹带路径片段（如科目名内含 `../`）
- **THEN** 服务以参数校验错误拒绝，且 MUST NOT 读写账套根目录之外的任何文件

### Requirement: 写操作串行化

服务 MUST 串行执行所有写操作，以保证凭证号分配的原子性（先读当月最大号再加一）。并发写入请求 MUST NOT 产生相同凭证号。

#### Scenario: 并发写入不发号冲突

- **WHEN** 两个 `voucher.add` 请求并发到达同一账套的同一月份
- **THEN** 两个凭证获得不同凭证号
- **AND** 两个文件均写入成功

### Requirement: 科目名归一化与候选提示

`subjects.match` MUST 对输入做归一化（去除首尾与内部空白、全角转半角），并按候选返回匹配的科目全路径及匹配方式。服务 MUST 只**返回候选供确认**，MUST NOT 自动替换或自动登记科目。

首版 MUST 至少支持精确匹配、归一化后精确匹配、包含匹配与编辑距离候选；同音字（拼音）候选不在首版范围。

#### Scenario: 全角与内部空格被归一化后命中

- **WHEN** 输入为 `管 理 费 用`（含内部空格）或含全角字符的等价输入
- **THEN** 返回 `管理费用` 相关科目作为候选，并标注匹配方式为归一化匹配

#### Scenario: 错字返回候选而非自动替换

- **WHEN** 输入为 `管埋费用`（同音错字）
- **THEN** 返回 `管理费用` 相关科目作为候选
- **AND** 服务 MUST NOT 据此写入任何凭证或修改科目树

#### Scenario: 无候选时明确空结果

- **WHEN** 输入与科目树中任何科目都不接近
- **THEN** 返回空候选列表，且不报错

### Requirement: 只读查询工具

`voucher.list` MUST 返回当月凭证清单（凭证号、日期、摘要、借贷合计）；`voucher.check` MUST 返回序列完整性与目录卫生检查结果；`ledger.check` 与 `ledger.query` MUST 复用既有账本检查与查询能力。全部只读工具 MUST NOT 修改账套内任何文件。

#### Scenario: 凭证清单可用于复述

- **WHEN** 客户端调用 `voucher.list` 指定某月
- **THEN** 返回该月每张凭证的凭证号、日期、摘要与借贷合计
- **AND** 账套内文件内容不变

#### Scenario: 检查结果结构化返回

- **WHEN** 客户端调用 `voucher.check`
- **THEN** 返回缺号、重号、非正式文件名三类问题的结构化清单
- **AND** 账套内文件内容不变
