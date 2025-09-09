package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to write a temporary config file
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestRun_ConfigFileNotFound(t *testing.T) {
	t.Parallel()

	code, addr, err := run([]string{"-c", "no_such_file.yaml"}, map[string]string{}, io.Discard, false)
	if code == 0 || err == nil {
		t.Fatalf("expected failure, got code=%d err=%v", code, err)
	}
	if addr != "" {
		t.Fatalf("expected empty addr, got %q", addr)
	}
}

func TestRun_InvalidPortEnv(t *testing.T) {
	t.Parallel()

	cfg := writeConfig(t, "routes: []\n")
	code, _, err := run([]string{"-c", cfg}, map[string]string{"PORT": "abc"}, io.Discard, false)
	if code == 0 || err == nil {
		t.Fatalf("expected invalid port error")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("expected PORT in error, got %v", err)
	}
}

func TestRun_EnvPortUsedWhenNoFlag(t *testing.T) {
	t.Parallel()

	cfg := writeConfig(t, "routes: []\n")
	code, addr, err := run([]string{"-c", cfg}, map[string]string{"PORT": "65001"}, io.Discard, false)
	if err != nil || code != 0 {
		t.Fatalf("unexpected error: code=%d err=%v", code, err)
	}
	if addr != ":65001" {
		t.Fatalf("expected :65001, got %q", addr)
	}
}

func TestRun_FlagPortOverridesEnv(t *testing.T) {
	t.Parallel()

	cfg := writeConfig(t, "routes: []\n")
	code, addr, err := run([]string{"-c", cfg, "-p", "62000"}, map[string]string{"PORT": "63000"}, io.Discard, false)
	if err != nil || code != 0 {
		t.Fatalf("unexpected error: code=%d err=%v", code, err)
	}
	if addr != ":62000" {
		t.Fatalf("expected :62000 got %q", addr)
	}
}

func TestRun_ConfigPortOverridesFlagAndEnv(t *testing.T) {
	t.Parallel()

	cfg := writeConfig(t, `port: 64000
routes:
  - responses:
      - body: ok
        code: 200
`)
	code, addr, err := run(
		[]string{"-c", cfg, "-p", "62000"},
		map[string]string{"PORT": "63000"},
		io.Discard,
		false,
	)
	if err != nil || code != 0 {
		t.Fatalf("unexpected error: code=%d err=%v", code, err)
	}
	if addr != ":64000" {
		t.Fatalf("expected :64000 got %q", addr)
	}
}

func TestRun_DecodeErrorUnknownField(t *testing.T) {
	t.Parallel()

	cfg := writeConfig(t, "unknown_field: 1\nroutes: []\n")
	code, _, err := run([]string{"-c", cfg}, map[string]string{}, io.Discard, false)
	if code == 0 || err == nil {
		t.Fatalf("expected decode failure")
	}
	// yaml KnownFields error strings vary; just ensure mentions unknown or field
	if !strings.Contains(err.Error(), "unknown") && !strings.Contains(err.Error(), "field") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
