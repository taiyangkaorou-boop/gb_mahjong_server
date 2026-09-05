package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDevDefaults(t *testing.T) {
	cfg := Default()
	if cfg.HTTPAddr != ":8080" || cfg.MaxConns <= 0 || cfg.ActionTimeout != 10*time.Second {
		t.Fatalf("%+v", cfg)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(p, []byte("http_addr: \":18080\"\naction_timeout_ms: 500\nmax_conns: 12\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := Load(p)
	if got.HTTPAddr != ":18080" || got.ActionTimeout != 500*time.Millisecond || got.MaxConns != 12 {
		t.Fatalf("%+v", got)
	}
}
