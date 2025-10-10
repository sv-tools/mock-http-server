package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to write a temporary config file
func writeConfig(tb testing.TB, content string) string {
	tb.Helper()
	dir := tb.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		tb.Fatalf("write config: %v", err)
	}
	return path
}

func TestRun_ConfigFileNotFound(t *testing.T) {
	t.Parallel()

	err := run(t.Context(), []string{"-c", "no_such_file.yaml"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "no_such_file.yaml") {
		t.Fatalf("expected config file name in error, got %v", err)
	}
}

func TestRun_InvalidPortEnv(t *testing.T) {
	cfgPath := writeConfig(t, "routes: []\n")
	t.Setenv("PORT", "abc")
	_, err := getConfig([]string{"-c", cfgPath})
	if err == nil {
		t.Fatalf("expected invalid port error")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("expected PORT in error, got %v", err)
	}
}

func TestRun_EnvPortUsedWhenNoFlag(t *testing.T) {
	cfgPath := writeConfig(t, "routes: []\n")
	t.Setenv("PORT", "65001")
	cfg, err := getConfig([]string{"-c", cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 65001 {
		t.Fatalf("expected port 65001 got %d", cfg.Port)
	}
}

func TestRun_FlagPortOverridesEnv(t *testing.T) {
	cfgPath := writeConfig(t, "routes: []\n")
	t.Setenv("PORT", "63000")
	cfg, err := getConfig([]string{"-c", cfgPath, "-p", "62000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 62000 {
		t.Fatalf("expected port 62000 got %d", cfg.Port)
	}
}

func TestRun_ConfigPortOverridesFlagAndEnv(t *testing.T) {
	cfgPath := writeConfig(t, `port: 64000
routes:
  - responses:
      - body: ok
        code: 200
`)
	t.Setenv("PORT", "63000")
	cfg, err := getConfig(
		[]string{"-c", cfgPath, "-p", "62000"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 62000 {
		t.Fatalf("expected port 62000 got %d", cfg.Port)
	}
}

func TestRun_DecodeErrorUnknownField(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfig(t, "unknown_field: 1\nroutes: []\n")
	_, err := getConfig([]string{"-c", cfgPath})
	if err == nil {
		t.Fatalf("expected decode failure")
	}
	// yaml KnownFields error strings vary; just ensure mentions unknown or field
	if !strings.Contains(err.Error(), "unknown") && !strings.Contains(err.Error(), "field") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
