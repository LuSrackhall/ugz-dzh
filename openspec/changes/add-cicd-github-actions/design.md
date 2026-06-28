## Context

手工账电子化生成系统（ledger）是一个 Go CLI 工具，用于生成 Excel 格式的会计账本。项目已完成全部开发阶段，现需要添加 GitHub Actions CI/CD 以自动化测试、构建和发布流程。

**当前状态：**
- Go 1.26.1 项目，使用 cobra CLI 框架
- 依赖 excelize/v2 生成 Excel 文件
- 测试覆盖：balance、cmd、generator、voucher、e2e
- 无现有 CI/CD 配置

## Goals / Non-Goals

**Goals:**
- 自动化测试：每次 PR 和 Push 运行单元测试
- 构建验证：Push 时验证多平台构建成功
- 自动发布：Tag 时创建 GitHub Release 并上传二进制文件
- 保护隐私：不上传本地真实凭证数据

**Non-Goals:**
- 不自动部署
- 不生成代码覆盖率报告
- 不支持 Go 不原生支持的平台（如鸿蒙 NEXT）

## Decisions

### 1. GoReleaser 作为构建工具

**选择：** GoReleaser v2
**原因：**
- Go 社区标准工具，被 Hugo、Gitea 等知名项目使用
- 配置简单，自动生成多平台二进制、checksum 和 changelog
- 支持 `--snapshot` 模式用于验证构建

**替代方案：**
- 纯手工 GitHub Actions：更透明，但需要维护更多构建逻辑
- 自定义构建脚本：灵活性高，但重复造轮子

### 2. 三文件 workflow 架构

**选择：** 分离为 `ci.yml`、`build.yml`、`release.yml`
**原因：**
- 职责清晰：测试、验证构建、发布各自独立
- 便于维护和理解触发逻辑
- 可以独立禁用或修改某个 workflow

**触发策略：**
```
PR        → ci.yml
Push      → ci.yml + build.yml
Tag (v*)  → ci.yml + build.yml + release.yml
```

### 3. CI 中创建简化测试数据

**选择：** 在 ci.yml 中动态创建简化 JSON 和 Markdown 测试数据
**原因：**
- 用户要求不上传本地真实凭证数据
- 简化数据足以验证基本功能
- 保留本地 e2e 测试覆盖完整场景

**实现：**
```yaml
- name: Create test data
  run: |
    mkdir -p vouchers/2026_01
    echo "简化测试凭证" > vouchers/2026_01/01.md
    echo '{"全局设置":{"启动月":"2026-01"},"科目树":{}}' > 科目余额总览.json
```

### 4. 多平台构建配置

**选择：** Linux、macOS、Windows × amd64/arm64
**原因：**
- 覆盖主流桌面平台
- Go 原生支持交叉编译，无需额外工具链
- 输出格式：`.tar.gz`（Linux/macOS）、`.zip`（Windows）

**GoReleaser 配置要点：**
- `CGO_ENABLED=0`：禁用 CGO，确保静态链接
- `--clean`：清理旧构建产物
- 自动生成 checksums.txt 和 changelog

### 5. GitHub Token 权限最小化

**选择：** release.yml 中声明 `permissions: contents: write`
**原因：**
- 遵循最小权限原则
- 只有发布 workflow 需要写权限
- 其他 workflow 使用默认只读权限

## Risks / Trade-offs

### 1. 简化测试数据的局限性
**风险：** CI 中创建的简化数据无法覆盖所有边界情况
**缓解：**
- 保留本地 e2e 测试，定期手动运行完整测试
- 简化数据验证基本功能，完整测试由开发者负责

### 2. GoReleaser 版本升级
**风险：** GoReleaser 大版本升级时配置格式可能变化
**缓解：**
- 使用 `~> v2` 锁定主版本
- 关注 GoReleaser changelog
- 配置文件版本化管理

### 3. GitHub Actions 依赖链
**风险：** 第三方 Action（如 actions/checkout）可能引入供应链攻击
**缓解：**
- 使用官方维护的 Action
- 锁定主版本（如 `@v4`）
- 定期审查依赖更新

### 4. 构建速度
**风险：** 每次构建重新下载依赖，速度慢
**缓解：**
- 在 `actions/setup-go` 中添加 `cache: true`
- Go 模块缓存会自动加速后续构建

## Migration Plan

### 部署步骤
1. 创建分支 `change/add-cicd-github-actions`
2. 添加四个新文件（三个 workflow + .goreleaser.yml）
3. 提交并创建 PR
4. CI 自动运行验证
5. 合并到 main
6. 创建第一个 Git Tag 测试发布流程

### 回滚策略
- 删除 `.github/workflows/` 目录和 `.goreleaser.yml`
- 无状态影响，可随时回滚

## Open Questions

1. **测试数据完整性**：简化测试数据是否足够覆盖主要功能路径？可能需要后续迭代优化。
2. **版本号策略**：是否需要自动从 Git Tag 提取版本号？当前使用 GoReleaser 默认行为。
3. **Changelog 格式**：是否需要自定义 changelog 生成规则？当前使用默认过滤（排除 docs/test commit）。
