package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindUserLevelSkills 构造临时 home，验证能发现各 agent 用户级技能残留目录。
func TestFindUserLevelSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// 造两个残留：WorkBuddy 与 Claude Code
	for _, rel := range []string{
		filepath.Join(".workbuddy", "skills", "ledger-accounting"),
		filepath.Join(".claude", "skills", "ledger-accounting"),
	} {
		if err := os.MkdirAll(filepath.Join(home, rel), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	// Cursor 不造——应不出现
	got := findUserLevelSkills()
	names := map[string]bool{}
	for _, f := range got {
		names[f[0]] = true
	}
	if !names["WorkBuddy"] || !names["Claude Code"] {
		t.Errorf("findUserLevelSkills 应发现 WorkBuddy 与 Claude Code，实际: %v", got)
	}
	if names["Cursor"] {
		t.Error("Cursor 未创建却出现在结果中")
	}
	if len(got) != 2 {
		t.Errorf("应恰好 2 个残留，实际 %d: %v", len(got), got)
	}
}

// TestFindUserLevelSkillsEmpty home 无残留时返回空。
func TestFindUserLevelSkillsEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got := findUserLevelSkills(); len(got) != 0 {
		t.Errorf("空 home 应返回空，实际: %v", got)
	}
}

// TestCheckUserLevelRemnantsKeep  --keep-user-level 保留残留不删除。
func TestCheckUserLevelRemnantsKeep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	remnant := filepath.Join(home, ".workbuddy", "skills", "ledger-accounting")
	if err := os.MkdirAll(remnant, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := checkUserLevelRemnants(true); err != nil {
		t.Fatalf("checkUserLevelRemnants(keep=true): %v", err)
	}
	if _, err := os.Stat(remnant); err != nil {
		t.Error("--keep-user-level 不应删除残留")
	}
}

// TestCheckUserLevelRemnantsRemove 非终端 + 未 keep → 默认移除残留。
func TestCheckUserLevelRemnantsRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	remnant := filepath.Join(home, ".claude", "skills", "ledger-accounting")
	if err := os.MkdirAll(remnant, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := checkUserLevelRemnants(false); err != nil {
		t.Fatalf("checkUserLevelRemnants(keep=false): %v", err)
	}
	if _, err := os.Stat(remnant); !os.IsNotExist(err) {
		t.Error("非终端默认应移除残留")
	}
}
