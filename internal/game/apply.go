package game

import (
	"errors"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

var (
	ErrBadTurn     = errors.New("turn_id mismatch")
	ErrBadState    = errors.New("bad game state")
	ErrBadAction   = errors.New("illegal action")
	ErrNotYourTurn = errors.New("not your turn")
)

func evBase(t *Table, typ ActType, seat int) Event {
	return Event{Type: typ, Seat: seat, WallLeft: t.wallLeft(), PrivateSeat: -1, TurnID: t.TurnID}
}

func (t *Table) Deal(now time.Time) ([]Event, error) {
	logx.Tracef("game Deal banker=%d phase=%s", t.Banker, t.Phase)
	if t.Phase != PhaseIdle {
		logx.Warnf("game Deal rejected phase=%s", t.Phase)
		return nil, ErrBadState
	}
	for i := 0; i < 13; i++ {
		for s := 0; s < 4; s++ {
			seat := (t.Banker + s) % 4
			x, ok := t.Wall.Draw()
			if !ok {
				return t.finishHuang()
			}
			t.Seats[seat].Hand = append(t.Seats[seat].Hand, x)
		}
	}
	x, ok := t.Wall.Draw()
	if !ok {
		return t.finishHuang()
	}
	t.Seats[t.Banker].Hand = append(t.Seats[t.Banker].Hand, x)
	t.LastDraw = x
	t.drawnThisTurn = true

	evs := []Event{}
	for s := 0; s < 4; s++ {
		seat := (t.Banker + s) % 4
		more, err := t.replaceFlowers(seat, false)
		if err != nil {
			return evs, err
		}
		evs = append(evs, more...)
		if t.Phase == PhaseOver {
			return evs, nil
		}
	}
	t.bump()
	t.Phase = PhaseSelfAct
	t.Current = t.Banker
	t.AfterMeldNoKong = false
	t.armDeadline(now)
	deal := Event{IsDeal: true, Banker: t.Banker, Wind: t.Prevailing, TurnID: t.TurnID, WallLeft: t.wallLeft(), PrivateSeat: -1}
	for i := 0; i < 4; i++ {
		deal.DealHands[i] = append([]tile.Tile(nil), t.Seats[i].Hand...)
	}
	out := []Event{deal}
	out = append(out, evs...)
	return out, nil
}

func (t *Table) replaceFlowers(seat int, announceDraw bool) ([]Event, error) {
	var evs []Event
	for {
		p := &t.Seats[seat]
		idx := -1
		for i, x := range p.Hand {
			if x.IsHua() {
				idx = i
				break
			}
		}
		if idx < 0 {
			tile.Sort(p.Hand)
			return evs, nil
		}
		hua := p.Hand[idx]
		p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
		p.Flowers = append(p.Flowers, hua)
		rep, ok := t.Wall.Replace()
		if !ok || (t.Wall.Left() == 0 && rep.IsHua()) {
			evs = append(evs, Event{Type: ActBuhua, Seat: seat, Tile: hua, WallLeft: t.wallLeft(), PrivateSeat: -1, TurnID: t.TurnID})
			more, _ := t.finishHuang()
			return append(evs, more...), nil
		}
		if t.Wall.Left() == 0 && !rep.IsHua() {
			t.HaidiDraw = true
		}
		if t.AfterKong {
			t.AfterKongThenHua = true
		}
		p.Hand = append(p.Hand, rep)
		t.LastDraw = rep
		evs = append(evs, Event{Type: ActBuhua, Seat: seat, Tile: hua, WallLeft: t.wallLeft(), PrivateSeat: -1, TurnID: t.TurnID})
		if announceDraw && !rep.IsHua() {
			evs = append(evs, Event{Type: ActDraw, Seat: seat, Tile: rep, WallLeft: t.wallLeft(), PrivateSeat: seat, TurnID: t.TurnID})
		}
		if rep.IsHua() && t.HaidiDraw {
			more, _ := t.finishHuang()
			return append(evs, more...), nil
		}
	}
}

func (t *Table) Apply(seat int, act Action, now time.Time) ([]Event, error) {
	logx.Tracef("game Apply seat=%d phase=%s act=%s turn=%d table_turn=%d", seat, t.Phase, act.Type, act.TurnID, t.TurnID)
	if t.Finished != nil || t.Phase == PhaseOver {
		logx.Tracef("game Apply rejected seat=%d reason=finished", seat)
		return nil, ErrBadState
	}
	if act.TurnID != t.TurnID {
		logx.Warnf("game turn_id mismatch seat=%d got=%d want=%d", seat, act.TurnID, t.TurnID)
		return nil, ErrBadTurn
	}
	switch t.Phase {
	case PhaseSelfAct:
		return t.applySelf(seat, act, now)
	case PhaseResp:
		return t.applyResp(seat, act, now)
	case PhaseQiang:
		return t.applyQiang(seat, act, now)
	default:
		return nil, ErrBadState
	}
}

func (t *Table) Tick(now time.Time) ([]Event, error) {
	if t.Finished != nil || t.Phase == PhaseOver {
		return nil, nil
	}
	switch t.Phase {
	case PhaseSelfAct:
		if !t.Seats[t.Current].Hosted && !t.seatTimedOut(t.Current, now) {
			return nil, nil
		}
		p := &t.Seats[t.Current]
		disc := t.LastDraw
		if disc == 0 || tile.Count(p.Hand, disc) == 0 {
			if len(p.Hand) == 0 {
				return t.finishHuang()
			}
			disc = p.Hand[len(p.Hand)-1]
		}
		logx.Tracef("game Tick auto-discard seat=%d hosted=%v extra=%s", t.Current, t.Seats[t.Current].Hosted, t.ExtraLeft[t.Current])
		return t.Apply(t.Current, Action{TurnID: t.TurnID, Type: ActDiscard, Tile: disc}, now)
	case PhaseResp, PhaseQiang:
		for i := 0; i < 4; i++ {
			if i == t.LastDiscarder && t.Phase == PhaseResp {
				continue
			}
			if i == t.JiaGangSeat && t.Phase == PhaseQiang {
				continue
			}
			if t.Claims[i].got {
				continue
			}
			if t.Seats[i].Hosted || t.seatTimedOut(i, now) {
				t.consumeExtra(i, now)
				t.Claims[i] = claim{got: true, act: Action{Type: ActPass}}
			}
		}
		if !t.allClaimed() {
			t.Deadline = t.nextDeadline()
			return nil, nil
		}
		if t.Phase == PhaseResp {
			logx.Tracef("game Tick resolve phase=%s", t.Phase)
			return t.resolveResp(now)
		}
		logx.Tracef("game Tick resolve phase=%s", t.Phase)
		return t.resolveQiang(now)
	default:
		return nil, nil
	}
}

func (t *Table) applySelf(seat int, act Action, now time.Time) ([]Event, error) {
	logx.Tracef("game applySelf seat=%d act=%s", seat, act.Type)
	if seat != t.Current {
		return nil, ErrNotYourTurn
	}
	switch act.Type {
	case ActDiscard:
		return t.doDiscard(seat, act.Tile, now)
	case ActHu:
		return t.tryHu(seat, true, t.LastDraw, t.AfterKong && !t.AfterKongThenHua, now)
	case ActAnGang:
		if t.AfterMeldNoKong || t.HaidiDraw {
			return nil, ErrBadAction
		}
		return t.doAnGang(seat, act.Tile, now)
	case ActJiaGang:
		if t.AfterMeldNoKong || t.HaidiDraw {
			return nil, ErrBadAction
		}
		return t.doJiaGang(seat, act.Tile, now)
	default:
		return nil, ErrBadAction
	}
}

func (t *Table) doDiscard(seat int, x tile.Tile, now time.Time) ([]Event, error) {
	logx.Tracef("game doDiscard seat=%d tile=%s wall=%d", seat, x, t.wallLeft())
	p := &t.Seats[seat]
	next, ok := tile.RemoveN(p.Hand, x, 1)
	if !ok {
		return nil, ErrBadAction
	}
	p.Hand = next
	tile.Sort(p.Hand)
	t.consumeExtra(seat, now)
	p.Discards = append(p.Discards, x)
	t.LastDiscard = x
	t.LastDiscarder = seat
	t.AfterMeldNoKong = false
	t.AfterKong = false
	t.AfterKongThenHua = false
	t.drawnThisTurn = false
	t.LastDraw = 0
	t.bump()
	ev := evBase(t, ActDiscard, seat)
	ev.Tile = x
	t.Phase = PhaseResp
	t.resetClaims(seat)
	t.armDeadline(now)
	return []Event{ev}, nil
}

func (t *Table) resetClaims(except int) {
	for i := 0; i < 4; i++ {
		t.Claims[i] = claim{}
		if i == except {
			t.Claims[i].got = true
			t.Claims[i].act.Type = ActPass
		}
	}
}

func (t *Table) applyResp(seat int, act Action, now time.Time) ([]Event, error) {
	if seat == t.LastDiscarder {
		return nil, ErrNotYourTurn
	}
	if t.Claims[seat].got {
		return nil, ErrBadAction
	}
	x := t.LastDiscard
	switch act.Type {
	case ActPass:
		t.Claims[seat] = claim{got: true, act: act}
	case ActHu:
		t.Claims[seat] = claim{got: true, act: act}
	case ActPeng:
		if t.Wall.Left() == 0 {
			return nil, ErrBadAction
		}
		if !CanPeng(t.Seats[seat].Hand, x) {
			return nil, ErrBadAction
		}
		t.Claims[seat] = claim{got: true, act: act}
	case ActGangMing:
		if t.Wall.Left() == 0 {
			return nil, ErrBadAction
		}
		if !CanMingGang(t.Seats[seat].Hand, x) {
			return nil, ErrBadAction
		}
		t.Claims[seat] = claim{got: true, act: act}
	case ActChi:
		if t.Wall.Left() == 0 {
			return nil, ErrBadAction
		}
		if (t.LastDiscarder+1)%4 != seat {
			return nil, ErrBadAction
		}
		a, b, ok := ChiTiles(act.ChiMid, x)
		if !ok {
			return nil, ErrBadAction
		}
		if tile.Count(t.Seats[seat].Hand, a) == 0 || tile.Count(t.Seats[seat].Hand, b) == 0 {
			return nil, ErrBadAction
		}
		t.Claims[seat] = claim{got: true, act: act}
	default:
		return nil, ErrBadAction
	}
	t.consumeExtra(seat, now)
	if t.allClaimed() {
		return t.resolveResp(now)
	}
	t.Deadline = t.nextDeadline()
	return nil, nil
}

func (t *Table) allClaimed() bool {
	for i := 0; i < 4; i++ {
		if !t.Claims[i].got {
			return false
		}
	}
	return true
}

func nearestHu(from int, seats []int) int {
	best, bestD := seats[0], 99
	for _, s := range seats {
		d := (s - from + 4) % 4
		if d == 0 {
			continue
		}
		if d < bestD {
			bestD = d
			best = s
		}
	}
	return best
}

func (t *Table) resolveResp(now time.Time) ([]Event, error) {
	var hu []int
	peng, gang, chi := -1, -1, -1
	var chiAct Action
	for s := 0; s < 4; s++ {
		if s == t.LastDiscarder {
			continue
		}
		c := t.Claims[s]
		if !c.got {
			continue
		}
		switch c.act.Type {
		case ActHu:
			hu = append(hu, s)
		case ActPeng:
			peng = s
		case ActGangMing:
			gang = s
		case ActChi:
			chi = s
			chiAct = c.act
		}
	}
	if len(hu) > 0 {
		w := nearestHu(t.LastDiscarder, hu)
		return t.tryHu(w, false, t.LastDiscard, false, now)
	}
	if gang >= 0 {
		return t.takeMingGang(gang, now)
	}
	if peng >= 0 {
		return t.takePeng(peng, now)
	}
	if chi >= 0 {
		return t.takeChi(chi, chiAct.ChiMid, now)
	}
	if t.Wall.Left() == 0 {
		return t.finishHuang()
	}
	next := (t.LastDiscarder + 1) % 4
	return t.drawFor(next, now)
}

func (t *Table) takePeng(seat int, now time.Time) ([]Event, error) {
	p := &t.Seats[seat]
	x := t.LastDiscard
	next, ok := tile.RemoveN(p.Hand, x, 2)
	if !ok {
		return nil, ErrBadAction
	}
	p.Hand = next
	off := OfferRel(seat, t.LastDiscarder)
	p.Melds = append(p.Melds, rules.Meld{Type: rules.MeldPeng, Tiles: []tile.Tile{x, x, x}, Offer: off, Mid: x})
	t.removeLastDiscard()
	t.Current = seat
	t.Phase = PhaseSelfAct
	t.AfterMeldNoKong = true
	t.LastDraw = 0
	t.bump()
	t.armDeadline(now)
	ev := evBase(t, ActPeng, seat)
	ev.Tile = x
	ev.Tiles = []tile.Tile{x, x}
	return []Event{ev}, nil
}

func (t *Table) takeChi(seat int, mid tile.Tile, now time.Time) ([]Event, error) {
	p := &t.Seats[seat]
	x := t.LastDiscard
	a, b, ok := ChiTiles(mid, x)
	if !ok {
		return nil, ErrBadAction
	}
	h, ok1 := tile.RemoveN(p.Hand, a, 1)
	h, ok2 := tile.RemoveN(h, b, 1)
	if !ok1 || !ok2 {
		return nil, ErrBadAction
	}
	p.Hand = h
	// 副露必须是完整顺子 左-中-右。不能用 ChiTiles 的两张下手牌去拼，
	// 否则吃左边一张时会变成 中-中-弃牌（例如 4,4,3），算番库解析失败当成错和。
	left, right := mid.Pred(), mid.Succ()
	if left == 0 || right == 0 {
		return nil, ErrBadAction
	}
	tiles := []tile.Tile{left, mid, right}
	p.Melds = append(p.Melds, rules.Meld{Type: rules.MeldChi, Tiles: tiles, Offer: ChiOffer(mid, x), Mid: mid})
	t.removeLastDiscard()
	t.Current = seat
	t.Phase = PhaseSelfAct
	t.AfterMeldNoKong = true
	t.LastDraw = 0
	t.bump()
	t.armDeadline(now)
	ev := evBase(t, ActChi, seat)
	ev.Tile = x
	ev.Tiles = []tile.Tile{a, b}
	return []Event{ev}, nil
}

func (t *Table) takeMingGang(seat int, now time.Time) ([]Event, error) {
	p := &t.Seats[seat]
	x := t.LastDiscard
	next, ok := tile.RemoveN(p.Hand, x, 3)
	if !ok {
		return nil, ErrBadAction
	}
	p.Hand = next
	off := OfferRel(seat, t.LastDiscarder)
	p.Melds = append(p.Melds, rules.Meld{Type: rules.MeldMingGang, Tiles: []tile.Tile{x, x, x, x}, Offer: off, Mid: x})
	t.removeLastDiscard()
	t.AfterKong = true
	t.AfterKongThenHua = false
	t.AfterMeldNoKong = false
	t.Current = seat
	t.bump()
	ev := evBase(t, ActGangMing, seat)
	ev.Tile = x
	ev.Tiles = []tile.Tile{x, x, x}
	more, err := t.kongReplace(seat, now)
	return append([]Event{ev}, more...), err
}

func (t *Table) removeLastDiscard() {
	p := &t.Seats[t.LastDiscarder]
	if n := len(p.Discards); n > 0 && p.Discards[n-1] == t.LastDiscard {
		p.Discards = p.Discards[:n-1]
	}
}

func (t *Table) doAnGang(seat int, x tile.Tile, now time.Time) ([]Event, error) {
	if tile.Count(t.Seats[seat].Hand, x) < 4 {
		return nil, ErrBadAction
	}
	next, ok := tile.RemoveN(t.Seats[seat].Hand, x, 4)
	if !ok {
		return nil, ErrBadAction
	}
	t.Seats[seat].Hand = next
	t.Seats[seat].Melds = append(t.Seats[seat].Melds, rules.Meld{Type: rules.MeldAnGang, Tiles: []tile.Tile{x, x, x, x}, Offer: 0, Mid: x})
	t.consumeExtra(seat, now)
	t.AfterKong = true
	t.AfterKongThenHua = false
	t.bump()
	ev := evBase(t, ActAnGang, seat)
	ev.Tile = 0
	more, err := t.kongReplace(seat, now)
	return append([]Event{ev}, more...), err
}

func (t *Table) doJiaGang(seat int, x tile.Tile, now time.Time) ([]Event, error) {
	p := &t.Seats[seat]
	found := -1
	for i, m := range p.Melds {
		if m.Type == rules.MeldPeng && m.Mid == x {
			found = i
			break
		}
	}
	if found < 0 || tile.Count(p.Hand, x) == 0 {
		return nil, ErrBadAction
	}
	next, ok := tile.RemoveN(p.Hand, x, 1)
	if !ok {
		return nil, ErrBadAction
	}
	p.Hand = next
	m := p.Melds[found]
	m.Type = rules.MeldJiaGang
	m.Tiles = []tile.Tile{x, x, x, x}
	m.Offer = m.Offer + 4
	p.Melds[found] = m
	t.QiangGangTile = x
	t.JiaGangSeat = seat
	t.Phase = PhaseQiang
	t.resetClaims(seat)
	t.consumeExtra(seat, now)
	t.bump()
	t.armDeadline(now)
	ev := evBase(t, ActJiaGang, seat)
	ev.Tile = x
	return []Event{ev}, nil
}

func (t *Table) applyQiang(seat int, act Action, now time.Time) ([]Event, error) {
	if seat == t.JiaGangSeat {
		return nil, ErrNotYourTurn
	}
	if t.Claims[seat].got {
		return nil, ErrBadAction
	}
	if act.Type != ActPass && act.Type != ActHu {
		return nil, ErrBadAction
	}
	t.Claims[seat] = claim{got: true, act: act}
	t.consumeExtra(seat, now)
	if t.allClaimed() {
		return t.resolveQiang(now)
	}
	t.Deadline = t.nextDeadline()
	return nil, nil
}

func (t *Table) resolveQiang(now time.Time) ([]Event, error) {
	var hu []int
	for s := 0; s < 4; s++ {
		if s == t.JiaGangSeat {
			continue
		}
		if t.Claims[s].got && t.Claims[s].act.Type == ActHu {
			hu = append(hu, s)
		}
	}
	if len(hu) > 0 {
		w := nearestHu(t.JiaGangSeat, hu)
		t.LastDiscarder = t.JiaGangSeat
		t.LastDiscard = t.QiangGangTile
		return t.tryHu(w, false, t.QiangGangTile, true, now)
	}
	t.AfterKong = true
	t.AfterKongThenHua = false
	t.Current = t.JiaGangSeat
	return t.kongReplace(t.JiaGangSeat, now)
}

func (t *Table) kongReplace(seat int, now time.Time) ([]Event, error) {
	x, ok := t.Wall.Replace()
	if !ok {
		return t.finishHuang()
	}
	if x.IsHua() && t.Wall.Left() == 0 {
		return t.finishHuang()
	}
	t.Seats[seat].Hand = append(t.Seats[seat].Hand, x)
	t.LastDraw = x
	t.drawnThisTurn = true
	t.Current = seat
	t.Phase = PhaseSelfAct
	t.HaidiDraw = t.Wall.Left() == 0
	t.armDeadline(now)
	return t.announceDraw(seat, x)
}

func (t *Table) drawFor(seat int, now time.Time) ([]Event, error) {
	x, ok := t.Wall.Draw()
	if !ok {
		return t.finishHuang()
	}
	if x.IsHua() && t.Wall.Left() == 0 {
		return t.finishHuang()
	}
	t.Seats[seat].Hand = append(t.Seats[seat].Hand, x)
	t.LastDraw = x
	t.drawnThisTurn = true
	t.Current = seat
	t.Phase = PhaseSelfAct
	t.AfterMeldNoKong = false
	t.AfterKong = false
	t.AfterKongThenHua = false
	t.HaidiDraw = t.Wall.Left() == 0
	t.bump()
	t.armDeadline(now)
	return t.announceDraw(seat, x)
}

// announceDraw 花牌不单独下发摸牌，等补花后只亮最终进张，避免客户端把花打出去。
func (t *Table) announceDraw(seat int, x tile.Tile) ([]Event, error) {
	var evs []Event
	if !x.IsHua() {
		evs = append(evs, Event{Type: ActDraw, Seat: seat, Tile: x, WallLeft: t.wallLeft(), PrivateSeat: seat, TurnID: t.TurnID})
	}
	more, err := t.replaceFlowers(seat, true)
	return append(evs, more...), err
}

func (t *Table) tryHu(seat int, zimo bool, win tile.Tile, gang bool, now time.Time) ([]Event, error) {
	logx.Tracef("game tryHu seat=%d zimo=%v gang=%v win=%s", seat, zimo, gang, win)
	if t.Judge == nil {
		return nil, ErrBadState
	}
	if zimo {
		t.consumeExtra(seat, now)
	}
	ctx := t.Context(seat, win, zimo, gang)
	legal, wrong, fr := t.Judge(ctx)
	if wrong || !legal {
		logx.Warnf("game wrong hu seat=%d fan=%d fans=%s", seat, fr.TotalFan, settle.FormatFans(fr.Items))
		res := settle.Result{Kind: settle.KindWrong, Winner: -1, Discarder: t.LastDiscarder, Items: fr.Items, StartFan: fr.TotalFan - fr.FlowerFan, Fan: fr.TotalFan}
		res.Scores = settle.PayWrong(seat)
		return t.finish(res)
	}
	kind := settle.KindRon
	if zimo {
		kind = settle.KindZimo
		t.LastDiscarder = -1
	} else if gang {
		kind = settle.KindQiang
	}
	fan := fr.TotalFan
	res := settle.Result{
		Kind:      kind,
		Winner:    seat,
		Discarder: t.LastDiscarder,
		Fan:       fan,
		StartFan:  fr.TotalFan - fr.FlowerFan,
		Items:     fr.Items,
		Scores:    settle.PayHu(zimo, fan, seat, t.LastDiscarder),
	}
	return t.finish(res)
}

func (t *Table) finishHuang() ([]Event, error) {
	return t.finish(settle.Huang())
}

func (t *Table) finish(res settle.Result) ([]Event, error) {
	t.Phase = PhaseOver
	t.Finished = &res
	t.bump()
	logx.Infof("game finished kind=%s winner=%d fan=%d start=%d fans=%s scores=%v", settle.KindName(res.Kind), res.Winner, res.Fan, res.StartFan, settle.FormatFans(res.Items), res.Scores)
	return []Event{{Type: 0, Settle: &res, TurnID: t.TurnID, WallLeft: t.wallLeft(), PrivateSeat: -1,
		DealHands: [4][]tile.Tile{
			append([]tile.Tile(nil), t.Seats[0].Hand...),
			append([]tile.Tile(nil), t.Seats[1].Hand...),
			append([]tile.Tile(nil), t.Seats[2].Hand...),
			append([]tile.Tile(nil), t.Seats[3].Hand...),
		}}}, nil
}
