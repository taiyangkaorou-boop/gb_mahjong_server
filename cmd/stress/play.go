package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
)

type playStats struct {
	ok, fail, wrong int
	kinds           map[int32]int
	durs            []time.Duration
}

func runPlay(addr, prefix string, rooms int) error {
	if rooms <= 0 {
		return fmt.Errorf("-rooms must be > 0")
	}
	st := playStats{kinds: map[int32]int{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(rooms)
	t0 := time.Now()
	for r := 0; r < rooms; r++ {
		r := r
		go func() {
			defer wg.Done()
			start := time.Now()
			kind, err := playOneRoom(addr, prefix, r)
			d := time.Since(start)
			mu.Lock()
			defer mu.Unlock()
			st.durs = append(st.durs, d)
			if err != nil {
				st.fail++
				log.Printf("room %d: %v", r, err)
				return
			}
			st.ok++
			st.kinds[kind]++
			if kind == 4 {
				st.wrong++
			}
		}()
	}
	wg.Wait()
	log.Printf("stress play rooms=%d ok=%d fail=%d wrong_hu=%d kinds=%v wall=%s p50=%s p95=%s max=%s",
		rooms, st.ok, st.fail, st.wrong, st.kinds, fmtDur(time.Since(t0)),
		fmtDur(pct(st.durs, 0.50)), fmtDur(pct(st.durs, 0.95)), fmtDur(pct(st.durs, 1)))
	if st.fail > 0 {
		return fmt.Errorf("%d room(s) failed", st.fail)
	}
	if st.wrong > 0 {
		return fmt.Errorf("%d wrong hu (kind=4)", st.wrong)
	}
	return nil
}

func playOneRoom(addr, prefix string, roomIdx int) (int32, error) {
	roomCh := make(chan string, 4)
	kindCh := make(chan int32, 4)
	errCh := make(chan error, 4)
	base := roomIdx * 4
	var wg sync.WaitGroup
	for seat := 0; seat < 4; seat++ {
		seat := seat
		wg.Add(1)
		go func() {
			defer wg.Done()
			kind, err := playSeat(addr, uname(prefix, base+seat), seat, roomCh)
			if err != nil {
				errCh <- err
				return
			}
			kindCh <- kind
		}()
	}
	wg.Wait()
	close(errCh)
	close(kindCh)
	for err := range errCh {
		if err != nil {
			return 0, err
		}
	}
	var kind int32
	for k := range kindCh {
		kind = k
	}
	return kind, nil
}

// dumb 只打最后一张、别人出牌一律过。用来压服务器，不算番。
type dumb struct {
	c         *client
	leader    bool
	seat      int
	hand      []byte
	turnID    uint32
	needSelf  bool
	needClaim bool
	pending   bool
	dealt     bool
	readyN    int
	roomID    string
	startSent time.Time
}

func playSeat(addr, name string, seat int, roomCh chan string) (int32, error) {
	_, tok, err := registerLogin(addr, name)
	if err != nil {
		return 0, err
	}
	c, err := dialAuth(addr, tok)
	if err != nil {
		return 0, err
	}
	defer c.close()
	d := &dumb{c: c, leader: seat == 0, seat: seat}
	_ = c.send(pb.Cmd_C2S_ROOM_LEAVE, &pb.C2SRoomLeave{})

	if d.leader {
		if err := c.send(pb.Cmd_C2S_ROOM_CREATE, &pb.C2SRoomCreate{}); err != nil {
			return 0, err
		}
		if err := d.waitRoom(8 * time.Second); err != nil {
			return 0, fmt.Errorf("create: %w", err)
		}
		for i := 0; i < 3; i++ {
			roomCh <- d.roomID
		}
	} else {
		id := <-roomCh
		d.roomID = id
		if err := c.send(pb.Cmd_C2S_ROOM_JOIN, &pb.C2SRoomJoin{RoomId: id}); err != nil {
			return 0, err
		}
		if err := d.waitJoin(8 * time.Second); err != nil {
			return 0, fmt.Errorf("join: %w", err)
		}
	}
	if err := c.send(pb.Cmd_C2S_ROOM_SIT, &pb.C2SRoomSit{Seat: uint32(seat)}); err != nil {
		return 0, err
	}
	if err := c.send(pb.Cmd_C2S_ROOM_READY, &pb.C2SRoomReady{Ready: true}); err != nil {
		return 0, err
	}

	_ = c.ws.SetReadDeadline(time.Now().Add(3 * time.Minute))
	for {
		env, err := c.read()
		if err != nil {
			return 0, err
		}
		if env.Cmd == pb.Cmd_S2C_SETTLE {
			var s pb.S2CSettle
			_ = netx.UnmarshalBody(env, &s)
			return s.HuKind, nil
		}
		if env.Code != 0 {
			if env.Cmd == pb.Cmd_S2C_ERROR || env.Cmd == pb.Cmd_S2C_GAME_EVENT {
				d.pending = false
			}
		}
		d.onEnv(env)
		if d.leader && !d.dealt && d.readyN >= 4 {
			if d.startSent.IsZero() || time.Since(d.startSent) > 400*time.Millisecond {
				d.startSent = time.Now()
				_ = c.send(pb.Cmd_C2S_ROOM_START, &pb.C2SRoomStart{})
			}
		}
		d.maybeAct()
	}
}

func (d *dumb) waitRoom(timeout time.Duration) error {
	_ = d.c.ws.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = d.c.ws.SetReadDeadline(time.Time{}) }()
	for {
		env, err := d.c.read()
		if err != nil {
			return err
		}
		d.onEnv(env)
		if d.roomID != "" {
			return nil
		}
	}
}

func (d *dumb) waitJoin(timeout time.Duration) error {
	_ = d.c.ws.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = d.c.ws.SetReadDeadline(time.Time{}) }()
	for {
		env, err := d.c.read()
		if err != nil {
			return err
		}
		d.onEnv(env)
		if env.Cmd == pb.Cmd_S2C_ROOM_STATE && env.Code == 0 {
			return nil
		}
	}
}

func (d *dumb) onEnv(env *pb.Envelope) {
	switch env.Cmd {
	case pb.Cmd_S2C_ROOM_STATE:
		var st pb.S2CRoomState
		_ = netx.UnmarshalBody(env, &st)
		if st.RoomId != "" {
			d.roomID = st.RoomId
		}
		n := 0
		for _, s := range st.Seats {
			if s != nil && s.Uid != 0 && s.Ready {
				n++
			}
		}
		d.readyN = n
	case pb.Cmd_S2C_DEAL:
		var deal pb.S2CDeal
		_ = netx.UnmarshalBody(env, &deal)
		d.seat = int(deal.Seat)
		d.hand = append([]byte(nil), deal.Hand...)
		d.turnID = deal.TurnId
		d.dealt = true
		d.pending = false
		d.needClaim = false
		d.needSelf = len(d.hand)%3 == 2 && !hasHua(d.hand)
	case pb.Cmd_S2C_GAME_EVENT:
		var e pb.S2CGameEvent
		_ = netx.UnmarshalBody(env, &e)
		d.pending = false
		if e.TurnId != 0 {
			d.turnID = e.TurnId
		}
		d.onGame(&e)
	}
}

func (d *dumb) onGame(e *pb.S2CGameEvent) {
	me := int(e.Seat) == d.seat
	switch e.Type {
	case pb.ActionType_ACT_DRAW:
		d.needClaim = false
		if me && e.Tile != 0 {
			d.hand = append(d.hand, byte(e.Tile))
		}
		d.needSelf = me && len(d.hand)%3 == 2 && !hasHua(d.hand)
	case pb.ActionType_ACT_BUHUA:
		if me {
			d.hand = removeOne(d.hand, byte(e.Tile))
			d.needSelf = len(d.hand)%3 == 2 && !hasHua(d.hand)
		}
	case pb.ActionType_ACT_DISCARD:
		if me {
			d.hand = removeOne(d.hand, byte(e.Tile))
			d.needSelf = false
			d.needClaim = false
			return
		}
		d.needSelf = false
		d.needClaim = true
	case pb.ActionType_ACT_CHI, pb.ActionType_ACT_PENG:
		d.needClaim = false
		if me {
			for _, x := range e.Tiles {
				d.hand = removeOne(d.hand, x)
			}
			d.needSelf = true
		} else {
			d.needSelf = false
		}
	case pb.ActionType_ACT_GANG_MING, pb.ActionType_ACT_GANG_AN, pb.ActionType_ACT_GANG_JIA:
		d.needClaim = e.Type == pb.ActionType_ACT_GANG_JIA && !me
		if me {
			if e.Type == pb.ActionType_ACT_GANG_MING {
				d.hand = removeOne(d.hand, byte(e.Tile))
				d.hand = removeOne(d.hand, byte(e.Tile))
				d.hand = removeOne(d.hand, byte(e.Tile))
			} else if e.Type == pb.ActionType_ACT_GANG_JIA {
				d.hand = removeOne(d.hand, byte(e.Tile))
			}
			d.needSelf = false
		} else {
			d.needSelf = false
		}
	}
}

func (d *dumb) maybeAct() {
	if d.pending || hasHua(d.hand) {
		return
	}
	if d.needSelf && len(d.hand)%3 == 2 {
		t := lastPlayable(d.hand)
		if t == 0 {
			return
		}
		d.pending = true
		d.needSelf = false
		_ = d.c.send(pb.Cmd_C2S_ACTION, &pb.C2SAction{
			TurnId: d.turnID,
			Type:   pb.ActionType_ACT_DISCARD,
			Tile:   uint32(t),
		})
		return
	}
	if d.needClaim {
		d.pending = true
		d.needClaim = false
		_ = d.c.send(pb.Cmd_C2S_ACTION, &pb.C2SAction{
			TurnId: d.turnID,
			Type:   pb.ActionType_ACT_PASS,
		})
	}
}

func hasHua(hand []byte) bool {
	for _, x := range hand {
		if x>>4 == 6 {
			return true
		}
	}
	return false
}

func lastPlayable(hand []byte) byte {
	for i := len(hand) - 1; i >= 0; i-- {
		if hand[i]>>4 != 6 {
			return hand[i]
		}
	}
	return 0
}

func removeOne(hand []byte, t byte) []byte {
	for i, x := range hand {
		if x == t {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}
