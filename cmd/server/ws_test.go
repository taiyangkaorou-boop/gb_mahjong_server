package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/auth"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/config"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/memory"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/room"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/social"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
	"google.golang.org/protobuf/proto"
)

func newWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	on := memory.NewPresence()
	users := &user.Service{DB: db}
	cfg := config.Default()
	app := &App{
		cfg:   cfg,
		auth:  auth.New(db, time.Hour),
		users: users,
		soc:   social.New(db, users, on),
		reg:   netx.NewRegistry(),
		on:    on,
	}
	app.rooms = room.NewManager(func(uid int64, cmd pb.Cmd, code int32, msg proto.Message) {
		raw, err := netx.Encode(0, cmd, code, msg)
		if err != nil {
			return
		}
		app.reg.Push(uid, raw)
	}, users, nil, time.Second, 0, 10, db.AreFriends)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/register", app.handleRegister)
	mux.HandleFunc("/v1/login", app.handleLogin)
	mux.HandleFunc("/ws", app.handleWS)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func registerLogin(t *testing.T, base, user, pass string) (int64, string) {
	t.Helper()
	post := func(path string, v interface{}) []byte {
		raw, _ := json.Marshal(v)
		resp, err := http.Post(base+path, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out bytes.Buffer
		_, _ = out.ReadFrom(resp.Body)
		if resp.StatusCode >= 300 {
			t.Fatalf("%s %s %s", path, resp.Status, out.String())
		}
		return out.Bytes()
	}
	_ = post("/v1/register", map[string]string{"user": user, "pass": pass})
	raw := post("/v1/login", map[string]string{"user": user, "pass": pass})
	var lr struct {
		UID   int64  `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &lr); err != nil || lr.Token == "" {
		t.Fatalf("login %s", raw)
	}
	return lr.UID, lr.Token
}

func dialWS(t *testing.T, httpURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(httpURL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

func sendPB(t *testing.T, ws *websocket.Conn, seq uint32, cmd pb.Cmd, body proto.Message) {
	t.Helper()
	raw, err := netx.Encode(seq, cmd, 0, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, raw); err != nil {
		t.Fatal(err)
	}
}

func readEnv(t *testing.T, ws *websocket.Conn, d time.Duration) *pb.Envelope {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(d))
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	env, err := netx.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestWSAuthFail(t *testing.T) {
	srv := newWSServer(t)
	ws := dialWS(t, srv.URL)
	sendPB(t, ws, 1, pb.Cmd_C2S_AUTH, &pb.C2SAuth{Token: "nope"})
	env := readEnv(t, ws, 2*time.Second)
	if env.Cmd != pb.Cmd_S2C_AUTH || env.Code == 0 {
		t.Fatalf("want auth fail, got cmd=%v code=%d", env.Cmd, env.Code)
	}
}

func TestWSAuthChatAndRoom(t *testing.T) {
	srv := newWSServer(t)
	_, tokA := registerLogin(t, srv.URL, "alice", "secret")
	_, tokB := registerLogin(t, srv.URL, "bob", "secret")
	a := dialWS(t, srv.URL)
	b := dialWS(t, srv.URL)
	sendPB(t, a, 1, pb.Cmd_C2S_AUTH, &pb.C2SAuth{Token: tokA})
	sendPB(t, b, 1, pb.Cmd_C2S_AUTH, &pb.C2SAuth{Token: tokB})
	ea := readEnv(t, a, 2*time.Second)
	eb := readEnv(t, b, 2*time.Second)
	if ea.Cmd != pb.Cmd_S2C_AUTH || ea.Code != 0 {
		t.Fatalf("alice auth cmd=%v code=%d", ea.Cmd, ea.Code)
	}
	if eb.Cmd != pb.Cmd_S2C_AUTH || eb.Code != 0 {
		t.Fatalf("bob auth cmd=%v code=%d", eb.Cmd, eb.Code)
	}

	sendPB(t, a, 2, pb.Cmd_C2S_CHAT, &pb.C2SChat{Text: []byte("hi")})
	got := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		env := readEnv(t, b, time.Until(deadline))
		if env.Cmd == pb.Cmd_S2C_CHAT && env.Code == 0 {
			var m pb.S2CChat
			if err := netx.UnmarshalBody(env, &m); err != nil {
				t.Fatal(err)
			}
			if string(m.Text) != "hi" {
				t.Fatalf("chat %q", m.Text)
			}
			got = true
			break
		}
	}
	if !got {
		t.Fatal("bob did not receive chat")
	}

	sendPB(t, a, 3, pb.Cmd_C2S_ROOM_CREATE, &pb.C2SRoomCreate{})
	gotRoom := false
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		env := readEnv(t, a, time.Until(deadline))
		if env.Cmd == pb.Cmd_S2C_ROOM_STATE && env.Code == 0 {
			var st pb.S2CRoomState
			if err := netx.UnmarshalBody(env, &st); err != nil {
				t.Fatal(err)
			}
			if st.RoomId == "" {
				t.Fatal("empty room id")
			}
			gotRoom = true
			break
		}
		if env.Cmd == pb.Cmd_S2C_ERROR {
			t.Fatalf("room create error code=%d", env.Code)
		}
	}
	if !gotRoom {
		t.Fatal("no room state after create")
	}
}
