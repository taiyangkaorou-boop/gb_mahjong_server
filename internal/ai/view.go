package ai

import (
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// View 是座位视角：只看见自己的手牌和公开信息。
type View struct {
	Seat            int
	Banker          int
	Wind            tile.Tile
	TurnID          uint32
	Hand            []tile.Tile
	Melds           []rules.Meld
	Flowers         []tile.Tile
	LastDraw        tile.Tile
	LastDiscard     tile.Tile
	LastDiscarder   int
	WallLeft        int
	NeedSelfAct     bool
	NeedClaim       bool
	Qiang           bool
	AfterMeldNoKong bool
	AfterKong       bool
	Haidi           bool
	visible         map[tile.Tile]int
}

// Stats 一局里自己发出的动作次数。
type Stats struct {
	Chi, Peng, Gang, Hu, Discard, Pass int
}

func (s *Stats) Add(t game.ActType) {
	switch t {
	case game.ActChi:
		s.Chi++
	case game.ActPeng:
		s.Peng++
	case game.ActGangMing, game.ActAnGang, game.ActJiaGang:
		s.Gang++
	case game.ActHu:
		s.Hu++
	case game.ActDiscard:
		s.Discard++
	case game.ActPass:
		s.Pass++
	}
}

// OnEvents 按顺序吸收牌桌事件。
func (v *View) OnEvents(evs []game.Event) {
	for _, e := range evs {
		v.OnEvent(e)
	}
}

// OnEvent 更新自己的手牌与待行动标志。
func (v *View) OnEvent(ev game.Event) {
	if ev.TurnID != 0 {
		v.TurnID = ev.TurnID
	}
	if !ev.IsDeal {
		v.WallLeft = ev.WallLeft
		v.Haidi = v.WallLeft == 0
	}
	if ev.Settle != nil {
		v.NeedSelfAct = false
		v.NeedClaim = false
		return
	}
	if ev.IsDeal {
		v.onDeal(ev)
		return
	}
	switch ev.Type {
	case game.ActDraw:
		v.onDraw(ev)
	case game.ActBuhua:
		v.onBuhua(ev)
	case game.ActDiscard:
		v.onDiscard(ev)
	case game.ActChi:
		v.onChi(ev)
	case game.ActPeng:
		v.onPeng(ev)
	case game.ActGangMing:
		v.onMingGang(ev)
	case game.ActAnGang:
		v.onAnGang(ev)
	case game.ActJiaGang:
		v.onJiaGang(ev)
	}
}

func (v *View) onDeal(ev game.Event) {
	v.Banker = ev.Banker
	v.Wind = ev.Wind
	if v.Wind == 0 {
		v.Wind = tile.East
	}
	v.Hand = append([]tile.Tile(nil), ev.DealHands[v.Seat]...)
	v.Melds = nil
	v.Flowers = nil
	v.LastDraw = 0
	v.LastDiscard = 0
	v.LastDiscarder = -1
	v.NeedClaim = false
	v.Qiang = false
	v.AfterMeldNoKong = false
	v.visible = map[tile.Tile]int{}
	v.NeedSelfAct = len(v.Hand)%3 == 2
	v.AfterKong = false
	tile.Sort(v.Hand)
}

func (v *View) onDraw(ev game.Event) {
	v.NeedClaim = false
	v.Qiang = false
	if ev.Seat != v.Seat {
		v.NeedSelfAct = false
		return
	}
	if ev.Tile != 0 {
		v.Hand = append(v.Hand, tile.Tile(uint8(ev.Tile)))
		v.LastDraw = tile.Tile(uint8(ev.Tile))
	}
	v.NeedSelfAct = true
	v.AfterMeldNoKong = false
}

func (v *View) onBuhua(ev game.Event) {
	if ev.Seat != v.Seat {
		return
	}
	x := tile.Tile(uint8(ev.Tile))
	if h, ok := tile.RemoveN(v.Hand, x, 1); ok {
		v.Hand = h
	}
	v.Flowers = append(v.Flowers, x)
}

func (v *View) onDiscard(ev game.Event) {
	x := tile.Tile(uint8(ev.Tile))
	v.LastDiscard = x
	v.LastDiscarder = ev.Seat
	v.Qiang = false
	v.addVisible(x, 1)
	if ev.Seat == v.Seat {
		if h, ok := tile.RemoveN(v.Hand, x, 1); ok {
			v.Hand = h
		}
		v.NeedSelfAct = false
		v.NeedClaim = false
		v.LastDraw = 0
		v.AfterKong = false
		return
	}
	v.NeedSelfAct = false
	v.NeedClaim = true
}

func (v *View) onChi(ev game.Event) {
	v.takeDiscard()
	if ev.Seat != v.Seat {
		v.NeedClaim = false
		return
	}
	for _, x := range ev.Tiles {
		if h, ok := tile.RemoveN(v.Hand, tile.Tile(x), 1); ok {
			v.Hand = h
		}
	}
	disc := tile.Tile(uint8(ev.Tile))
	if len(ev.Tiles) >= 2 {
		a, b := tile.Tile(ev.Tiles[0]), tile.Tile(ev.Tiles[1])
		mid := chiMidOf(a, b, disc)
		left, right := mid.Pred(), mid.Succ()
		if left == 0 || right == 0 {
			left, right = a, b
		}
		v.Melds = append(v.Melds, rules.Meld{
			Type:  rules.MeldChi,
			Tiles: []tile.Tile{left, mid, right},
			Mid:   mid,
			Offer: game.ChiOffer(mid, disc),
		})
	}
	v.NeedSelfAct = true
	v.NeedClaim = false
	v.AfterMeldNoKong = true
	v.AfterKong = false
	v.LastDraw = 0
}

func chiMidOf(a, b, disc tile.Tile) tile.Tile {
	ts := []tile.Tile{a, b, disc}
	tile.Sort(ts)
	if len(ts) == 3 {
		return ts[1]
	}
	return disc
}

func (v *View) onPeng(ev game.Event) {
	x := tile.Tile(uint8(ev.Tile))
	v.takeDiscard()
	if ev.Seat != v.Seat {
		v.NeedClaim = false
		return
	}
	if h, ok := tile.RemoveN(v.Hand, x, 2); ok {
		v.Hand = h
	}
	v.Melds = append(v.Melds, rules.Meld{
		Type:  rules.MeldPeng,
		Tiles: []tile.Tile{x, x, x},
		Mid:   x,
		Offer: game.OfferRel(v.Seat, v.LastDiscarder),
	})
	v.NeedSelfAct = true
	v.NeedClaim = false
	v.AfterMeldNoKong = true
	v.AfterKong = false
	v.LastDraw = 0
}

func (v *View) onMingGang(ev game.Event) {
	x := tile.Tile(uint8(ev.Tile))
	v.takeDiscard()
	if ev.Seat != v.Seat {
		v.NeedClaim = false
		return
	}
	if h, ok := tile.RemoveN(v.Hand, x, 3); ok {
		v.Hand = h
	}
	v.Melds = append(v.Melds, rules.Meld{
		Type:  rules.MeldMingGang,
		Tiles: []tile.Tile{x, x, x, x},
		Mid:   x,
		Offer: game.OfferRel(v.Seat, v.LastDiscarder),
	})
	v.NeedSelfAct = false
	v.NeedClaim = false
	v.AfterMeldNoKong = false
	v.AfterKong = true
	v.LastDraw = 0
}

func (v *View) onAnGang(ev game.Event) {
	if ev.Seat != v.Seat {
		v.NeedClaim = false
		return
	}
	x := tile.Tile(uint8(ev.Tile))
	if x == 0 {
		return
	}
	if h, ok := tile.RemoveN(v.Hand, x, 4); ok {
		v.Hand = h
	}
	v.Melds = append(v.Melds, rules.Meld{Type: rules.MeldAnGang, Tiles: []tile.Tile{x, x, x, x}, Mid: x})
	v.NeedSelfAct = false
	v.AfterKong = true
	v.LastDraw = 0
}

// ApplyLocal 用于暗杠：服务端事件不带牌面，必须按自己刚发出的动作改手牌。
func (v *View) ApplyLocal(act game.Action) {
	if act.Type != game.ActAnGang || act.Tile == 0 {
		return
	}
	if h, ok := tile.RemoveN(v.Hand, act.Tile, 4); ok {
		v.Hand = h
	}
	v.Melds = append(v.Melds, rules.Meld{Type: rules.MeldAnGang, Tiles: []tile.Tile{act.Tile, act.Tile, act.Tile, act.Tile}, Mid: act.Tile})
	v.NeedSelfAct = false
	v.AfterKong = true
	v.LastDraw = 0
}

// NoteSent 发出动作后立刻关掉待行动标志，避免下一条广播又重复提交。
func (v *View) NoteSent(act game.Action) {
	switch act.Type {
	case game.ActPass, game.ActChi, game.ActPeng, game.ActGangMing:
		v.NeedClaim = false
		v.Qiang = false
	case game.ActHu:
		v.NeedClaim = false
		v.NeedSelfAct = false
		v.Qiang = false
	case game.ActDiscard, game.ActJiaGang:
		v.NeedSelfAct = false
	case game.ActAnGang:
		v.ApplyLocal(act)
	}
}

func (v *View) onJiaGang(ev game.Event) {
	x := tile.Tile(uint8(ev.Tile))
	if ev.Seat == v.Seat {
		if h, ok := tile.RemoveN(v.Hand, x, 1); ok {
			v.Hand = h
		}
		for i := range v.Melds {
			if v.Melds[i].Type == rules.MeldPeng && v.Melds[i].Mid == x {
				v.Melds[i].Type = rules.MeldJiaGang
				v.Melds[i].Tiles = []tile.Tile{x, x, x, x}
				break
			}
		}
		v.NeedSelfAct = false
		v.NeedClaim = false
		v.Qiang = false
		v.AfterKong = true
		return
	}
	v.LastDiscard = x
	v.LastDiscarder = ev.Seat
	v.Qiang = true
	v.NeedClaim = true
	v.NeedSelfAct = false
}

func (v *View) takeDiscard() {
	if v.LastDiscard != 0 {
		v.addVisible(v.LastDiscard, -1)
	}
}

func (v *View) addVisible(x tile.Tile, n int) {
	if v.visible == nil {
		v.visible = map[tile.Tile]int{}
	}
	v.visible[x] += n
	if v.visible[x] < 0 {
		v.visible[x] = 0
	}
}

func (v *View) hasHua() bool {
	for _, x := range v.Hand {
		if x.IsHua() {
			return true
		}
	}
	return false
}

func (v *View) seatWind() tile.Tile {
	return tile.SeatWind(v.Banker, v.Seat)
}

func (v *View) jueZhang(x tile.Tile) bool {
	if x == 0 || v.visible == nil {
		return false
	}
	return v.visible[x] >= 3
}

func (v *View) ctx(win tile.Tile, zimo, gang bool) rules.HandContext {
	concealed := append([]tile.Tile(nil), v.Hand...)
	if win != 0 {
		if next, ok := tile.RemoveN(concealed, win, 1); ok {
			concealed = next
		}
	}
	return rules.HandContext{
		Concealed:      concealed,
		Melds:          append([]rules.Meld(nil), v.Melds...),
		Flowers:        append([]tile.Tile(nil), v.Flowers...),
		LastTile:       win,
		Zimo:           zimo,
		Juezhang:       v.jueZhang(win),
		Haidi:          v.Haidi,
		Gang:           gang,
		PrevailingWind: v.Wind,
		SeatWind:       v.seatWind(),
	}
}
