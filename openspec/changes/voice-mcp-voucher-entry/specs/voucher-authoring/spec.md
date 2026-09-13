# voucher-authoring（新增）

## ADDED Requirements

### Requirement: 凭证创建命令

系统 SHALL 提供 `ledger voucher add`，从结构化输入创建一张符合解析器格式的凭证 Markdown 文件，并写入该凭证日期所属月份的凭证目录（`vouchers/YYYY_MM/`）。

输入 SHALL 支持两种等价形态且互斥：重复 flag（`--debit <科目>=<金额>` / `--credit <科目>=<金额>`）与 `--json`（结构化分录数组，供 agent/MCP 使用）。日期与摘要为必填。

#### Scenario: 创建一张平衡的两分录凭证

- **WHEN** 用户执行 `ledger voucher add --date 2026-03-05 --summary "付养老金" --debit "公益支出-补助费用=9990.00" --credit "应付款-养老金=9990.00"`
- **THEN** 系统在 `vouchers/2026_03/` 下创建 `记字第XXXX号.md`（XXXX 为分配的凭证号）
- **AND** 该文件可被 `voucher.ParseFile` 解析出两条分录，科目、借贷方向、金额与入参一致
- **AND** 命令输出凭证号、文件路径与借贷合计

#### Scenario: 等价 JSON 入参

- **WHEN** 用户执行 `ledger voucher add --date 2026-03-05 --summary "付养老金" --json -` 并从 stdin 提供等价的分录数组
- **THEN** 生成结果与重复 flag 形态逐字节等价
- **AND** 若同时提供了重复 flag 与 `--json`，命令 MUST 报错拒绝

#### Scenario: 日期决定落盘月份

- **WHEN** 入参日期为 `2026-03-05`
- **THEN** 文件写入 `vouchers/2026_03/`，而不是当前系统月份所对应的目录

### Requirement: 落盘前强制借贷平衡校验

`voucher add` MUST 在写盘**之前**调用既有借贷平衡校验（净额口径，红字为负值参与）。不平衡时 MUST 拒绝写盘且不产生任何文件。

#### Scenario: 不平衡拒绝写盘

- **WHEN** 分录借方合计 100.00、贷方合计 90.00
- **THEN** 命令以非零码退出并报告借贷差额
- **AND** 凭证目录 MUST 不新增任何文件

#### Scenario: 红字参与净额平衡

- **WHEN** 分录为 借方 15000.00 与 贷方 15000.00 外加一条贷方红字 -5000.00 与借方红字 -5000.00
- **THEN** 校验按净额通过并正常写盘

### Requirement: 科目必须在科目树已定义

`voucher add` MUST 复用"先定义后生成"的同一套科目定义检查：入参中任一科目（全路径）未在科目树定义时，MUST 拒绝写盘并输出未定义科目清单（科目名 + 出现次数）。MUST NOT 在 `voucher add` 路径上提供 `--allow-new` 之类的自动登记逃生。

#### Scenario: 未定义科目拒绝写盘

- **WHEN** 分录使用科目 `管埋费用`（科目树未定义）
- **THEN** 命令以非零码退出并列出未定义科目清单
- **AND** 凭证目录 MUST 不新增任何文件
- **AND** 提示用户走 `subjects scan` → `subjects import` 或 `map` 纠错

### Requirement: 凭证号分配

`voucher add` SHALL 默认自动分配凭证号：扫描目标月目录内既有凭证的最大凭证号并加一。MUST 支持 `--num` 显式指定凭证号（补录场景）。显式指定的号与既有凭证冲突时 MUST 拒绝写盘。

#### Scenario: 自动分配取当月最大号加一

- **WHEN** `vouchers/2026_03/` 内已有 `记字第0001号.md` 至 `记字第0004号.md`
- **THEN** 新凭证号为 0005，文件名为 `记字第0005号.md`

#### Scenario: 显式指定号冲突拒绝

- **WHEN** 用户执行 `--num 4`，而 `记字第0004号.md` 已存在
- **THEN** 命令以非零码退出并报告凭证号冲突

### Requirement: 重试幂等

`voucher add` MUST 支持 `--idempotency-key <key>`。同一 key 在同一账套内的重复调用 MUST 返回首次创建的结果（凭证号与路径），且 MUST NOT 产生第二张凭证。

幂等索引 MUST 存放在凭证月目录**之外**（`vouchers/.voucher-keys/YYYY_MM.json`），以保证月目录内只有正式凭证 md。

#### Scenario: 同一 key 重复调用不产生第二张凭证

- **WHEN** 以 `--idempotency-key k1` 调用两次，两次入参相同
- **THEN** 第二次调用返回首次的凭证号与路径
- **AND** 目标月目录内凭证文件数量不变

### Requirement: 写入器 round-trip 自检

凭证写入器 MUST 在写盘后立即用 `voucher.ParseFile` 解析刚写出的文件，并断言解析结果与入参的日期、凭证号、科目、借贷方向、金额逐项一致。断言失败时 MUST 以非零码退出并报告差异，且 MUST NOT 保留该文件。

#### Scenario: 写入格式与解析器兼容

- **WHEN** 通过 `voucher add` 创建任意一张合法凭证
- **THEN** 解析出的分录条数、科目、方向、金额与入参完全一致
- **AND** 解析过程 MUST 不产生"凭证号未解析"告警（凭证号可解析）

#### Scenario: 自检失败不保留文件

- **WHEN** 写入器产生的文件无法被解析回等价的入参（模拟格式漂移）
- **THEN** 命令以非零码退出
- **AND** 该文件 MUST 被删除

### Requirement: 创建凭证不触发生成

`voucher add` MUST NOT 触发账本生成，也 MUST NOT 修改 `{year}.json` 或任何 xlsx/print 产物。账本生成 SHALL 保持为需用户显式调用的独立动作。

#### Scenario: 记账后账本未变

- **WHEN** 在已有账本的账套中执行一次 `voucher add`
- **THEN** 凭证 md 已新增
- **AND** 该月 xlsx 与 `{year}.json` 的修改时间与内容均不变
