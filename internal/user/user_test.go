package user

import (
	"path/filepath"
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
)

func TestNameLookup(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := db.CreateUser("carol", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	if s.Name(id) != "carol" {
		t.Fatalf("got %q", s.Name(id))
	}
	if s.Name(999) != "" {
		t.Fatal("missing should be empty")
	}
	empty := &Service{}
	if empty.Name(1) != "" {
		t.Fatal("nil db")
	}
}
