## Context

当前 HTML 打印版文件与 Excel 文件混合输出在同一个目录。需要将 HTML 输出分离到单独的 `html/` 子目录。

### 技术栈
- Go 1.21+、html/template、embed.FS
- 现有架构：generator 包负责输出

## Goals / Non-Goals

**Goals:**
- 修改 `generateAccountHTML` 函数的输出路径
- 自动创建 `html/` 子目录（如果不存在）
- 保持文件命名格式不变

**Non-Goals:**
- 不改变 Excel 文件的输出位置
- 不改变 CLI 参数
- 不改变 HTML 文件的内容和格式

## Decisions

### 决策 1：修改输出路径

**选择**：在 `generateAccountHTML` 函数中将 `outputDir` 改为 `filepath.Join(outputDir, "html")`

**理由**：
- 改动最小，只需修改一行代码
- 自动创建目录，用户无需手动操作
- 保持在年份目录下，便于按年份管理

**实现细节**：
```go
// 修改前
outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s-print.html", month, account))

// 修改后
htmlDir := filepath.Join(outputDir, "html")
os.MkdirAll(htmlDir, 0o755)
outputPath := filepath.Join(htmlDir, fmt.Sprintf("%s-%s-print.html", month, account))
```

## Risks / Trade-offs

### [路径变更] → 用户需要适应新的文件位置

如果用户有脚本引用旧路径，需要更新。但这是必要的改进，长期收益大于短期成本。
