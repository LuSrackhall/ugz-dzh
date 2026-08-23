## 1. 记录器基础设施（generator/print_recorder.go + print_common.go）

- [ ] 1.1 定义 PageRecorder / SheetRecord / PageRecord / RowRecord 类型（含 SheetKind、RowKind 枚举）
- [ ] 1.2 实现 Record(sheet, pageNum, row) 方法：按 sheet 名自动创建 SheetRecord，按 pageNum 归入 PageRecord
- [ ] 1.3 保留 splitCNY、dividerStyles、digitColLabels、printDigitFontSize（从旧 print_convert.go 迁移到 print_common.go）
- [ ] 1.4 实现列布局计算工具：printColMap(base, moneyOffsets) → 每个逻辑 offset 对应的物理首列号（纯算术，不插列）
- [ ] 1.5 编写单测：splitCNY 表驱动、dividerStyles 断言、printColMap 计算正确性

## 2. GL 记录器集成（generator/gl_sheet.go + monthly_close.go）

- [ ] 2.1 appendToGLSheet：每条分录写入后通知 recorder（RowEntry，含 debit/credit/dir/balance）
- [ ] 2.2 writePageBreakRow：通知 recorder（RowPageBreak，含 pageDebit/pageCredit/balance）
- [ ] 2.3 writeCarryForwardRow / insertCarryForward / insertCarryForwardAtRow：通知 recorder（RowCarryForward）
- [ ] 2.4 WriteMonthClosings：本月合计/本季合计/本年累计/期末余额各通知 recorder（RowMonthlyClose 等）

## 3. ML 记录器集成（generator/ml_sheet.go + monthly_close_ml.go）

- [ ] 3.1 appendToMLSheet：每条分录通知 recorder（RowEntry，含 debit/credit/dir/balance + details[14]）
- [ ] 3.2 writeMLPageBreakRow / writeMLCarryForwardRow：通知 recorder
- [ ] 3.3 WriteMLMonthClosings：月结行通知 recorder

## 4. GL 打印渲染器（generator/print_render.go）

- [ ] 4.1 实现 RenderPrintVersion(recorder, printPath)：新建工作簿 → 遍历 recorder.sheets → 按 Kind 分发渲染 → SaveAs
- [ ] 4.2 实现 printGLSheet：列布局计算（12 小列）→ 列宽设置 → 逐页渲染标题区/表头/数据行/过次页
- [ ] 4.3 数据行渲染：RowRecord → splitCNY 写 12 小格 + 方向/摘要/日期等非金额列原样写入
- [ ] 4.4 表头渲染：大标题跨 12 列合并 + SubHeaderRow 小列标签 + 分组边框
- [ ] 4.5 边框渲染：数据行分组竖线（dividerStyles）+ 上下边框（每5行加粗、过次页红双线底边——从 RowKind 推导）
- [ ] 4.6 垂直分页符：按计算出的列位置直接写入

## 5. ML 打印渲染器（generator/print_render_ml.go）

- [ ] 5.1 实现 printMLSheet：ML 列布局计算（Back 7 金额列 + Front 10 金额列各展开 12 小列）
- [ ] 5.2 四行表头渲染：借/贷/余额 h1-h3 矩形合并、明细名 h2-h3 扩展、h4 小列标签、「( )方金/额 分析」行
- [ ] 5.3 数据行渲染：含明细 1-14 的 12 小格拆位
- [ ] 5.4 Paper1 占位页渲染（仅 Front 侧明细 5-14）
- [ ] 5.5 边框与分页符

## 6. 入口与调用点

- [ ] 6.1 GenerateWorkbook 末尾：创建 recorder → 现有管线运行（recorder 通知散布在步骤 2-3）→ wb.Save() → RenderPrintVersion
- [ ] 6.2 cmd/generate.go：删除旧 ConvertToPrint 调用，保留 print/ 级联清理逻辑
- [ ] 6.3 删除旧 print_convert.go / print_gl.go / print_ml.go / print_convert_test.go

## 7. 验证

- [ ] 7.1 `go test ./...` 全绿（含新单测）
- [ ] 7.2 `bash scripts/test-e2e.sh --skip-test` 多月生成成功
- [ ] 7.3 抽查打印版：Sheet 集合一致、行数与查看版一致、样本数字位正确、分页符位置正确
- [ ] 7.4 打开打印版目检：表头/边框/字号/版面
