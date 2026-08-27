# 打印版配置文件 print-config.json

`generate` 支持通过 `--config print-config.json`（可选）配置打印版的**平台补偿系数**与**分区域字体**，用于解决 WPS 各平台/各机器渲染尺寸不一致的问题，且**无需改代码、重新编译、发版**——直接改 JSON 重新生成即可。

不传 `--config` 时使用内置默认值（与当前标定行为一致），配置完全向后兼容。

## 用法

```bash
ledger generate -v <凭证目录> -o <输出目录> --config print-config.json   # 显式指定
ledger generate -v <凭证目录> -o <输出目录>                                # 自动发现
```

**自动发现**：未传 `--config` 时，`generate` 自动加载**当前工作目录**下的 `print-config.json`（有则生效，无则用默认值）。把配置文件放在运行 ledger 的目录即可生效，无需传参（显式 `--config` 优先）。

## ⚠️ 常见错误（agent 操作必读）

以下情况会导致"配置不起作用"，**代码已防呆（会明确报错或自动容忍）**：

| # | 错误做法 | 后果 | 正确做法 |
|---|---|---|---|
| 1 | 配置放在**非当前工作目录**（如输出目录、凭证目录），以为会被自动加载 | 自动发现只查当前工作目录，找不到就用默认值 | 放在运行 `ledger` 的当前目录，或显式 `--config <路径>` |
| 2 | 配置用**中文键**（如 `{"平台":{...}}`、`{"列宽系数":...}`） | 旧格式已废弃；现在会**明确报错**（早期版本是静默忽略不生效） | 用英文键 `platforms.{windows,mac}.{colScale,rowScale,fonts...}` |
| 3 | 文件存成 **GBK/ANSI 编码**或 **UTF-8 with BOM** | 解析失败报错（BOM 已自动容忍；GBK 仍会报错） | 存 **UTF-8（无 BOM）**；JSON 不能有注释、尾逗号 |
| 4 | 查看**查看版** xlsx 或**旧的打印版文件** | 看不出任何变化（补偿只作用于打印版） | 看 `输出目录/<年份>/print/*.xlsx`，且是**重新 generate 后新生成**的 |

**快速自检**：`generate` 启动时会打印一行 `打印版配置: 已加载: <路径> / 已自动发现当前目录: <路径> / 默认 → 平台=.. 列宽系数=.. 行高系数=.. 字体=..`——
- 显示 `已加载` / `已自动发现当前目录` + 你配置的系数值 → 配置生效
- 显示 `默认` → 配置文件没被读取（检查放置目录 / 传 `--config`）
- 若目标平台与你预期不符，检查 `-p/--platform` 参数（Windows 上默认 auto=windows）

## 字段说明

```json
{
  "platforms": {
    "windows": {
      "colScale": 1.1075,          // 列宽补偿系数（Windows 默认 1.1075）
      "rowScale": 0.992,           // 行高补偿系数（Windows 默认 0.992）
      "fonts": {
        "normal": "Calibri",       // Normal 默认字体（列宽像素计算基准）
        "digit": "Noteworthy",     // 数据区金额数字字体
        "title": "仿宋",            // 大标题字体（总分类账/明细分户帐）
        "default": "宋体"           // 表头/标签/摘要等其余区域字体
      }
    },
    "mac": {
      "colScale": 1.0,             // Mac 恒 1.0（不补偿）
      "rowScale": 1.0,
      "fonts": {
        "normal": "Calibri",
        "digit": "Noteworthy",
        "title": "仿宋",
        "default": "宋体"
      }
    }
  }
}
```

### 各字段含义

| 字段 | 含义 | 默认值（windows/mac） |
|---|---|---|
| `platforms.<平台>.colScale` | 该平台打印版**列宽补偿系数**（乘在列宽值上，适配该平台渲染） | 1.1075 / 1.0 |
| `platforms.<平台>.rowScale` | 该平台打印版**行高补偿系数** | 0.992 / 1.0 |
| `platforms.<平台>.fonts.normal` | Normal 默认字体——**列宽像素计算的基准字体**。如需跨机器/平台统一尺寸，可让各端安装同一字体文件（如开源 Arimo）并在此统一填 `"Arimo"` | Calibri |
| `platforms.<平台>.fonts.digit` | 数据区金额数字字体 | Noteworthy |
| `platforms.<平台>.fonts.title` | 大标题字体 | 仿宋 |
| `platforms.<平台>.fonts.default` | 表头/标签/摘要等其余区域字体 | 宋体 |
| `platforms.<平台>.fonts.labelSize` | **摘要/借/贷/余额 表头**字号（0=现状：GL 7pt / ML 6pt） | 0 |
| `platforms.<平台>.fonts.labelBold` | **摘要/借/贷/余额 表头**加粗（null=现状加粗；false=不加粗） | null |
| `platforms.<平台>.fonts.digitSize` | **金额区域列**数字字号（0=现状：GL 7pt / ML 6pt） | 0 |
| `platforms.<平台>.fonts.digitBold` | **金额区域列**数字加粗（null=现状不加粗；true=加粗） | null |
| `platforms.<平台>.gl` / `.ml` | **GL（总分类账）/ ML（多科目明细账）分账本覆盖**（可选）：单独设该账本的 colScale/rowScale/fonts，未填字段回退平台级 | Windows GL: 1.13595/0.99495（2026-08-28 标定）；ML/其余: 用平台级 |

### GL/ML 分账本配置

总分类账与多科目明细账结构不同（ML 多栏 + 装订区），可独立调参：

```json
{
  "platforms": {
    "windows": {
      "colScale": 1.1075, "rowScale": 0.992,
      "fonts": { "normal": "Calibri", "digit": "Noteworthy", "title": "仿宋", "default": "宋体" },
      "gl": { "colScale": 1.3, "fonts": { "digit": "宋体" } },
      "ml": { "rowScale": 0.98 }
    }
  }
}
```

- `gl`：总分类账专用——列宽系数 1.3、数字字体宋体；行高、normal/title/default 回退平台级
- `ml`：多科目明细账专用——行高 0.98；其余回退平台级
- 只写要覆盖的字段；`"gl": {}` / `"ml": {}` 视为未配置

### 特性

- **按平台独立**：`platforms.windows` 与 `platforms.mac` 各自完整配置（系数 + 字体），互不影响
- **部分覆盖**：只写要改的字段，其余保持默认（如只调 windows 系数，mac 与字体不动）
- **未知平台**：配置中未列出的平台（如 linux）按 mac 默认（系数 1.0）
- **目标平台选择**：`--platform auto`（默认，按当前系统）/ `--platform windows` / `--platform mac`——在 Mac 上可用 `--platform windows` 生成 Windows 版测试（配合 `scripts/gen-win-test.sh`）

## 为什么需要补偿系数

WPS 各平台/各机器渲染列宽、行高存在差异（字体环境、渲染引擎不同）：同一份打印版 xlsx 在 Windows 上实测整体偏小（表格宽约 -12.5%、行高约 -6%）。补偿系数在生成时乘到列宽/行高值上，使目标平台的输出适配其页面。

默认值（windows 平台级 1.1075 / 0.992；GL 独立 1.13595 / 0.99495）为多轮肉眼观察标定的收敛值；换环境后可自行调整（GL/ML 可独立调），观察方法见 `scripts/gen-win-test.sh` 顶部说明。

## 完整示例

见 `docs/print-config.example.json`；`ledger init` 也会自动生成 `print-config.json` 模板到输出根目录。
