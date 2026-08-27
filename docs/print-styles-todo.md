# 打印版区域样式配置（待办设计）

> 状态：**设计已确认、未实施（2026-08-28）**。工作区相关代码改动已撤回。
> 当前版本**只实现**了一个小子集（见文末"已实现"），其余全部为待办。

## 目标

打印版（GL 总分类账 / ML 多科目明细账）的字体样式（字号/加粗/字体）按**区域**和**数据列**开放配置，写入 `print-config.json`（`platforms.<平台>.styles`，GL/ML 可各自覆盖 `.gl.styles` / `.ml.styles`）。

## 已确认的设计边界（用户确认，勿改）

1. **列宽、行高：不可配置**——平台补偿系数（colScale/rowScale）是核心，区域级行列宽高自定义太复杂，不做。
2. **数据区按列定义**：`month`(月/日)、`voucher`(字/号)、`summary`(摘要)、`dir`(借或贷)、**`amount`(金额区 = 覆盖所有金额列：借/贷/余/明细)**。
3. **表头所有内容可单独定义（按区域分）**，与数据区**互不影响**——即使表头格和数据格在同一列，也各自取各自的配置：
   - 标题行 `title`
   - 科目行 `subject`（GL）
   - 表头标签行子区域：`header.label`（月/日/字/号/摘要/借或贷 常规标签）、`header.amount`（**金额位数区**：十亿…分 位数标签）、`header.detail`（**ML 明细科目区**：明细 1-14 科目名表头）
4. **合计行（本月合计/本年累计/期末余额）属于数据区**，不单独设区域。
5. 样式属性：字号 `fontSize` / 加粗 `bold`（三态 null/true/false）/ 字体 `family`；`0`/空/nil = 保持现状（默认值=当前标定行为）。

## 完整设计 Schema（待办）

```json
{
  "platforms": {
    "windows": {
      "colScale": 1.1075, "rowScale": 0.992,
      "fonts": { "normal": "Calibri", "digit": "Noteworthy", "title": "仿宋", "default": "宋体" },
      "styles": {
        "title":   { "fontSize": 22, "bold": true, "family": "仿宋" },
        "subject": { "fontSize": 0, "bold": null, "family": "" },
        "header": {
          "label":  { "fontSize": 0, "bold": null, "family": "" },
          "amount": { "fontSize": 0, "bold": null, "family": "" },
          "detail": { "fontSize": 0, "bold": null, "family": "" }
        },
        "columns": {
          "month":   { "fontSize": 0, "bold": null, "family": "" },
          "voucher": { "fontSize": 0, "bold": null, "family": "" },
          "summary": { "fontSize": 0, "bold": null, "family": "" },
          "dir":     { "fontSize": 0, "bold": null, "family": "" },
          "amount":  { "fontSize": 0, "bold": null, "family": "" }
        }
      }
    }
  }
}
```

## 实现要点（待办）

- `print_config.go`：`FontStyle{fontSize,bold,family}`、`HeaderStyles{label,amount,detail}`、`StylesConfig{title,subject,header,columns}`；平台级 + GL/ML 覆盖合并（类似现有 fonts 的 mergeFonts）。
- `print_common.go`：`printSheetConfig` 加 `styles`；`applyPrintFont` 按"行区域 × 列区域"应用字体（需要 colMap 的"打印列 → 查看版列 → 列类型"映射，以及 isTitleRow/isSubjectRow 判定）。
- GL/ML：填列类型映射（month/voucher/summary/dir/detail）+ 标题/科目行判定 + 表头金额位数区/明细区判定。
- 标题字号加粗字体：`applyGLTitleArea`/`applyMLTitleArea` 硬编码 22/18 + Bold 改为读 `styles.title`（默认 22/18/仿宋）。
- 表头标签字号：现有 `labelFontSize`（GL 7 / ML 6）→ 由 `header.*` 覆盖。
- 金额数字字号/字体：现有 `dataFontSize`/`dataFontFamily` → 由 `columns.amount` 覆盖。

## 已实现（当前版本，2026-08-28）

仅：**GL/ML 的 摘要/借方/贷方/余额 表头字体样式 + 金额区域列字体样式**。
配置形态与实现细节见 `print-config.md`（fonts 扩展：labelSize/labelBold、digitSize/digitBold）。
其余（表头按区域细分、数据区各列、GL 科目行等）均为本待办。
