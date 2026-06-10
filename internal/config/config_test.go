package config

import "testing"

func TestLoadDefaultAPIBase(t *testing.T) {
	t.Setenv("SPOO_API_URL", "")
	cfg := Load()
	if cfg.APIBase != "https://spoo.me" {
		t.Fatalf("APIBase = %q, want https://spoo.me", cfg.APIBase)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("SPOO_API_URL", "http://localhost:8000")
	cfg := Load()
	if cfg.APIBase != "http://localhost:8000" {
		t.Fatalf("APIBase = %q, want http://localhost:8000", cfg.APIBase)
	}
}

func TestDirCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp) // honored on linux; darwin uses HOME
	t.Setenv("HOME", tmp)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("empty config dir")
	}
}
