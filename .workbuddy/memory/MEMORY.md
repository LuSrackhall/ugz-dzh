# 项目长期记忆（ugz-dzh 手工账电子化）

## 打印版位格输出（2026-08-24）

- 架构：**同文件复制 + 几何变换**（非记录器、非 InsertCols）。复制查看版 xlsx 为打印版，对 GL/ML Sheet delete+NewSheet+MoveSheet 重建，逐格复制（非金额复用 styleID，金额列展开 12 小列）。查看版生成代码 0 改动。
- 入口 `generator.TransformToPrint(viewPath, printPath)`，cmd/generate.go 在 GenerateWorkbook 后调用（失败仅告警）；`-f` 级联清理 `print/`。
- GL/ML 均为**多页块**结构，isLabelRow 须按块周期判定（非固定首行）：GL blockRows=(SubHeaderRow+1)+pageSize+1+BottomMarginRows；ML blockRows=DataStartRow+pageSize+1+BottomMarginRows。
- **元|角红细线在 dividerStyles 索引 9**（元=标签索引9、角=索引10）。旧 worktree-print-digit-columns 错放在索引8（未采用）。
- **excelize 字节级输出非确定**（map 迭代影响 sharedStrings/样式注册顺序）——main 自比也字节不同。验证查看版"不变"须用语义对比（单元格值多重集），勿用 cmp/字节 diff。
- **金额列空值格必须走拆位**（按 0 生成 12 空格 + 分组竖线），不能铺原样式——否则原金额格红双线左右边框污染 12 小格（双线溢出、无分组竖线、"整体金额"视觉）。判别：金额列 val=="" 但 sid!=0 → 拆位；val=="" 且 sid==0 → 跳过（未渲染侧）。
- **拆位（分组竖线）仅限数据区**（isDataRow：GL (r-7)%28<21 / ML (r-9)%30<21，=数据20行+过次页1行）。表头/标题区/下边距区的空值金额格必须铺 **amountEdgeStyle（仅继承边界：top/bottom 继承铺满、k=0/k=11 继承左右、中间格无边框）**——不能拆位（分组竖线溢出标题区）、不能整体铺原样式（红双线复制 12 条）。
- **坑：`yuanStrToCents("")` 返回 (0,nil)**——空串被当数值！用 `isNumericAmount` 判数值必须带 `val != ""`。
- 装订侧红双线在**数据列边缘**（GL col3 左/col28 右；ML 明细3右/明细4左），不在装订列本身（装订列 sid=0）。验证红双线延伸须查数据列边缘，勿查装订列。
- **坑：excelize GetRows 会裁剪末尾"仅有样式、无值"的行**（末块下边距行只有红双线无文字）→ readSheetMeta 必须向上探测尾部样式行（maxRow+1..+5 GetCellStyle!=0），否则打印版末块下边距行整行丢失（"红双线未延伸至下边距行"，只在最后一张逻辑表出现）。行数对比用 openpyxl max_row（含样式行），勿用 excelize GetRows。
- **ML 第 1 块（Paper1 r1-30）Back 侧查看版未渲染**（无边框无数据）→ 打印版镜像一致即可，**勿擅自补线**（d8e5c3a 因此被撤回）。
- **h4 标签行"装订边看不到"的真正根因（用户亲自定位，e0199c5 解决）**：12 列每列仅 ~1.08 字符宽，7pt 字写入即贴边 → 边框被字体"视觉挤掉"（实际已设置，删字即见）。**解法=减少列数**而非补线/合并区（前两个假设 d8e5c3a/755db46 均被撤回）。教训：数据存在≠渲染可见，但也要考虑**列宽 vs 字体**的物理挤压。
- **ML 金额列数参数化（e0199c5）**：借/贷/余 **11 列**（去十亿位）、明细 **10 列**（去十亿/亿，最高千万）。数据上限验证：9 个月最大金额 24,180,554.26 元（2418 万）< 明细 10 列上限 9999 万 < 借/贷/余 11 列上限 99.9 亿。**GL 仍 12 列**。API：`digitColLabels(n)`（12:十亿…分 / 11:亿…分 / 10:千万…分）、`splitCNY(cents,n)`（元对齐 n-3）；colMap 支持每列独立展开数 `splitCols`。
- **分组边框规则（用户定义，58ba4b5）**：单红线（CC0000/thin）**永远在元|角之间**（分隔符索引=元所在索引=n-4）；从元开始每三位一组（元十百|千万十万|百万千万亿），**组界加粗绿线**（006100/medium）= 分隔符索引 **yuanIdx-3/-6/-9**（12列→6,3,0；11列→5,2=千|百、百|十；10列→4,1）。**dividerStyles(n) 勿硬编码 0/3/6**（仅 12 列碰巧对）。
- **坑：样式缓存 key 必须含 n**——amountSubStyle/amountEdgeStyle 的 cache key=(styleID,k,n)，否则同一 styleID 在 GL(12列)/ML(11,10列) 下 k 相同但元位置不同，缓存串用 → 红细线错位、列内部出现不该有的红线。
- **ML 打印版新列坐标**（借/贷/余=11列、明细=10列）：明细4末格=c80（装订边左界右框）、明细5首格=c85（装订边右界左框）；装订区 c81-84。**旧 12 列坐标 c91/c96 已废弃**。
- **查看版 cols 列宽是范围化的**（excelize GetColWidth 读取正确）；**openpyxl 读列宽不可靠**（范围列返回默认 13.0）——判断列宽用 excelize。

## 工作树约定
- 打印版相关工作树：`worktree-print-fresh`（本次全新实现，已提交未 push）。旧 `worktree-print-digit-columns`（locked，记录器架构，未采用）。
