package memory

import "testing"

func TestPresenceSetAndCount(t *testing.T) {
	p := NewPresence()
	if p.Online(1) || p.Count() != 0 {
		t.Fatal("empty")
	}
	p.Set(1, true)
	p.Set(2, true)
	p.Set(1, true)
	if !p.Online(1) || p.Count() != 2 {
		t.Fatalf("on count=%d", p.Count())
	}
	p.Set(1, false)
	if p.Online(1) || !p.Online(2) || p.Count() != 1 {
		t.Fatal("offline")
	}
}
