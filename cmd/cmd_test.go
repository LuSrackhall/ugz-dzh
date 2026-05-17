package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootCommand(t *testing.T) {
	if rootCmd.Use != "ledger" {
		t.Errorf("root command Use = %q, want %q", rootCmd.Use, "ledger")
	}
}

func TestGenerateCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"generate"})
	if cmd.Use != "generate" {
		t.Errorf("generate command Use = %q, want %q", cmd.Use, "generate")
	}
}

func TestInitCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"init"})
	if cmd.Use != "init" {
		t.Errorf("init command Use = %q, want %q", cmd.Use, "init")
	}
}

func TestCheckCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"check"})
	if cmd.Use != "check" {
		t.Errorf("check command Use = %q, want %q", cmd.Use, "check")
	}
}

func TestAddManualCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"add-manual"})
	if cmd.Use != "add-manual" {
		t.Errorf("add-manual command Use = %q, want %q", cmd.Use, "add-manual")
	}
}

func TestResetCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"reset"})
	if cmd.Use != "reset" {
		t.Errorf("reset command Use = %q, want %q", cmd.Use, "reset")
	}
}

func TestYearCloseCommandRegistered(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"year-close"})
	if cmd.Use != "year-close" {
		t.Errorf("year-close command Use = %q, want %q", cmd.Use, "year-close")
	}
}

func TestCentsToYuan(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0"},
		{100, "1.00"},
		{12345, "123.45"},
		{-500, "-5.00"},
		{99, "0.99"},
		{1, "0.01"},
	}
	for _, tt := range tests {
		got := CentsToYuan(tt.cents)
		if got != tt.want {
			t.Errorf("CentsToYuan(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

func TestCellName(t *testing.T) {
	tests := []struct {
		col, row int
		want     string
	}{
		{1, 1, "A1"},
		{7, 3, "G3"},
		{26, 1, "Z1"},
		{27, 2, "AA2"},
	}
	for _, tt := range tests {
		got := CellName(tt.col, tt.row)
		if got != tt.want {
			t.Errorf("CellName(%d, %d) = %q, want %q", tt.col, tt.row, got, tt.want)
		}
	}
}

func TestInitSubcommandOverwriteProtection(t *testing.T) {
	dir := t.TempDir()
	rootCmd.SetArgs([]string{"init", "-s", "2026-01", "-o", dir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	configPath := filepath.Join(dir, "科目余额总览.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	rootCmd.SetArgs([]string{"init", "-s", "2026-01", "-o", dir})
	if err := rootCmd.Execute(); err == nil {
		t.Error("second init should fail due to overwrite protection")
	}
}
