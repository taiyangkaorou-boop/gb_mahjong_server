package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/auth"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/config"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/memory"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/room"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/social"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
	"google.golang.org/protobuf/proto"
)

type cred struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type App struct {
	cfg   config.Config
	auth  *auth.Service
	users *user.Service
	soc   *social.Service
	rooms *room.Manager
	reg   *netx.Registry
	on    *memory.Presence
}

func main() {
	cfgPath := "configs/server.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg := config.Load(cfgPath)
	if !logx.KnownLevel(cfg.LogLevel) {
		logx.Warnf("unknown log_level %q, using info", cfg.LogLevel)
	}
	logx.SetLevel(logx.ParseLevel(cfg.LogLevel))
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logx.Fatalf("mkdir data_dir=%s: %v", cfg.DataDir, err)
	}
	db, err := sqlite.Open(filepath.Join(cfg.DataDir, "gbmj.db"))
	if err != nil {
		logx.Fatalf("open db: %v", err)
	}
	defer db.Close()

	on := memory.NewPresence()
	users := &user.Service{DB: db}
	au := auth.New(db, cfg.TokenTTL)
	soc := social.New(db, users, on)
	reg := netx.NewRegistry()

	app := &App{cfg: cfg, auth: au, users: users, soc: soc, reg: reg, on: on}
	push := func(uid int64, cmd pb.Cmd, code int32, msg proto.Message) {
		raw, err := netx.Encode(0, cmd, code, msg)
		if err != nil {
			return
		}
		reg.Push(uid, raw)
	}
	app.rooms = room.NewManager(push, users, rules.NewCGOEngine(), cfg.ActionTimeout, cfg.ExtraTimeout, cfg.MaxRooms, db.AreFriends)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/register", app.handleRegister)
	mux.HandleFunc("/v1/login", app.handleLogin)
	mux.HandleFunc("/ws", app.handleWS)
	app.registerGM(mux)
	if cfg.Pprof {
		registerPprof(mux)
		logx.Infof("pprof enabled path=/debug/pprof/")
	}

	logx.Infof("listen %s data=%s log_level=%s", cfg.HTTPAddr, cfg.DataDir, logx.CurrentLevel())
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		logx.Fatalf("listen %s: %v", cfg.HTTPAddr, err)
	}
}

func registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logx.Warnf("http register method=%s", r.Method)
		http.Error(w, "method", 405)
		return
	}
	var c cred
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&c); err != nil {
		logx.Warnf("http register bad json")
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	id, err := a.auth.Register(c.User, c.Pass)
	if err == auth.ErrDup {
		writeJSON(w, 409, map[string]string{"error": "exists"})
		return
	}
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"uid": id})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logx.Warnf("http login method=%s", r.Method)
		http.Error(w, "method", 405)
		return
	}
	var c cred
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&c); err != nil {
		logx.Warnf("http login bad json")
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	uid, tok, err := a.auth.Login(c.User, c.Pass)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "auth failed"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"uid": uid, "token": tok})
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	defer logx.Recover("handleWS")
	if a.reg.Count() >= a.cfg.MaxConns {
		logx.Warnf("ws rejected: max_conns")
		http.Error(w, "full", 503)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Warnf("ws upgrade: %v", err)
		return
	}
	c := netx.NewConn(ws)
	defer c.Close()

	failAuth := func(seq uint32) {
		raw, err := netx.Encode(seq, pb.Cmd_S2C_AUTH, int32(pb.Code_CODE_AUTH_FAIL), nil)
		if err != nil {
			return
		}
		_ = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = ws.WriteMessage(websocket.BinaryMessage, raw)
	}

	_ = ws.SetReadDeadline(time.Now().Add(8 * time.Second))
	_, raw, err := ws.ReadMessage()
	if err != nil {
		logx.Warnf("ws auth read: %v", err)
		return
	}
	env, err := netx.Decode(raw)
	if err != nil || env.Cmd != pb.Cmd_C2S_AUTH {
		logx.Warnf("ws auth rejected: first packet invalid")
		failAuth(envSeq(env))
		return
	}
	var au pb.C2SAuth
	if err := netx.UnmarshalBody(env, &au); err != nil {
		logx.Warnf("ws auth rejected: bad body")
		failAuth(env.Seq)
		return
	}
	uid, err := a.auth.Resolve(au.Token)
	if err != nil {
		failAuth(env.Seq)
		return
	}
	name := a.users.Name(uid)
	c.UID, c.Name = uid, name
	go c.WriteLoop()
	if old := a.reg.Bind(uid, c); old != nil && old != c {
		logx.Warnf("ws replace uid=%d", uid)
		old.Close()
	}
	a.on.Set(uid, true)
	defer func() {
		a.reg.Unbind(uid, c)
		if a.reg.Get(uid) == nil {
			a.on.Set(uid, false)
			a.notifyFriends(uid)
			a.rooms.Disconnect(uid)
		}
	}()
	logx.Infof("ws auth uid=%d name=%s", uid, name)
	a.reply(c, env.Seq, pb.Cmd_S2C_AUTH, 0, &pb.S2CAuth{Uid: uid, Name: name})
	a.notifyFriends(uid)
	_ = a.rooms.Sync(uid)

	for {
		_ = ws.SetReadDeadline(time.Now().Add(120 * time.Second))
		_, raw, err := ws.ReadMessage()
		if err != nil {
			logx.Infof("ws disconnect uid=%d: %v", c.UID, err)
			return
		}
		env, err := netx.Decode(raw)
		if err != nil {
			continue
		}
		a.dispatch(c, env)
	}
}

func envSeq(env *pb.Envelope) uint32 {
	if env == nil {
		return 0
	}
	return env.Seq
}

func (a *App) reply(c *netx.Conn, seq uint32, cmd pb.Cmd, code int32, msg proto.Message) {
	raw, err := netx.Encode(seq, cmd, code, msg)
	if err != nil {
		return
	}
	c.Send(raw)
}

func (a *App) notifyFriends(uid int64) {
	ids := a.soc.FriendIDs(uid)
	for _, id := range ids {
		friends, pending, err := a.soc.Snapshot(id)
		if err != nil {
			logx.Errorf("ws friend snapshot uid=%d peer=%d: %v", uid, id, err)
			continue
		}
		raw, err := netx.Encode(0, pb.Cmd_S2C_FRIEND_SYNC, 0, toFriendSync(friends, pending))
		if err != nil {
			continue
		}
		a.reg.Push(id, raw)
	}
}

func toFriendSync(friends, pending []social.Item) *pb.S2CFriendSync {
	m := &pb.S2CFriendSync{}
	for _, it := range friends {
		m.Friends = append(m.Friends, &pb.FriendItem{Uid: it.UID, Name: it.Name, Online: it.Online})
	}
	for _, it := range pending {
		m.Pending = append(m.Pending, &pb.FriendItem{Uid: it.UID, Name: it.Name, Online: it.Online})
	}
	return m
}

func (a *App) dispatch(c *netx.Conn, env *pb.Envelope) {
	defer func() {
		if p := recover(); p != nil {
			logx.Errorf("panic recovered where=dispatch uid=%d cmd=%v panic=%v", c.UID, env.Cmd, p)
			a.reply(c, env.Seq, pb.Cmd_S2C_ERROR, int32(pb.Code_CODE_BAD_ACTION), nil)
		}
	}()
	logx.Tracef("ws dispatch uid=%d cmd=%v seq=%d", c.UID, env.Cmd, env.Seq)
	fail := func(code pb.Code) {
		logx.Warnf("ws reject uid=%d cmd=%v code=%v", c.UID, env.Cmd, code)
		a.reply(c, env.Seq, pb.Cmd_S2C_ERROR, int32(code), nil)
	}
	bind := func(msg proto.Message) bool {
		if err := netx.UnmarshalBody(env, msg); err != nil {
			logx.Warnf("ws unmarshal uid=%d cmd=%v: %v", c.UID, env.Cmd, err)
			fail(pb.Code_CODE_BAD_ACTION)
			return false
		}
		return true
	}
	switch env.Cmd {
	case pb.Cmd_C2S_CHAT:
		var m pb.C2SChat
		if !bind(&m) {
			return
		}
		if len(m.Text) == 0 || len(m.Text) > a.cfg.ChatMaxBytes {
			fail(pb.Code_CODE_BAD_ACTION)
			return
		}
		if !a.soc.AllowChat(c.UID, time.Now()) {
			a.reply(c, env.Seq, pb.Cmd_S2C_CHAT, int32(pb.Code_CODE_RATE_LIMITED), nil)
			return
		}
		out := &pb.S2CChat{Uid: c.UID, Text: m.Text}
		raw, err := netx.Encode(0, pb.Cmd_S2C_CHAT, 0, out)
		if err != nil {
			return
		}
		a.reg.ForEach(func(o *netx.Conn) { o.Send(raw) })
	case pb.Cmd_C2S_FRIEND_ASK:
		var m pb.C2SFriendAsk
		if !bind(&m) {
			return
		}
		if err := a.soc.Ask(c.UID, m.Peer); err != nil {
			fail(pb.Code_CODE_BAD_ACTION)
			return
		}
		a.pushFriends(c.UID)
		a.pushFriends(m.Peer)
	case pb.Cmd_C2S_FRIEND_RESP:
		var m pb.C2SFriendResp
		if !bind(&m) {
			return
		}
		if err := a.soc.Respond(c.UID, m.Peer, m.Accept); err != nil {
			fail(pb.Code_CODE_BAD_ACTION)
			return
		}
		a.pushFriends(c.UID)
		a.pushFriends(m.Peer)
	case pb.Cmd_C2S_FRIEND_LIST:
		a.pushFriends(c.UID)
	case pb.Cmd_C2S_ROOM_CREATE:
		var req pb.C2SRoomCreate
		if !bind(&req) {
			return
		}
		turn, extra := roomTimerFromCreate(req)
		_, err := a.rooms.CreateTimed(c.UID, turn, extra)
		if err != nil {
			fail(mapRoomErr(err))
			return
		}
		_ = a.rooms.Sync(c.UID)
	case pb.Cmd_C2S_ROOM_JOIN:
		var m pb.C2SRoomJoin
		if !bind(&m) {
			return
		}
		if err := a.rooms.Join(c.UID, strings.TrimSpace(m.RoomId)); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_SIT:
		var m pb.C2SRoomSit
		if !bind(&m) {
			return
		}
		if err := a.rooms.Sit(c.UID, int(m.Seat)); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_READY:
		var m pb.C2SRoomReady
		if !bind(&m) {
			return
		}
		if err := a.rooms.Ready(c.UID, m.Ready); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_KICK:
		var m pb.C2SRoomKick
		if !bind(&m) {
			return
		}
		if err := a.rooms.Kick(c.UID, m.Uid); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_INVITE:
		var m pb.C2SRoomInvite
		if !bind(&m) {
			return
		}
		if err := a.rooms.Invite(c.UID, m.Uid); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_START:
		if err := a.rooms.Start(c.UID); err != nil {
			fail(mapRoomErr(err))
		}
	case pb.Cmd_C2S_ROOM_LEAVE:
		_ = a.rooms.Leave(c.UID)
	case pb.Cmd_C2S_ACTION:
		var m pb.C2SAction
		if !bind(&m) {
			return
		}
		act := game.Action{
			TurnID: m.TurnId,
			Type:   fromPBAct(m.Type),
			Tile:   tile.Tile(uint8(m.Tile)),
			ChiMid: tile.Tile(uint8(m.ChiMid)),
		}
		if err := a.rooms.Action(c.UID, act); err != nil {
			if err == game.ErrBadTurn {
				fail(pb.Code_CODE_BAD_STATE)
				return
			}
			fail(mapRoomErr(err))
		}
	default:
		fail(pb.Code_CODE_BAD_ACTION)
	}
}

func (a *App) pushFriends(uid int64) {
	friends, pending, err := a.soc.Snapshot(uid)
	if err != nil {
		logx.Errorf("ws push friends uid=%d: %v", uid, err)
		return
	}
	raw, err := netx.Encode(0, pb.Cmd_S2C_FRIEND_SYNC, 0, toFriendSync(friends, pending))
	if err != nil {
		return
	}
	a.reg.Push(uid, raw)
}

func roomTimerFromCreate(req pb.C2SRoomCreate) (turn, extra time.Duration) {
	turn, extra = 0, -1
	if req.TurnSec > 0 {
		turn = clampSec(req.TurnSec, 3, 120)
	}
	if req.ExtraSec > 0 {
		extra = clampSec(req.ExtraSec, 1, 300)
	}
	return
}

func clampSec(v, min, max uint32) time.Duration {
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return time.Duration(v) * time.Second
}

func mapRoomErr(err error) pb.Code {
	switch err {
	case room.ErrPerm, game.ErrNotYourTurn:
		return pb.Code_CODE_PERM_DENIED
	case room.ErrNotFound:
		return pb.Code_CODE_NOT_FOUND
	case room.ErrFull:
		return pb.Code_CODE_ROOM_FULL
	case room.ErrBusy, room.ErrState, game.ErrBadState, game.ErrBadAction:
		return pb.Code_CODE_BAD_STATE
	default:
		if err == game.ErrBadTurn {
			return pb.Code_CODE_BAD_STATE
		}
		return pb.Code_CODE_BAD_ACTION
	}
}

func fromPBAct(t pb.ActionType) game.ActType {
	switch t {
	case pb.ActionType_ACT_PASS:
		return game.ActPass
	case pb.ActionType_ACT_DISCARD:
		return game.ActDiscard
	case pb.ActionType_ACT_CHI:
		return game.ActChi
	case pb.ActionType_ACT_PENG:
		return game.ActPeng
	case pb.ActionType_ACT_GANG_MING:
		return game.ActGangMing
	case pb.ActionType_ACT_GANG_AN:
		return game.ActAnGang
	case pb.ActionType_ACT_GANG_JIA:
		return game.ActJiaGang
	case pb.ActionType_ACT_HU:
		return game.ActHu
	default:
		return 0
	}
}
