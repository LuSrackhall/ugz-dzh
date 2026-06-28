## Context

手工账电子化生成系统（ledger）已完成全部 5 个开发阶段，当前没有 CI/CD 配置。项目托管在 GitHub（LuSrackhall/ugz-dzh），使用 Go 1.26.1 + excelize/v2 生成 Excel 账本。

**现状：**
- GitHub remote 已配置
- 所有测试通过（balance、cmd、generator、voucher、e2e）
- 无 `.github/workflows/` 目录
- 本地有 `test/` 和 `example/` 目录，包含真实凭证数据

**目标：** 添加 GitHub Actions CI/CD，实现自动化测试、构建和发布。

## Goals / Non-Goals

**Goals:**
- 每次 PR 和 Push 自动运行测试
- 验证多平台构建成功
- 创建 Git Tag 时自动发布 GitHub Release
- 在 CI 中使用简化测试数据，不上传本地真实凭证

**Non-Goals:**
- 不自动部署到生产环境
- 不包含代码覆盖率报告
- 不添加复杂的通知机制
- 不支持鸿蒙 NEXT 等 Go 不原生支持的平台

## Decisions

### 1. 使用 GoReleaser 处理构建和发布

**选择：** GoReleaser v2
**原因：** Go 社区标准工具，配置简单，自动生成多平台二进制、checksum 和 changelog
**替代方案：** 纯手工 GitHub Actions（更透明但代码多）

### 2. 三个 Workflow 文件分离职责

**选择：**
- `ci.yml` - 测试（PR/Push）
- `build.yml` - 验证构建（Push）
- `release.yml` - 发布（Tag）

**原因：** 职责清晰，便于维护和理解
**替代方案：** 单文件或双文件（更简单但职责混合）

### 3. CI 中创建简化测试数据

**选择：** 在 ci.yml 中动态创建简化 JSON 和 Markdown 测试数据
**原因：** 用户要求不上传本地真实凭证数据，保护隐私
**实现：**
```yaml
- name: Create test data
  run: |
    mkdir -p vouchers/2026_01
    echo "简化测试凭证" > vouchers/2026_01/01.md
    echo '{"全局设置":{"启动月":"2026-01"},"科目树":{}}' > 科目余额总览.json
```

### 4. 触发策略

**选择：**
- PR → ci.yml
- Push → ci.yml + build.yml
- Tag (v*) → ci.yml + build.yml + release.yml

**原因：** 全覆盖，确保每次变更都经过测试验证

### 5. 目标平台

**选择：** Linux、macOS、Windows × amd64/arm64
**原因：** 覆盖主流桌面平台，Go 原生支持交叉编译
**格式：** `.tar.gz`（Linux/macOS）、`.zip`（Windows）

## Risks / Trade-offs

### 1. 简化测试数据可能不够充分
**风险：** CI 中创建的简化数据无法覆盖所有边界情况
**缓解：** 保留本地 e2e 测试，定期手动运行完整测试

### 2. GoReleaser 版本升级可能破坏配置
**风险：** GoReleaser 大版本升级时配置格式可能变化
**缓解：** 使用 `~> v2` 锁定主版本，关注 changelog

### 3. GitHub Token 权限
**风险：** 权限过大可能导致安全问题
**缓解：** release.yml 中只声明 `permissions: contents: write`，最小化权限

### 4. 缺少 Go 缓存
**风险：** 每次构建都重新下载依赖，速度慢
**缓解：** 在 `actions/setup-go` 中添加 `cache: true`

---

## 文件清单

### 新增文件
1. `.github/workflows/ci.yml` - 测试 workflow
2. `.github/workflows/build.yml` - 构建验证 workflow
3. `.github/workflows/release.yml` - 发布 workflow
4. `.goreleaser.yml` - GoReleaser 配置

### 修改文件
1. `.gitignore` - 添加 `.worktrees/`（可选）
