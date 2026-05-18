package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	content := `
# comment
FOO=bar
EMPTY=
SPACED = value
QUOTED="hello world"
SINGLE='yes'
INLINE=value # trailing comment
export EXPORT_K=v1
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}

	t.Setenv("FOO", "already-set")
	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv error: %v", err)
	}

	if got := os.Getenv("FOO"); got != "already-set" {
		t.Fatalf("expected existing env preserved, got %q", got)
	}
	if got := os.Getenv("EMPTY"); got != "" {
		t.Fatalf("expected EMPTY to be empty string, got %q", got)
	}
	if got := os.Getenv("SPACED"); got != "value" {
		t.Fatalf("expected SPACED=value, got %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("expected QUOTED=hello world, got %q", got)
	}
	if got := os.Getenv("SINGLE"); got != "yes" {
		t.Fatalf("expected SINGLE=yes, got %q", got)
	}
	if got := os.Getenv("INLINE"); got != "value" {
		t.Fatalf("expected INLINE=value, got %q", got)
	}
	if got := os.Getenv("EXPORT_K"); got != "v1" {
		t.Fatalf("expected EXPORT_K=v1, got %q", got)
	}
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("expected missing file to be ignored, got %v", err)
	}
}
