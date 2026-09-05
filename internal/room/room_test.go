package room

import (
	"sync"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
	"google.golang.org/protobuf/proto"
)

func TestKickAndStartPermission(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/u.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.CreateUser("a", []byte("x"))
	_, _ = db.CreateUser("b", []byte("x"))
	users := &user.Service{DB: db}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 10, func(a, b int64) bool { return true })
	id, err := m.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Join(2, id); err != nil {
		t.Fatal(err)
	}
	if err := m.Kick(2, 1); err != ErrPerm {
		t.Fatalf("member kick owner: %v", err)
	}
	if err := m.Start(2); err != ErrPerm {
		t.Fatalf("member start: %v", err)
	}
	if err := m.Kick(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.Leave(1); err != nil {
		t.Fatal(err)
	}
}

func TestCannotJoinTwoRooms(t *testing.T) {
	users := &user.Service{}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 10, nil)
	id1, err := m.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := m.Create(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Join(1, id2); err != ErrBusy {
		t.Fatalf("want busy, got %v id1=%s", err, id1)
	}
	_ = m.Leave(1)
	_ = m.Leave(2)
}

func TestStartNeedsFourReady(t *testing.T) {
	users := &user.Service{}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 10, nil)
	id, err := m.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Leave(1)
	for uid := int64(2); uid <= 4; uid++ {
		if err := m.Join(uid, id); err != nil {
			t.Fatal(err)
		}
	}
	for s, uid := range []int64{1, 2, 3, 4} {
		if err := m.Sit(uid, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Start(1); err != ErrState {
		t.Fatalf("no ready: %v", err)
	}
	for s, uid := range []int64{1, 2, 3} {
		if err := m.Ready(uid, true); err != nil {
			t.Fatalf("ready %d: %v", s, err)
		}
	}
	if err := m.Start(1); err != ErrState {
		t.Fatalf("3 ready: %v", err)
	}
	if err := m.Ready(4, true); err != nil {
		t.Fatal(err)
	}
	if err := m.Sit(2, 0); err != ErrFull {
		t.Fatalf("taken seat: %v", err)
	}
}

func TestMaxRoomsAndInvite(t *testing.T) {
	users := &user.Service{}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 1, func(a, b int64) bool { return a == b })
	id, err := m.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(2); err != ErrFull {
		t.Fatalf("max rooms: %v", err)
	}
	if err := m.Invite(1, 9); err != ErrPerm {
		t.Fatalf("non-friend invite: %v", err)
	}
	_ = id
	_ = m.Leave(1)
}

func TestDisconnectLeavesIdleRoom(t *testing.T) {
	users := &user.Service{}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 10, nil)
	if _, err := m.Create(1); err != nil {
		t.Fatal(err)
	}
	m.Disconnect(1)
	if m.RoomOf(1) != "" {
		t.Fatal("idle disconnect should leave")
	}
	if _, err := m.Create(1); err != nil {
		t.Fatal(err)
	}
	_ = m.Leave(1)
}

func fourReady(t *testing.T, m *Manager) string {
	t.Helper()
	id, err := m.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	for uid := int64(2); uid <= 4; uid++ {
		if err := m.Join(uid, id); err != nil {
			t.Fatal(err)
		}
	}
	for s, uid := range []int64{1, 2, 3, 4} {
		if err := m.Sit(uid, s); err != nil {
			t.Fatal(err)
		}
		if err := m.Ready(uid, true); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestStartDealsAndDiscard(t *testing.T) {
	users := &user.Service{}
	var mu sync.Mutex
	deals := map[int64]*pb.S2CDeal{}
	discards := 0
	m := NewManager(func(uid int64, cmd pb.Cmd, _ int32, msg proto.Message) {
		mu.Lock()
		defer mu.Unlock()
		switch cmd {
		case pb.Cmd_S2C_DEAL:
			d, ok := msg.(*pb.S2CDeal)
			if ok {
				deals[uid] = d
			}
		case pb.Cmd_S2C_GAME_EVENT:
			e, ok := msg.(*pb.S2CGameEvent)
			if ok && e.Type == pb.ActionType_ACT_DISCARD {
				discards++
			}
		}
	}, users, nil, time.Second, 10, nil)
	_ = fourReady(t, m)
	if err := m.Start(1); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	d0 := deals[1]
	nDeal := len(deals)
	mu.Unlock()
	if nDeal != 4 || d0 == nil || len(d0.Hand) < 14 {
		t.Fatalf("deals=%d banker=%v", nDeal, d0)
	}
	if err := m.Action(2, game.Action{TurnID: d0.TurnId, Type: game.ActDiscard, Tile: tile.Tile(d0.Hand[0])}); err != game.ErrNotYourTurn {
		t.Fatalf("non-banker discard: %v", err)
	}
	if err := m.Action(1, game.Action{TurnID: 0, Type: game.ActDiscard, Tile: tile.Tile(d0.Hand[len(d0.Hand)-1])}); err != game.ErrBadTurn {
		t.Fatalf("stale turn: %v", err)
	}
	x := tile.Tile(d0.Hand[len(d0.Hand)-1])
	if err := m.Action(1, game.Action{TurnID: d0.TurnId, Type: game.ActDiscard, Tile: x}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	nDisc := discards
	mu.Unlock()
	if nDisc < 4 {
		t.Fatalf("discard events %d", nDisc)
	}
	_ = m.Leave(1)
}

func TestInGameDisconnectKeepsSeat(t *testing.T) {
	users := &user.Service{}
	m := NewManager(func(int64, pb.Cmd, int32, proto.Message) {}, users, nil, time.Second, 10, nil)
	_ = fourReady(t, m)
	if err := m.Start(1); err != nil {
		t.Fatal(err)
	}
	m.Disconnect(2)
	if m.RoomOf(2) == "" {
		t.Fatal("in-game disconnect should keep the seat")
	}
	if err := m.Leave(2); err != nil {
		t.Fatal(err)
	}
	if m.RoomOf(2) == "" {
		t.Fatal("in-game leave should not unbind")
	}
	_ = m.Leave(1)
}
