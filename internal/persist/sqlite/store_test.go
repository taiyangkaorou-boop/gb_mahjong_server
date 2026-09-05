package sqlite

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUsersSessionsFriends(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a, err := s.CreateUser("alice", []byte("ha"))
	if err != nil || a == 0 {
		t.Fatalf("a %d %v", a, err)
	}
	if _, err := s.CreateUser("alice", []byte("ha")); err == nil {
		t.Fatal("dup name")
	}
	b, err := s.CreateUser("bob", []byte("hb"))
	if err != nil {
		t.Fatal(err)
	}
	ua, err := s.UserByName("alice")
	if err != nil || ua.ID != a {
		t.Fatal(err)
	}
	ub, err := s.UserByID(b)
	if err != nil || ub.Name != "bob" {
		t.Fatal(err)
	}

	if err := s.PutSession("tok1", a, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	uid, exp, err := s.Session("tok1")
	if err != nil || uid != a || exp.Before(time.Now()) {
		t.Fatalf("%d %v %v", uid, exp, err)
	}
	if err := s.DeleteSessionsOf(a); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Session("tok1"); err == nil {
		t.Fatal("session should be gone")
	}

	if err := s.AddFriendRequest(a, b); err != nil {
		t.Fatal(err)
	}
	pend, err := s.PendingTo(b)
	if err != nil || len(pend) != 1 || pend[0] != a {
		t.Fatalf("%v %v", pend, err)
	}
	if err := s.AddFriends(a, b); err != nil {
		t.Fatal(err)
	}
	if !s.AreFriends(b, a) {
		t.Fatal("friends")
	}
	ids, err := s.FriendsOf(a)
	if err != nil || len(ids) != 1 || ids[0] != b {
		t.Fatalf("%v", ids)
	}
	pend, err = s.PendingTo(b)
	if err != nil || len(pend) != 0 {
		t.Fatalf("pending after accept %v", pend)
	}
}
