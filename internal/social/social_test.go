package social

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/memory"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
)

func TestChatRateLimit(t *testing.T) {
	s := New(nil, nil, memory.NewPresence())
	now := time.Now()
	for i := 0; i < 4; i++ {
		if !s.AllowChat(1, now) {
			t.Fatalf("burst %d denied", i)
		}
	}
	if s.AllowChat(1, now) {
		t.Fatal("5th should be limited")
	}
	if !s.AllowChat(1, now.Add(time.Second)) {
		t.Fatal("after 1s should refill")
	}
}

func TestFriendAskAcceptReject(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, err := db.CreateUser("alice", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateUser("bob", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := db.CreateUser("cara", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	on := memory.NewPresence()
	on.Set(b, true)
	s := New(db, &user.Service{DB: db}, on)

	if err := s.Ask(a, a); err == nil {
		t.Fatal("self ask")
	}
	if err := s.Ask(a, b); err != nil {
		t.Fatal(err)
	}
	friends, pending, err := s.Snapshot(b)
	if err != nil || len(friends) != 0 || len(pending) != 1 || pending[0].UID != a {
		t.Fatalf("pending %+v %+v %v", friends, pending, err)
	}
	if err := s.Respond(b, a, true); err != nil {
		t.Fatal(err)
	}
	friends, pending, err = s.Snapshot(a)
	if err != nil || len(pending) != 0 || len(friends) != 1 {
		t.Fatalf("after accept friends=%+v pending=%+v err=%v", friends, pending, err)
	}
	if friends[0].UID != b || friends[0].Name != "bob" || !friends[0].Online {
		t.Fatalf("item %+v", friends[0])
	}
	if err := s.Ask(a, b); err != nil {
		t.Fatal(err)
	}
	ids := s.FriendIDs(a)
	if len(ids) != 1 || ids[0] != b {
		t.Fatalf("ids %v", ids)
	}

	if err := s.Ask(a, c); err != nil {
		t.Fatal(err)
	}
	if err := s.Respond(c, a, false); err != nil {
		t.Fatal(err)
	}
	_, pending, err = s.Snapshot(c)
	if err != nil || len(pending) != 0 {
		t.Fatalf("rejected still pending %v %v", pending, err)
	}
	if db.AreFriends(a, c) {
		t.Fatal("reject must not be friends")
	}
}
