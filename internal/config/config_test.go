package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDevDefaults(t *testing.T) {
	cfg := Default()
	if cfg.HTTPAddr != ":8080" || cfg.MaxConns <= 0 || cfg.ActionTimeout != 10*time.Second || cfg.ExtraTimeout != 20*time.Second || cfg.LogLevel != "info" {
		t.Fatalf("%+v", cfg)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(p, []byte("http_addr: \":18080\"\naction_timeout_ms: 500\nextra_timeout_ms: 1500\nmax_conns: 12\ngm_token: \"gm\"\nlog_level: TRACE\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := Load(p)
	if got.HTTPAddr != ":18080" || got.ActionTimeout != 500*time.Millisecond || got.ExtraTimeout != 1500*time.Millisecond || got.MaxConns != 12 || got.GMToken != "gm" || got.LogLevel != "trace" {
		t.Fatalf("%+v", got)
	}
}
