## 1. 创建 GoReleaser 配置

- [x] 1.1 创建 `.goreleaser.yml` 文件，配置版本号为 2
- [x] 1.2 配置 builds 部分：main 入口、binary 名称、CGO_ENABLED=0
- [x] 1.3 配置目标平台：linux/darwin/windows × amd64/arm64
- [x] 1.4 配置 archives 部分：tar.gz 格式，Windows 使用 zip
- [x] 1.5 配置 checksum 生成：输出 checksums.txt
- [x] 1.6 配置 changelog：排除 docs: 和 test: 前缀的提交

## 2. 创建 CI Workflow

- [x] 2.1 创建 `.github/workflows/ci.yml` 文件
- [x] 2.2 配置触发条件：PR 到 main、Push 到 main
- [x] 2.3 添加 Go 环境设置步骤：actions/setup-go@v5，go-version 1.26，启用缓存
- [x] 2.4 添加测试数据创建步骤：创建简化 vouchers 和 科目余额总览.json
- [x] 2.5 添加测试执行步骤：运行 go test ./...

## 3. 创建 Build Workflow

- [x] 3.1 创建 `.github/workflows/build.yml` 文件
- [x] 3.2 配置触发条件：Push 到 main
- [x] 3.3 添加 Go 环境设置步骤
- [x] 3.4 添加 GoReleaser snapshot 构建步骤：使用 goreleaser/goreleaser-action@v6，args: release --snapshot --clean

## 4. 创建 Release Workflow

- [x] 4.1 创建 `.github/workflows/release.yml` 文件
- [x] 4.2 配置触发条件：Tag 推送，匹配 v* 模式
- [x] 4.3 配置权限：permissions: contents: write
- [x] 4.4 添加 checkout 步骤：fetch-depth: 0 获取完整历史
- [x] 4.5 添加 Go 环境设置步骤
- [x] 4.6 添加 GoReleaser release 步骤：使用 GITHUB_TOKEN

## 5. 更新 .gitignore

- [x] 5.1 添加 `.worktrees/` 到 .gitignore

## 6. 测试验证

- [ ] 6.1 本地运行 GoReleaser snapshot 验证配置正确
- [ ] 6.2 提交并创建 PR 验证 CI workflow 运行
- [ ] 6.3 合并到 main 验证 build workflow 运行
- [ ] 6.4 创建测试 Tag 验证 release workflow 运行

---

## Post-Implementation Workflow

After completing ALL tasks above, follow this sequence strictly:

1. **Verify**: Run `/opsx:verify` to produce verify.md
2. **User Acceptance**: Present change summary, ask user to confirm the problem is solved
3. **Merge**: After user accepts, go to main branch and merge (must ask user)
4. **Archive**: Run `/opsx:archive` on main
5. **Cleanup**: `git worktree remove .worktrees/change/<name>`

**Iteration**: If user does not accept, analyze the issue and recommend:
fix in place / new change / git reset + stash / git reset / abandon.
