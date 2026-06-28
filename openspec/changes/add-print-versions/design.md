## Context

### 背景
手工账电子化系统当前只生成单一版本的 Excel 文件，金额使用普通数字格式。用户需要三套输出：
- 查看版 Excel：用于计算验证
- 打印版 Excel：金额分栏，用于快速预览
- HTML 打印版：精美样式，支持正反面打印

### 技术栈
- Go 1.21+、excelize/v2、标准库
- 现有架构：voucher（解析）→ balance（余额）→ generator（输出）

### 约束
- 三套输出共用同一份数据源（`entries` 和 `initials`）
- 查看版保持现有实现不变
- 不引入新的外部依赖

---

## Goals / Non-Goals

**Goals:**
- 实现 `centsToDigits` 纯函数，将金额拆为 12 位数字
- 实现打印版 Excel 生成（金额分栏布局、美化样式、打印参数）
- 实现 HTML 打印版生成（模板、CSS 样式、正反面布局）
- 扩展 CLI 命令支持按需生成

**Non-Goals:**
- 不修改查看版的计算功能
- 不实现自动双面打印（依赖打印机驱动）
- 不支持 Web 服务（CLI 项目）

---

## Decisions

### 决策 1：模块划分

**选择**：在 generator 包内新增文件，不创建新包

**理由**：
- 打印版与查看版共享数据结构和样式函数
- generator 包已是输出层，新增打印版是自然扩展
- 避免循环依赖（generator 需要访问 voucher.Entry）

**文件结构**：
```
generator/
├── amount.go              # centsToDigits 纯函数
├── amount_test.go         # 单元测试
├── styles.go              # 共享样式定义
├── print_gl_sheet.go      # 打印版总分类账
├── print_ml_sheet.go      # 打印版多科目明细账
├── html_print.go          # HTML 模板渲染
└── templates/             # HTML 模板文件（embed.FS）
```

---

### 决策 2：金额分栏实现

**选择**：`centsToDigits(c int64) [12]int` 纯函数

**理由**：
- 纯函数易于测试，无副作用
- 返回数组而非切片，编译器可优化
- 12 位覆盖"十亿千百十万千百十元角分"

**实现细节**：
```go
// centsToDigits 将金额（分）拆为 12 位数字数组
// 返回 [十亿, 亿, 千万, 百万, 十万, 万, 千, 百, 十, 元, 角, 分]
func centsToDigits(cents int64) [12]int {
    // 负数取绝对值，方向由"方向"列标记
    // 零值所有位为 0
}
```

**单元测试覆盖**：
- 0 → [0,0,0,0,0,0,0,0,0,0,0,0]
- 999999999999（999亿9999万9999元99分）→ [9,9,9,9,9,9,9,9,9,9,9,9]
- 123456（1234.56元）→ [0,0,0,0,0,0,1,2,3,4,5,6]

---

### 决策 3：打印版 Excel 列布局

**选择**：所有金额列都分栏，共约 38 列

**总分类账列布局**：
```
A: 日期 | B: 凭证号 | C: 摘要
D-O: 借方金额(12栏：十亿~分)
P-AA: 贷方金额(12栏：十亿~分)
AB: 方向
AC-AN: 余额(12栏：十亿~分)
```

**多科目明细账列布局**：
- 左页（A-G）：日期/凭证号/摘要/借方/贷方/方向/余额（普通格式）
- 右页（H-U）：14 个明细科目（每科 12 栏分栏）

**风险**：列数多，A4 横向可能放不下
**缓解**：缩小字号（8-9pt）或使用 A3 纸张

---

### 决策 4：HTML 模板结构

**选择**：单文件 HTML，CSS 嵌入 `<style>` 标签

**理由**：
- 便于分享和归档（单个文件）
- 不依赖外部 CSS 文件路径
- 适合 CLI 项目（生成完整文件）

**模板结构**：
```html
<!DOCTYPE html>
<html>
<head>
    <style>
        @page { size: A4 landscape; margin: 10mm; }
        @page :left { margin-right: 5mm; }
        @page :right { margin-left: 5mm; }
        table { border-collapse: collapse; width: 100%; }
        td { border: 1px solid #000; padding: 2px 4px; text-align: center; }
        .amount-cell { display: inline-block; width: 1.2em; border-left: 1px solid #000; }
    </style>
</head>
<body>
    <div class="page-left">
        <!-- 左页内容 -->
    </div>
    <div class="page-right">
        <!-- 右页内容 -->
    </div>
</body>
</html>
```

---

### 决策 5：样式对齐机制

**选择**：共享 `TableStyles` 结构体，CSS 先行探索

**理由**：
- 开发阶段用 CSS 快速迭代样式
- 确定样式后通过 `TableStyles` 落地到 Excel
- 三个版本的非金额部分格式保持一致

**实现**：
```go
type TableStyles struct {
    HeaderBgColor   string
    HeaderFontBold  bool
    DataFontSize    float64
    BorderColor     string
    BorderWidth     int
}

func (s *TableStyles) ApplyToExcel(f *excelize.File, sheet string) { ... }
func (s *TableStyles) ToCSS() string { ... }
```

---

## Risks / Trade-offs

### [Excel 列宽超限] → 缩小字号或使用 A3 纸张
打印版 Excel 38 列可能放不下 A4 横向。缓解：设置字号 8-9pt，或建议用户使用 A3 纸张。

### [HTML 浏览器兼容性] → 测试主流浏览器
不同浏览器打印效果可能有差异。缓解：测试 Chrome、Firefox、Edge，使用 CSS 标准属性。

### [正反面对齐困难] → 提供打印指引
物理打印时左右页可能不对齐。缓解：提供打印操作指引，首次做样板测试。

### [三套输出数据不一致] → 共用数据源 + 单元测试
打印版数据可能错误。缓解：共用 `entries` 和 `initials`，单元测试覆盖 `centsToDigits`。

---

## Migration Plan

### 部署步骤
1. 实现 `centsToDigits` 函数和单元测试
2. 实现打印版 Excel 生成
3. 实现 HTML 打印版生成
4. 扩展 CLI 命令
5. 端到端测试

### 回滚策略
- 查看版 Excel 保持不变，可独立使用
- 新增功能通过 CLI 参数控制，不影响现有流程
- 如有问题，可临时禁用打印版生成

---

## Open Questions

1. **A3 纸张支持**：是否需要在 CLI 中添加 `-a3` 参数自动设置纸张大小？
2. **HTML 模板路径**：模板文件是否需要支持用户自定义？
3. **样式配置**：是否需要支持从 JSON 配置文件读取样式参数？
