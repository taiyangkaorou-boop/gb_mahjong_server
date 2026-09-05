package netx

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	raw, err := Encode(7, pb.Cmd_C2S_CHAT, 0, &pb.C2SChat{Text: []byte("hi")})
	if err != nil {
		t.Fatal(err)
	}
	env, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env.Seq != 7 || env.Cmd != pb.Cmd_C2S_CHAT || env.Code != 0 {
		t.Fatalf("%+v", env)
	}
	var chat pb.C2SChat
	if err := UnmarshalBody(env, &chat); err != nil {
		t.Fatal(err)
	}
	if string(chat.Text) != "hi" {
		t.Fatalf("body %q", chat.Text)
	}
}

func TestUnmarshalEmptyBody(t *testing.T) {
	env := &pb.Envelope{}
	var chat pb.C2SChat
	if err := UnmarshalBody(env, &chat); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeGarbage(t *testing.T) {
	if _, err := Decode([]byte{0xff, 0x00, 0x01}); err == nil {
		t.Fatal("want error")
	}
}

func TestRegistryBindUnbind(t *testing.T) {
	r := NewRegistry()
	c1 := &Conn{UID: 1, send: make(chan []byte, 2), dead: make(chan struct{})}
	c2 := &Conn{UID: 1, send: make(chan []byte, 2), dead: make(chan struct{})}
	if old := r.Bind(1, c1); old != nil {
		t.Fatal("first bind")
	}
	if old := r.Bind(1, c2); old != c1 {
		t.Fatal("kick old")
	}
	if r.Count() != 1 || r.Get(1) != c2 {
		t.Fatal("get")
	}
	r.Unbind(1, c1)
	if r.Get(1) != c2 {
		t.Fatal("unbind only matching conn")
	}
	r.Unbind(1, c2)
	if r.Count() != 0 {
		t.Fatal("empty")
	}
	r.Push(99, []byte("x"))
	n := 0
	r.ForEach(func(*Conn) { n++ })
	if n != 0 {
		t.Fatal("foreach")
	}
}

func TestConnSendDropsWhenFull(t *testing.T) {
	c := &Conn{send: make(chan []byte, 1), dead: make(chan struct{})}
	c.Send([]byte("a"))
	c.Send([]byte("b"))
	if len(c.send) != 1 {
		t.Fatalf("len=%d", len(c.send))
	}
}
