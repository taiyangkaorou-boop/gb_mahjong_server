package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr      string
	DataDir       string
	TokenTTL      time.Duration
	ActionTimeout time.Duration
	ExtraTimeout  time.Duration
	Reconnect     time.Duration
	MaxConns      int
	MaxRooms      int
	ChatMaxBytes  int
	Pprof         bool
	GMToken       string
	LogLevel      string
}

func Default() Config {
	return Config{
		HTTPAddr:      ":8080",
		DataDir:       "./data",
		TokenTTL:      72 * time.Hour,
		ActionTimeout: 10 * time.Second,
		ExtraTimeout:  20 * time.Second,
		Reconnect:     60 * time.Second,
		MaxConns:      10000,
		MaxRooms:      2000,
		ChatMaxBytes:  64,
		Pprof:         true,
		LogLevel:      "info",
	}
}

func Load(path string) Config {
	cfg := Default()
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
		switch k {
		case "http_addr":
			cfg.HTTPAddr = v
		case "data_dir":
			cfg.DataDir = v
		case "token_ttl_hours":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.TokenTTL = time.Duration(n) * time.Hour
			}
		case "action_timeout_ms":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.ActionTimeout = time.Duration(n) * time.Millisecond
			}
		case "extra_timeout_ms":
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				cfg.ExtraTimeout = time.Duration(n) * time.Millisecond
			}
		case "reconnect_sec":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.Reconnect = time.Duration(n) * time.Second
			}
		case "max_conns":
			if n, err := strconv.Atoi(v); err == nil {
				cfg.MaxConns = n
			}
		case "max_rooms":
			if n, err := strconv.Atoi(v); err == nil {
				cfg.MaxRooms = n
			}
		case "chat_max_bytes":
			if n, err := strconv.Atoi(v); err == nil {
				cfg.ChatMaxBytes = n
			}
		case "pprof":
			cfg.Pprof = v == "true" || v == "1"
		case "gm_token":
			cfg.GMToken = v
		case "log_level":
			if v != "" {
				cfg.LogLevel = strings.ToLower(v)
			}
		}
	}
	return cfg
}
