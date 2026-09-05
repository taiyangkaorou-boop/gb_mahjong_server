package ai

import (
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// Decide 根据自己看见的牌选择下一步。Type==0 表示现在不该动手（例如等补花）。
func Decide(v *View, eng rules.Engine) game.Action {
	if v == nil || v.hasHua() {
		return game.Action{}
	}
	if v.NeedSelfAct && len(v.Hand)%3 == 2 {
		return decideSelf(v, eng)
	}
	if v.NeedClaim {
		return decideClaim(v, eng)
	}
	return game.Action{}
}

func decideSelf(v *View, eng rules.Engine) game.Action {
	base := game.Action{TurnID: v.TurnID}
	if canHuSelf(v, eng) {
		base.Type = game.ActHu
		return base
	}
	if !v.AfterMeldNoKong && !v.Haidi {
		if xs := game.AnGangTiles(v.Hand); len(xs) > 0 {
			base.Type = game.ActAnGang
			base.Tile = xs[0]
			return base
		}
		if xs := game.JiaGangTiles(v.Hand, v.Melds); len(xs) > 0 {
			base.Type = game.ActJiaGang
			base.Tile = xs[0]
			return base
		}
	}
	base.Type = game.ActDiscard
	base.Tile = pickDiscard(v, eng)
	return base
}

func decideClaim(v *View, eng rules.Engine) game.Action {
	base := game.Action{TurnID: v.TurnID}
	if len(v.Hand)%3 != 1 {
		base.Type = game.ActPass
		return base
	}
	win := v.LastDiscard
	if v.Qiang {
		if canRon(v, eng, win, true) {
			base.Type = game.ActHu
			return base
		}
		base.Type = game.ActPass
		return base
	}
	if v.LastDiscarder == v.Seat {
		base.Type = game.ActPass
		return base
	}
	if canRon(v, eng, win, false) {
		base.Type = game.ActHu
		return base
	}
	if !v.Haidi && game.CanMingGang(v.Hand, win) {
		base.Type = game.ActGangMing
		base.Tile = win
		return base
	}
	if !v.Haidi && game.CanPeng(v.Hand, win) && wantPeng(v, win) {
		base.Type = game.ActPeng
		base.Tile = win
		return base
	}
	xiajia := (v.LastDiscarder+1)%4 == v.Seat
	if !v.Haidi && xiajia {
		if mid, ok := wantChi(v, win); ok {
			base.Type = game.ActChi
			base.Tile = win
			base.ChiMid = mid
			return base
		}
	}
	base.Type = game.ActPass
	return base
}

func canHuSelf(v *View, eng rules.Engine) bool {
	if v.LastDraw != 0 && tile.Count(v.Hand, v.LastDraw) > 0 {
		return legalHu(v, eng, v.LastDraw, true, v.AfterKong)
	}
	// 吃碰之后必须打牌，不能把立牌拆开当自摸。
	if len(v.Melds) > 0 || v.AfterMeldNoKong {
		return false
	}
	seen := map[tile.Tile]bool{}
	for _, x := range v.Hand {
		if x.IsHua() || seen[x] {
			continue
		}
		seen[x] = true
		if legalHu(v, eng, x, true, false) {
			return true
		}
	}
	return false
}

func canRon(v *View, eng rules.Engine, win tile.Tile, qiang bool) bool {
	if win == 0 || !legalHu(v, eng, win, false, qiang) {
		return false
	}
	if eng == nil {
		return true
	}
	ting, err := eng.CalcTing(v.ctx(0, false, false))
	if err != nil {
		return false
	}
	for _, x := range ting {
		if x == win {
			return true
		}
	}
	return false
}

func legalHu(v *View, eng rules.Engine, win tile.Tile, zimo, gang bool) bool {
	if eng == nil || win == 0 {
		return false
	}
	ctx := v.ctx(win, zimo, gang)
	// 海底/绝张客户端容易算多，不算进起和番，避免错和。
	ctx.Juezhang = false
	ctx.Haidi = false
	legal, wrong, _, err := settle.Evaluate(eng, ctx)
	return err == nil && legal && !wrong
}

func pickDiscard(v *View, eng rules.Engine) tile.Tile {
	if len(v.Hand) == 0 {
		return 0
	}
	uniq := uniqueTiles(v.Hand)
	bestTing, bestN := tile.Tile(0), -1
	for _, x := range uniq {
		n := tingCountAfter(v, eng, x)
		if n > bestN {
			bestN = n
			bestTing = x
		}
	}
	if bestN > 0 {
		return bestTing
	}
	best, score := uniq[0], -1<<30
	for _, x := range uniq {
		s := junkScore(x, v.Hand, v.seatWind(), v.Wind)
		if s > score {
			score = s
			best = x
		}
	}
	return best
}

func tingCountAfter(v *View, eng rules.Engine, drop tile.Tile) int {
	if eng == nil {
		return 0
	}
	left, ok := tile.RemoveN(v.Hand, drop, 1)
	if !ok {
		return 0
	}
	ctx := v.ctx(0, false, false)
	ctx.Concealed = left
	ctx.LastTile = 0
	ting, err := eng.CalcTing(ctx)
	if err != nil {
		return 0
	}
	return len(ting)
}

func uniqueTiles(hand []tile.Tile) []tile.Tile {
	seen := map[tile.Tile]bool{}
	var out []tile.Tile
	for _, x := range hand {
		if x.IsHua() || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

// junkScore 越大越该打出去：孤张字牌优先，门风/箭对子留下。
func junkScore(x tile.Tile, hand []tile.Tile, seatWind, prevailing tile.Tile) int {
	n := tile.Count(hand, x)
	if x.IsHonor() {
		valuable := x.Suit() == tile.SuitJian || x == seatWind || x == prevailing
		if n >= 2 && valuable {
			return -100
		}
		if n >= 2 {
			return -20
		}
		if valuable {
			return 50
		}
		return 100
	}
	if n >= 3 {
		return -80
	}
	if n >= 2 {
		return -40
	}
	pred, succ := x.Pred(), x.Succ()
	adj := (pred != 0 && tile.Count(hand, pred) > 0) || (succ != 0 && tile.Count(hand, succ) > 0)
	skip := false
	if pred != 0 {
		pp := pred.Pred()
		if pp != 0 && tile.Count(hand, pp) > 0 {
			skip = true
		}
	}
	if succ != 0 {
		ss := succ.Succ()
		if ss != 0 && tile.Count(hand, ss) > 0 {
			skip = true
		}
	}
	if adj {
		return -30
	}
	if skip {
		return 10
	}
	if x.Rank() == 1 || x.Rank() == 9 {
		return 80
	}
	return 40
}

func wantPeng(v *View, disc tile.Tile) bool {
	pairs := 0
	seen := map[tile.Tile]bool{}
	for _, x := range v.Hand {
		if seen[x] || x.IsHua() {
			continue
		}
		seen[x] = true
		if tile.Count(v.Hand, x) >= 2 {
			pairs++
		}
	}
	sets := 0
	for _, m := range v.Melds {
		if m.Type == rules.MeldPeng || m.Type == rules.MeldMingGang || m.Type == rules.MeldAnGang || m.Type == rules.MeldJiaGang {
			sets++
		}
	}
	_ = disc
	return sets+pairs >= 3
}

func wantChi(v *View, disc tile.Tile) (tile.Tile, bool) {
	mids := game.ChiOptions(v.Hand, disc)
	if len(mids) == 0 {
		return 0, false
	}
	for _, mid := range mids {
		a, b, ok := game.ChiTiles(mid, disc)
		if !ok {
			continue
		}
		// 用孤张去吃，手牌更整齐；拆对子则不吃。
		if tile.Count(v.Hand, a) == 1 || tile.Count(v.Hand, b) == 1 {
			return mid, true
		}
	}
	return 0, false
}
