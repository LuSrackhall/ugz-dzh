package embedded

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillMirrorSync embedded 技能副本必须与 .agents 源逐字一致。
// B1 教训（2026-09-04 评审）：技能同步提交只改了 .agents 侧，embedded 副本
// 停留在 0.8.4——发布二进制自带旧技能，明文写着与"先定义后生成"收紧相反的
// 指导，而 doctor 只比对自写 VERSION 文件对此失明。此测试从结构上防再犯。
func TestSkillMirrorSync(t *testing.T) {
	root := ".." // 测试工作目录 = embedded/，仓库根为上一级

	// 正向：embedded 里的每个文件都必须与 .agents 源逐字一致
	err := fs.WalkDir(SkillFiles, "ledger-accounting", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		want, err := fs.ReadFile(SkillFiles, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(root, ".agents", "skills", filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("embedded 文件 %s 在 .agents 源中不存在: %w", path, err)
		}
		if string(got) != string(want) {
			return fmt.Errorf("技能副本漂移: %s 与 .agents 源不一致——先同步再发版", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// 反向：.agents 源的每个文件（VERSION 为 install 产物除外）都必须存在于 embedded
	agentsRoot := filepath.Join(root, ".agents", "skills", "ledger-accounting")
	err = filepath.WalkDir(agentsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(agentsRoot, path)
		if err != nil {
			return err
		}
		if rel == "VERSION" || strings.HasSuffix(rel, "VERSION") {
			return nil
		}
		if _, err := fs.Stat(SkillFiles, filepath.Join("ledger-accounting", filepath.ToSlash(rel))); err != nil {
			return fmt.Errorf(".agents 源文件 %s 缺失于 embedded 副本——先同步再发版", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
