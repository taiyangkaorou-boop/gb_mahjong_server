package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
)

func TestRegisterLoginAndReject(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, time.Hour)
	if _, err := s.Register("ab", "secret"); err != ErrBadUser {
		t.Fatalf("short name: %v", err)
	}
	if _, err := s.Register("alice", "abc"); err != ErrBadUser {
		t.Fatalf("short pass: %v", err)
	}
	id, err := s.Register("alice", "secret")
	if err != nil || id == 0 {
		t.Fatalf("register %d %v", id, err)
	}
	if _, err := s.Register("alice", "secret"); err != ErrDup {
		t.Fatalf("dup %v", err)
	}
	if _, _, err := s.Login("alice", "wrong"); err != ErrAuth {
		t.Fatalf("bad pass %v", err)
	}
	uid, tok, err := s.Login("alice", "secret")
	if err != nil || uid != id || tok == "" {
		t.Fatalf("login %d %s %v", uid, tok, err)
	}
	got, err := s.Resolve(tok)
	if err != nil || got != id {
		t.Fatalf("resolve %d %v", got, err)
	}
	_, tok2, err := s.Login("alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(tok); err != ErrAuth {
		t.Fatalf("old token should die, err=%v", err)
	}
	if _, err := s.Resolve(tok2); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredToken(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, -time.Second)
	_, _ = s.Register("bob_ok", "secret")
	_, tok, err := s.Login("bob_ok", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(tok); err != ErrAuth {
		t.Fatalf("want expire, err=%v", err)
	}
}
