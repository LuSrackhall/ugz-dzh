## Why

手工账电子化生成系统（ledger）已完成全部开发阶段，但缺少 CI/CD 配置。每次代码变更都需要手动运行测试和构建，无法保证代码质量，也难以快速发布新版本。添加 GitHub Actions 自动化流程可以：
- 确保每次 PR 和 Push 都经过测试验证
- 自动构建多平台二进制文件
- 通过 Git Tag 一键发布 GitHub Release

## What Changes

新增 GitHub Actions CI/CD 配置，实现自动化测试、构建和发布：

1. **新增 `.github/workflows/ci.yml`**
   - 触发：PR 和 Push 到 main
   - 内容：创建简化测试数据，运行 `go test ./...`

2. **新增 `.github/workflows/build.yml`**
   - 触发：Push 到 main
   - 内容：使用 GoReleaser snapshot 模式验证构建成功（不发布）

3. **新增 `.github/workflows/release.yml`**
   - 触发：Tag（`v*`）
   - 内容：使用 GoReleaser 创建 GitHub Release，上传多平台二进制文件

4. **新增 `.goreleaser.yml`**
   - 配置多平台构建：Linux、macOS、Windows × amd64/arm64
   - 自动生成 checksum 和 changelog
   - 输出格式：`.tar.gz`（Linux/macOS）、`.zip`（Windows）

## Capabilities

### New Capabilities
- `github-actions-ci`: 自动化测试 workflow，在 PR 和 Push 时运行单元测试
- `github-actions-build`: 构建验证 workflow，使用 GoReleaser snapshot 模式验证多平台构建
- `github-actions-release`: 发布 workflow，在创建 Git Tag 时自动发布 GitHub Release
- `goreleaser-config`: GoReleaser 配置，定义多平台构建规则和输出格式

### Modified Capabilities
（无）

## Impact

**新增文件：**
- `.github/workflows/ci.yml`
- `.github/workflows/build.yml`
- `.github/workflows/release.yml`
- `.goreleaser.yml`

**依赖变更：**
- 使用 `actions/checkout@v4`
- 使用 `actions/setup-go@v5`
- 使用 `goreleaser/goreleaser-action@v6`

**风险与缓解：**
1. **简化测试数据**：CI 中创建的简化数据无法覆盖所有边界情况 → 保留本地 e2e 测试
2. **GoReleaser 版本**：大版本升级可能破坏配置 → 使用 `~> v2` 锁定主版本
3. **GitHub Token 权限**：权限过大可能导致安全问题 → 最小化权限声明
4. **构建速度**：每次重新下载依赖 → 添加 Go 缓存
