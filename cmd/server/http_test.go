package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/auth"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/config"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/room"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
)

func TestHTTPHealthRegisterLogin(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{
		cfg:   config.Default(),
		auth:  auth.New(db, time.Hour),
		users: &user.Service{DB: db},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/register", app.handleRegister)
	mux.HandleFunc("/v1/login", app.handleLogin)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "ok" {
		t.Fatalf("healthz %d %q", resp.StatusCode, body)
	}

	post := func(path string, v interface{}) (*http.Response, []byte) {
		raw, _ := json.Marshal(v)
		r, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, b
	}
	r, b := post("/v1/register", map[string]string{"user": "alice", "pass": "secret"})
	if r.StatusCode != 200 {
		t.Fatalf("register %d %s", r.StatusCode, b)
	}
	r, b = post("/v1/register", map[string]string{"user": "alice", "pass": "secret"})
	if r.StatusCode != 409 {
		t.Fatalf("dup %d %s", r.StatusCode, b)
	}
	r, b = post("/v1/login", map[string]string{"user": "alice", "pass": "wrong"})
	if r.StatusCode != 401 {
		t.Fatalf("bad pass %d %s", r.StatusCode, b)
	}
	r, b = post("/v1/login", map[string]string{"user": "alice", "pass": "secret"})
	if r.StatusCode != 200 {
		t.Fatalf("login %d %s", r.StatusCode, b)
	}
	var lr struct {
		UID   int64  `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b, &lr); err != nil || lr.UID == 0 || lr.Token == "" {
		t.Fatalf("login body %s", b)
	}
}

func TestMapRoomErrAndFromPBAct(t *testing.T) {
	if mapRoomErr(room.ErrPerm) != pb.Code_CODE_PERM_DENIED {
		t.Fatal("perm")
	}
	if mapRoomErr(room.ErrFull) != pb.Code_CODE_ROOM_FULL {
		t.Fatal("full")
	}
	if mapRoomErr(game.ErrBadTurn) != pb.Code_CODE_BAD_STATE {
		t.Fatal("turn")
	}
	if fromPBAct(pb.ActionType_ACT_HU) != game.ActHu {
		t.Fatal("hu")
	}
	if fromPBAct(pb.ActionType_ACT_DISCARD) != game.ActDiscard {
		t.Fatal("discard")
	}
}
