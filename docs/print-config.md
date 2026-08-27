# 打印版配置文件 print-config.json

`generate` 支持通过 `--config print-config.json`（可选）配置打印版的**平台补偿系数**与**分区域字体**，用于解决 WPS 各平台/各机器渲染尺寸不一致的问题，且**无需改代码、重新编译、发版**——直接改 JSON 重新生成即可。

不传 `--config` 时使用内置默认值（与当前标定行为一致），配置完全向后兼容。

## 用法

```bash
ledger generate -v <凭证目录> -o <输出目录> --config print-config.json
```

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

### 特性

- **按平台独立**：`platforms.windows` 与 `platforms.mac` 各自完整配置（系数 + 字体），互不影响
- **部分覆盖**：只写要改的字段，其余保持默认（如只调 windows 系数，mac 与字体不动）
- **未知平台**：配置中未列出的平台（如 linux）按 mac 默认（系数 1.0）
- **目标平台选择**：`--platform auto`（默认，按当前系统）/ `--platform windows` / `--platform mac`——在 Mac 上可用 `--platform windows` 生成 Windows 版测试（配合 `scripts/gen-win-test.sh`）

## 为什么需要补偿系数

WPS 各平台/各机器渲染列宽、行高存在差异（字体环境、渲染引擎不同）：同一份打印版 xlsx 在 Windows 上实测整体偏小（表格宽约 -12.5%、行高约 -6%）。补偿系数在生成时乘到列宽/行高值上，使目标平台的输出适配其页面。

默认值（windows 1.1075 / 0.992）为多轮肉眼观察标定的收敛值；换环境后可自行调整，观察方法见 `scripts/gen-win-test.sh` 顶部说明。

## 完整示例

见 `example/print-config.json`。
