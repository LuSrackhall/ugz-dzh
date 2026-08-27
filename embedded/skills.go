// Package embedded 内嵌 ledger-accounting 会计技能文件（Go embed）。
// CLI 发布产物自带技能，`ledger install-skill` 负责安装到 WorkBuddy 技能目录供 agent 加载。
package embedded

import "embed"

// SkillFiles 嵌入 ledger-accounting 技能（SKILL.md + references/）。
// 技能源码随项目 git 管理；安装产物（如 ~/.workbuddy/skills/ledger-accounting/）可随时重建。
//
//go:embed ledger-accounting
var SkillFiles embed.FS
