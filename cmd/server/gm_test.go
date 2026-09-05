package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/config"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/room"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
	"google.golang.org/protobuf/proto"
)

func newGMServer(t *testing.T, token string) (*httptest.Server, *App) {
	t.Helper()
	users := &user.Service{}
	app := &App{
		cfg:   config.Config{GMToken: token, HTTPAddr: ":0"},
		users: users,
	}
	app.rooms = room.NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 0, 10, nil)
	mux := http.NewServeMux()
	app.registerGM(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(func() {
		if id := app.rooms.RoomOf(1); id != "" {
			_ = app.rooms.Leave(1)
		}
	})
	return srv, app
}

func TestGMDisabledWithoutToken(t *testing.T) {
	srv, _ := newGMServer(t, "")
	resp, err := http.Get(srv.URL + "/gm")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("disabled gm status %d", resp.StatusCode)
	}
}

func TestGMAddBotHTTP(t *testing.T) {
	srv, app := newGMServer(t, "secret")
	resp, err := http.Get(srv.URL + "/gm")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte("填充电脑")) {
		t.Fatalf("gm page %d %s", resp.StatusCode, body)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/gm/api/rooms", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("no token %d", resp.StatusCode)
	}

	id, err := app.rooms.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.rooms.Sit(1, 0); err != nil {
		t.Fatal(err)
	}

	raw, _ := json.Marshal(map[string]string{"room_id": id})
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/gm/api/addbot", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GM-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("addbot %d %s", resp.StatusCode, b)
	}
	var snap room.RoomSnap
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Added != 3 {
		t.Fatalf("added=%d body=%s", snap.Added, b)
	}
	bots := 0
	for _, s := range snap.Seats {
		if s.Bot {
			bots++
		}
	}
	if bots != 3 {
		t.Fatalf("bots=%d", bots)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/gm/api/rooms", nil)
	req.Header.Set("X-GM-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("rooms %d %s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte(id)) {
		t.Fatalf("rooms missing id %s", b)
	}
}
