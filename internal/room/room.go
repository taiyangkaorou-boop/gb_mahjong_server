package room

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
	"google.golang.org/protobuf/proto"
)

var (
	ErrFull     = errors.New("room full")
	ErrPerm     = errors.New("permission denied")
	ErrNotFound = errors.New("not found")
	ErrState    = errors.New("bad state")
	ErrBusy     = errors.New("already in room")
)

type PushFunc func(uid int64, cmd pb.Cmd, code int32, msg proto.Message)

type Manager struct {
	mu       sync.Mutex
	rooms    map[string]*Room
	byUser   map[int64]string
	push     PushFunc
	users    *user.Service
	eng      rules.Engine
	timeout  time.Duration
	maxRooms int
	friends  func(a, b int64) bool
}

func NewManager(push PushFunc, users *user.Service, eng rules.Engine, timeout time.Duration, maxRooms int, friends func(a, b int64) bool) *Manager {
	return &Manager{
		rooms:    map[string]*Room{},
		byUser:   map[int64]string{},
		push:     push,
		users:    users,
		eng:      eng,
		timeout:  timeout,
		maxRooms: maxRooms,
		friends:  friends,
	}
}

func (m *Manager) RoomOf(uid int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byUser[uid]
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}

func (m *Manager) Create(uid int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byUser[uid] != "" {
		return "", ErrBusy
	}
	if len(m.rooms) >= m.maxRooms {
		return "", ErrFull
	}
	id := m.newID()
	r := newRoom(id, uid, m)
	m.rooms[id] = r
	m.byUser[uid] = id
	go r.loop()
	return id, nil
}

func (m *Manager) Join(uid int64, id string) error {
	m.mu.Lock()
	if m.byUser[uid] != "" && m.byUser[uid] != id {
		m.mu.Unlock()
		return ErrBusy
	}
	r := m.rooms[id]
	m.mu.Unlock()
	if r == nil {
		return ErrNotFound
	}
	return r.submit(cmd{kind: cmdJoin, uid: uid})
}

func (m *Manager) bindLocked(uid int64, id string) {
	m.byUser[uid] = id
}

func (m *Manager) unbindLocked(uid int64, id string) {
	if m.byUser[uid] == id {
		delete(m.byUser, uid)
	}
}

func (m *Manager) drop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rooms[id]
	if r == nil {
		return
	}
	for i := 0; i < 4; i++ {
		if r.seats[i] != 0 {
			m.unbindLocked(r.seats[i], id)
		}
	}
	m.unbindLocked(r.owner, id)
	delete(m.rooms, id)
}

func (m *Manager) newID() string {
	for {
		var b [4]byte
		_, _ = rand.Read(b[:])
		n := binary.BigEndian.Uint32(b[:]) % 1000000
		id := sprintf6(n)
		if m.rooms[id] == nil {
			return id
		}
	}
}

func sprintf6(n uint32) string {
	s := [6]byte{'0', '0', '0', '0', '0', '0'}
	for i := 5; n > 0 && i >= 0; i-- {
		s[i] = byte('0' + n%10)
		n /= 10
	}
	return string(s[:])
}

func (m *Manager) Sit(uid int64, seat int) error {
	return m.fwd(uid, cmd{kind: cmdSit, uid: uid, seat: seat})
}
func (m *Manager) Ready(uid int64, ready bool) error {
	return m.fwd(uid, cmd{kind: cmdReady, uid: uid, ready: ready})
}
func (m *Manager) Kick(uid, target int64) error {
	return m.fwd(uid, cmd{kind: cmdKick, uid: uid, target: target})
}
func (m *Manager) Start(uid int64) error {
	return m.fwd(uid, cmd{kind: cmdStart, uid: uid})
}
func (m *Manager) Leave(uid int64) error {
	return m.fwd(uid, cmd{kind: cmdLeave, uid: uid})
}
func (m *Manager) Action(uid int64, act game.Action) error {
	return m.fwd(uid, cmd{kind: cmdAct, uid: uid, act: act})
}
func (m *Manager) Invite(uid, peer int64) error {
	return m.fwd(uid, cmd{kind: cmdInvite, uid: uid, target: peer})
}
func (m *Manager) Sync(uid int64) error {
	return m.fwd(uid, cmd{kind: cmdSync, uid: uid})
}
func (m *Manager) Disconnect(uid int64) {
	_ = m.fwd(uid, cmd{kind: cmdDisconnect, uid: uid})
}

func (m *Manager) fwd(uid int64, c cmd) error {
	m.mu.Lock()
	id := m.byUser[uid]
	r := m.rooms[id]
	m.mu.Unlock()
	if r == nil {
		return ErrNotFound
	}
	return r.submit(c)
}

const (
	cmdJoin = iota + 1
	cmdSit
	cmdReady
	cmdKick
	cmdStart
	cmdLeave
	cmdAct
	cmdInvite
	cmdSync
	cmdDisconnect
)

type cmd struct {
	kind   int
	uid    int64
	seat   int
	target int64
	ready  bool
	act    game.Action
	errc   chan error
}

type Room struct {
	id      string
	owner   int64
	seats   [4]int64
	ready   [4]bool
	inbox   chan cmd
	quit    chan struct{}
	table   *game.Table
	mgr     *Manager
	members map[int64]struct{}
}

func newRoom(id string, owner int64, mgr *Manager) *Room {
	r := &Room{id: id, owner: owner, inbox: make(chan cmd, 256), quit: make(chan struct{}), mgr: mgr, members: map[int64]struct{}{owner: {}}}
	return r
}

func (r *Room) submit(c cmd) error {
	c.errc = make(chan error, 1)
	select {
	case <-r.quit:
		return ErrNotFound
	case r.inbox <- c:
		return <-c.errc
	}
}

func (r *Room) loop() {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.quit:
			return
		case c := <-r.inbox:
			c.errc <- r.handle(c)
		case now := <-tick.C:
			if r.table != nil && r.table.Finished == nil {
				evs, err := r.table.Tick(now)
				if err != nil {
					log.Printf("room %s tick: %v", r.id, err)
					continue
				}
				r.emit(evs)
				if r.table.Finished != nil {
					r.table = nil
					r.clearReady()
					r.broadcastState("")
				}
			}
		}
	}
}

func (r *Room) handle(c cmd) error {
	switch c.kind {
	case cmdJoin:
		return r.join(c.uid)
	case cmdSit:
		return r.sit(c.uid, c.seat)
	case cmdReady:
		return r.setReady(c.uid, c.ready)
	case cmdKick:
		return r.kick(c.uid, c.target)
	case cmdStart:
		return r.start(c.uid)
	case cmdLeave:
		return r.leave(c.uid)
	case cmdAct:
		return r.act(c.uid, c.act)
	case cmdInvite:
		return r.invite(c.uid, c.target)
	case cmdSync:
		if r.table != nil {
			for i := 0; i < 4; i++ {
				if r.seats[i] == c.uid {
					r.table.Seats[i].Hosted = false
				}
			}
		}
		r.sendState(c.uid, "")
		return nil
	case cmdDisconnect:
		if r.table != nil {
			r.host(c.uid)
			return nil
		}
		return r.leave(c.uid)
	default:
		return ErrState
	}
}

func (r *Room) join(uid int64) error {
	if len(r.members) >= 8 {
		return ErrFull
	}
	r.members[uid] = struct{}{}
	r.mgr.mu.Lock()
	r.mgr.bindLocked(uid, r.id)
	r.mgr.mu.Unlock()
	r.broadcastState("")
	return nil
}

func (r *Room) sit(uid int64, seat int) error {
	if seat < 0 || seat > 3 {
		return ErrState
	}
	if r.table != nil {
		return ErrState
	}
	if _, ok := r.members[uid]; !ok {
		r.members[uid] = struct{}{}
	}
	for i := 0; i < 4; i++ {
		if r.seats[i] == uid {
			r.seats[i] = 0
			r.ready[i] = false
		}
	}
	if r.seats[seat] != 0 && r.seats[seat] != uid {
		return ErrFull
	}
	r.seats[seat] = uid
	r.mgr.mu.Lock()
	r.mgr.bindLocked(uid, r.id)
	r.mgr.mu.Unlock()
	r.broadcastState("")
	return nil
}

func (r *Room) setReady(uid int64, ready bool) error {
	if r.table != nil {
		return ErrState
	}
	for i := 0; i < 4; i++ {
		if r.seats[i] == uid {
			r.ready[i] = ready
			r.broadcastState("")
			return nil
		}
	}
	return ErrState
}

func (r *Room) kick(uid, target int64) error {
	if uid != r.owner {
		return ErrPerm
	}
	if r.table != nil {
		return ErrState
	}
	if target == r.owner {
		return ErrPerm
	}
	delete(r.members, target)
	for i := 0; i < 4; i++ {
		if r.seats[i] == target {
			r.seats[i] = 0
			r.ready[i] = false
		}
	}
	r.mgr.mu.Lock()
	r.mgr.unbindLocked(target, r.id)
	r.mgr.mu.Unlock()
	r.broadcastState("")
	return nil
}

func (r *Room) start(uid int64) error {
	if uid != r.owner {
		return ErrPerm
	}
	if r.table != nil {
		return ErrState
	}
	var uids [4]int64
	for i := 0; i < 4; i++ {
		if r.seats[i] == 0 || !r.ready[i] {
			log.Printf("room %s start denied: seat %d uid=%d ready=%v", r.id, i, r.seats[i], r.ready[i])
			return ErrState
		}
		uids[i] = r.seats[i]
	}
	eng := r.mgr.eng
	judge := func(ctx rules.HandContext) (bool, bool, rules.FanResult) {
		legal, wrong, fr, err := settle.Evaluate(eng, ctx)
		if err != nil {
			log.Printf("judge: %v", err)
			return false, true, fr
		}
		return legal, wrong, fr
	}
	r.table = game.NewTable(uids, 0, r.mgr.timeout, judge, nil)
	evs, err := r.table.Deal(time.Now())
	if err != nil {
		r.table = nil
		log.Printf("room %s deal: %v", r.id, err)
		return err
	}
	log.Printf("room %s started", r.id)
	r.emit(evs)
	if r.table != nil && r.table.Finished != nil {
		r.table = nil
		r.clearReady()
		r.broadcastState("")
	}
	return nil
}

func (r *Room) leave(uid int64) error {
	if r.table != nil {
		return nil
	}
	delete(r.members, uid)
	for i := 0; i < 4; i++ {
		if r.seats[i] == uid {
			r.seats[i] = 0
			r.ready[i] = false
		}
	}
	r.mgr.mu.Lock()
	r.mgr.unbindLocked(uid, r.id)
	r.mgr.mu.Unlock()
	if uid == r.owner {
		r.mgr.drop(r.id)
		close(r.quit)
		return nil
	}
	r.broadcastState("")
	return nil
}

func (r *Room) host(uid int64) {
	if r.table == nil {
		return
	}
	for i := 0; i < 4; i++ {
		if r.seats[i] == uid {
			r.table.Seats[i].Hosted = true
			break
		}
	}
	evs, err := r.table.Tick(time.Now())
	if err != nil {
		log.Printf("room %s host: %v", r.id, err)
		return
	}
	r.emit(evs)
	if r.table.Finished != nil {
		r.table = nil
		r.clearReady()
		r.broadcastState("")
	}
}

func (r *Room) act(uid int64, act game.Action) error {
	if r.table == nil {
		return ErrState
	}
	seat := -1
	for i := 0; i < 4; i++ {
		if r.seats[i] == uid {
			seat = i
			break
		}
	}
	if seat < 0 {
		return ErrPerm
	}
	evs, err := r.table.Apply(seat, act, time.Now())
	if err != nil {
		return err
	}
	r.emit(evs)
	if r.table.Finished != nil {
		r.table = nil
		r.clearReady()
		r.broadcastState("")
	}
	return nil
}

func (r *Room) invite(uid, peer int64) error {
	if r.mgr.friends != nil && !r.mgr.friends(uid, peer) {
		return ErrPerm
	}
	r.sendState(peer, r.id)
	return nil
}

func (r *Room) clearReady() {
	for i := 0; i < 4; i++ {
		r.ready[i] = false
	}
}

func (r *Room) state(hint string) *pb.S2CRoomState {
	st := &pb.S2CRoomState{RoomId: r.id, Owner: r.owner, InviteHint: hint, Seats: make([]*pb.SeatInfo, 4)}
	if r.table != nil {
		st.Phase = uint32(r.table.Phase)
	}
	for i := 0; i < 4; i++ {
		uid := r.seats[i]
		info := &pb.SeatInfo{Uid: uid, Ready: r.ready[i]}
		if uid != 0 {
			info.Name = r.mgr.users.Name(uid)
		}
		st.Seats[i] = info
	}
	return st
}

func (r *Room) broadcastState(hint string) {
	msg := r.state(hint)
	seen := map[int64]struct{}{}
	for uid := range r.members {
		r.mgr.push(uid, pb.Cmd_S2C_ROOM_STATE, 0, msg)
		seen[uid] = struct{}{}
	}
	for i := 0; i < 4; i++ {
		uid := r.seats[i]
		if uid != 0 {
			if _, ok := seen[uid]; !ok {
				r.mgr.push(uid, pb.Cmd_S2C_ROOM_STATE, 0, msg)
			}
		}
	}
	if r.owner != 0 {
		if _, ok := seen[r.owner]; !ok {
			r.mgr.push(r.owner, pb.Cmd_S2C_ROOM_STATE, 0, msg)
		}
	}
}

func (r *Room) sendState(uid int64, hint string) {
	r.mgr.push(uid, pb.Cmd_S2C_ROOM_STATE, 0, r.state(hint))
}

func (r *Room) emit(evs []game.Event) {
	for _, ev := range evs {
		r.emitOne(ev)
	}
}

func toPBAct(t game.ActType) pb.ActionType {
	switch t {
	case game.ActPass:
		return pb.ActionType_ACT_PASS
	case game.ActDiscard:
		return pb.ActionType_ACT_DISCARD
	case game.ActChi:
		return pb.ActionType_ACT_CHI
	case game.ActPeng:
		return pb.ActionType_ACT_PENG
	case game.ActGangMing:
		return pb.ActionType_ACT_GANG_MING
	case game.ActAnGang:
		return pb.ActionType_ACT_GANG_AN
	case game.ActJiaGang:
		return pb.ActionType_ACT_GANG_JIA
	case game.ActHu:
		return pb.ActionType_ACT_HU
	case game.ActDraw:
		return pb.ActionType_ACT_DRAW
	case game.ActBuhua:
		return pb.ActionType_ACT_BUHUA
	default:
		return pb.ActionType_ACT_UNSPECIFIED
	}
}

func tilesBytes(ts []tile.Tile) []byte {
	b := make([]byte, len(ts))
	for i, x := range ts {
		b[i] = byte(x)
	}
	return b
}

func (r *Room) emitOne(ev game.Event) {
	if ev.Settle != nil {
		msg := &pb.S2CSettle{
			Winner:    uint32(max0(ev.Settle.Winner)),
			HuKind:    int32(ev.Settle.Kind),
			Fan:       int32(ev.Settle.Fan),
			Scores:    []int32{ev.Settle.Scores[0], ev.Settle.Scores[1], ev.Settle.Scores[2], ev.Settle.Scores[3]},
			Discarder: uint32(max0(ev.Settle.Discarder)),
		}
		if ev.Settle.Winner < 0 {
			msg.Winner = 0
		}
		for _, it := range ev.Settle.Items {
			msg.Fans = append(msg.Fans, &pb.FanItem{Id: uint32(it.ID), Score: uint32(it.Score)})
		}
		var hands []byte
		for i := 0; i < 4; i++ {
			hands = append(hands, tilesBytes(ev.DealHands[i])...)
			hands = append(hands, 0xFF)
		}
		msg.Hands = hands
		for i := 0; i < 4; i++ {
			if r.seats[i] != 0 {
				r.mgr.push(r.seats[i], pb.Cmd_S2C_SETTLE, 0, msg)
			}
		}
		return
	}
	if ev.IsDeal {
		for i := 0; i < 4; i++ {
			if r.seats[i] == 0 {
				continue
			}
			msg := &pb.S2CDeal{
				Seat:   uint32(i),
				Hand:   tilesBytes(ev.DealHands[i]),
				Banker: uint32(ev.Banker),
				Wind:   uint32(ev.Wind),
				TurnId: ev.TurnID,
			}
			r.mgr.push(r.seats[i], pb.Cmd_S2C_DEAL, 0, msg)
		}
		return
	}
	msg := &pb.S2CGameEvent{
		TurnId:   ev.TurnID,
		Type:     toPBAct(ev.Type),
		Seat:     uint32(ev.Seat),
		Tile:     uint32(ev.Tile),
		Tiles:    tilesBytes(ev.Tiles),
		WallLeft: uint32(ev.WallLeft),
	}
	if ev.Type == game.ActAnGang {
		msg.Tile = 0
		msg.Tiles = nil
	}
	if ev.PrivateSeat >= 0 {
		uid := r.seats[ev.PrivateSeat]
		if uid != 0 {
			r.mgr.push(uid, pb.Cmd_S2C_GAME_EVENT, 0, msg)
		}
		pub := proto.Clone(msg).(*pb.S2CGameEvent)
		if ev.Type == game.ActDraw {
			pub.Tile = 0
			pub.Tiles = nil
		}
		for i := 0; i < 4; i++ {
			if i == ev.PrivateSeat || r.seats[i] == 0 {
				continue
			}
			r.mgr.push(r.seats[i], pb.Cmd_S2C_GAME_EVENT, 0, pub)
		}
		return
	}
	for i := 0; i < 4; i++ {
		if r.seats[i] != 0 {
			r.mgr.push(r.seats[i], pb.Cmd_S2C_GAME_EVENT, 0, msg)
		}
	}
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
