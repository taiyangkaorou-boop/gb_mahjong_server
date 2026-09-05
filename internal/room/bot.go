package room

import (
	"fmt"
	"sort"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/ai"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// SeatSnap 给 GM 看的一个座位。
type SeatSnap struct {
	Seat  int    `json:"seat"`
	UID   int64  `json:"uid"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
	Bot   bool   `json:"bot"`
}

// RoomSnap 给 GM 看的房间快照。
type RoomSnap struct {
	ID    string     `json:"room_id"`
	Owner int64      `json:"owner"`
	Phase int        `json:"phase"`
	Seats []SeatSnap `json:"seats"`
	Added int        `json:"added,omitempty"`
}

func botDisplayName(seat int) string {
	return fmt.Sprintf("电脑%d", seat+1)
}

// List 列出当前所有房间，给 GM 页刷新用。
func (m *Manager) List() []RoomSnap {
	m.mu.Lock()
	ids := make([]string, 0, len(m.rooms))
	for id := range m.rooms {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	sort.Strings(ids)
	out := make([]RoomSnap, 0, len(ids))
	for _, id := range ids {
		snap, err := m.Snapshot(id)
		if err != nil {
			continue
		}
		out = append(out, snap)
	}
	return out
}

// Snapshot 读一个房间当前座位。
func (m *Manager) Snapshot(id string) (RoomSnap, error) {
	r := m.roomByID(id)
	if r == nil {
		return RoomSnap{}, ErrNotFound
	}
	var snap RoomSnap
	err := r.submit(cmd{kind: cmdSnap, snap: &snap})
	return snap, err
}

// AddBots 把房间里还空着的座位填成电脑玩家，电脑立刻准备。
// 对局已经开始时不能加。没有空位返回 ErrFull。
func (m *Manager) AddBots(id string) (RoomSnap, error) {
	r := m.roomByID(id)
	if r == nil {
		return RoomSnap{}, ErrNotFound
	}
	var added int
	var snap RoomSnap
	err := r.submit(cmd{kind: cmdAddBot, added: &added, snap: &snap})
	snap.Added = added
	return snap, err
}

func (r *Room) snapshot() RoomSnap {
	st := RoomSnap{ID: r.id, Owner: r.owner, Seats: make([]SeatSnap, 4)}
	if r.table != nil {
		st.Phase = int(r.table.Phase)
	}
	for i := 0; i < 4; i++ {
		st.Seats[i] = SeatSnap{
			Seat:  i,
			UID:   r.seats[i],
			Name:  r.seatName(i),
			Ready: r.ready[i],
			Bot:   r.bot[i],
		}
	}
	return st
}

func (r *Room) addBots() (int, error) {
	if r.table != nil {
		return 0, ErrState
	}
	added := 0
	for i := 0; i < 4; i++ {
		if r.seats[i] != 0 {
			continue
		}
		uid := r.allocBotUID()
		r.seats[i] = uid
		r.ready[i] = true
		r.bot[i] = true
		r.members[uid] = struct{}{}
		added++
	}
	if added == 0 {
		return 0, ErrFull
	}
	logx.Infof("room addbot room=%s filled=%d", r.id, added)
	r.broadcastState("")
	return added, nil
}

func (r *Room) allocBotUID() int64 {
	r.mgr.mu.Lock()
	defer r.mgr.mu.Unlock()
	uid := r.mgr.nextBot
	r.mgr.nextBot--
	if r.mgr.nextBot >= 0 {
		r.mgr.nextBot = -1
	}
	r.mgr.bindLocked(uid, r.id)
	return uid
}

func (r *Room) initBotViews() {
	for i := 0; i < 4; i++ {
		if r.bot[i] {
			r.views[i] = &ai.View{Seat: i}
		} else {
			r.views[i] = nil
		}
	}
}

func (r *Room) feedBots(evs []game.Event) {
	if len(evs) == 0 {
		return
	}
	for i := 0; i < 4; i++ {
		if r.views[i] != nil {
			r.views[i].OnEvents(evs)
		}
	}
}

func (r *Room) afterEvents(evs []game.Event) {
	r.feedBots(evs)
	r.emit(evs)
	if r.finishIfOver() {
		return
	}
	r.playBots(time.Now())
}

func (r *Room) finishIfOver() bool {
	if r.table == nil || r.table.Finished == nil {
		return false
	}
	r.table = nil
	r.clearReady()
	r.broadcastState("")
	return true
}

func (r *Room) nextBotSeat() (int, bool) {
	if r.table == nil {
		return -1, false
	}
	for _, s := range r.table.WaitingSeats() {
		if s >= 0 && s < 4 && r.bot[s] {
			return s, true
		}
	}
	return -1, false
}

// playBots 让当前该动手的电脑按内置 AI 连续出牌，直到轮到真人或不该动手。
func (r *Room) playBots(now time.Time) {
	for step := 0; step < 800; step++ {
		if r.table == nil || r.table.Finished != nil {
			r.finishIfOver()
			return
		}
		seat, ok := r.nextBotSeat()
		if !ok {
			return
		}
		v := r.views[seat]
		if v == nil {
			v = &ai.View{Seat: seat}
			r.views[seat] = v
		}
		act := ai.Decide(v, r.mgr.eng)
		if act.Type == 0 {
			act = r.botFallback(seat)
		}
		if act.Type == 0 {
			logx.Warnf("room bot stuck room=%s seat=%d phase=%s", r.id, seat, r.table.Phase)
			return
		}
		logx.Tracef("room bot act room=%s seat=%d act=%s", r.id, seat, act.Type)
		act.TurnID = r.table.TurnID
		v.NoteSent(act)
		evs, err := r.table.Apply(seat, act, now)
		if err != nil {
			logx.Warnf("room bot act room=%s seat=%d act=%s: %v", r.id, seat, act.Type, err)
			return
		}
		r.feedBots(evs)
		r.emit(evs)
		if r.finishIfOver() {
			return
		}
		now = now.Add(time.Millisecond)
	}
	logx.Warnf("room bot too many steps room=%s", r.id)
}

func (r *Room) botFallback(seat int) game.Action {
	if r.table == nil {
		return game.Action{}
	}
	switch r.table.Phase {
	case game.PhaseResp, game.PhaseQiang:
		return game.Action{Type: game.ActPass}
	case game.PhaseSelfAct:
		p := &r.table.Seats[seat]
		disc := r.table.LastDraw
		if disc == 0 || tile.Count(p.Hand, disc) == 0 {
			if len(p.Hand) == 0 {
				return game.Action{}
			}
			disc = p.Hand[len(p.Hand)-1]
		}
		return game.Action{Type: game.ActDiscard, Tile: disc}
	default:
		return game.Action{}
	}
}
