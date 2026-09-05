package ai

import (
	"fmt"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// PlayUntilSettle 四座用同一策略打到终局。views 必须已按座位建好。
func PlayUntilSettle(tb *game.Table, views [4]*View, eng rules.Engine, now time.Time) (*settle.Result, error) {
	for step := 0; step < 800; step++ {
		if tb.Finished != nil {
			return tb.Finished, nil
		}
		waiting := tb.WaitingSeats()
		if len(waiting) == 0 {
			return nil, fmt.Errorf("no waiting seat phase=%d", tb.Phase)
		}
		seat := waiting[0]
		act := Decide(views[seat], eng)
		if act.Type == 0 {
			if tb.Phase == game.PhaseResp || tb.Phase == game.PhaseQiang {
				act = game.Action{TurnID: tb.TurnID, Type: game.ActPass}
			} else {
				return nil, fmt.Errorf("seat %d stuck phase=%d hand=%v", seat, tb.Phase, views[seat].Hand)
			}
		}
		act.TurnID = tb.TurnID
		if act.Type == game.ActHu {
			if err := checkHu(tb, views[seat], eng, seat); err != nil {
				return nil, fmt.Errorf("step %d seat %d: %w", step, seat, err)
			}
		}
		views[seat].NoteSent(act)
		evs, err := tb.Apply(seat, act, now)
		if err != nil {
			return nil, fmt.Errorf("step %d seat %d act=%d: %w", step, seat, act.Type, err)
		}
		for i := 0; i < 4; i++ {
			views[i].OnEvents(evs)
			if err := sameHand(views[i].Hand, tb.Seats[i].Hand); err != nil {
				return nil, fmt.Errorf("step %d seat %d %w view=%v table=%v act=%d", step, i, err, views[i].Hand, tb.Seats[i].Hand, act.Type)
			}
			if err := sameMelds(views[i].Melds, tb.Seats[i].Melds); err != nil {
				return nil, fmt.Errorf("step %d seat %d %w", step, i, err)
			}
		}
		now = now.Add(time.Millisecond)
	}
	return nil, fmt.Errorf("too many steps")
}

func sameHand(a, b []tile.Tile) error {
	ca, cb := append([]tile.Tile(nil), a...), append([]tile.Tile(nil), b...)
	tile.Sort(ca)
	tile.Sort(cb)
	if len(ca) != len(cb) {
		return fmt.Errorf("len %d vs %d", len(ca), len(cb))
	}
	for i := range ca {
		if ca[i] != cb[i] {
			return fmt.Errorf("tile %d %v vs %v", i, ca[i], cb[i])
		}
	}
	return nil
}

func sameMelds(a, b []rules.Meld) error {
	if len(a) != len(b) {
		return fmt.Errorf("melds %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Type != b[i].Type || a[i].Mid != b[i].Mid {
			return fmt.Errorf("meld %d type/mid %v/%v vs %v/%v", i, a[i].Type, a[i].Mid, b[i].Type, b[i].Mid)
		}
		if err := sameHand(a[i].Tiles, b[i].Tiles); err != nil {
			return fmt.Errorf("meld %d tiles: %w", i, err)
		}
	}
	return nil
}

func checkHu(tb *game.Table, v *View, eng rules.Engine, seat int) error {
	zimo := tb.Phase == game.PhaseSelfAct
	gang := false
	win := tb.LastDiscard
	if zimo {
		win = tb.LastDraw
		gang = tb.AfterKong && !tb.AfterKongThenHua
	} else if tb.Phase == game.PhaseQiang {
		win = tb.QiangGangTile
		gang = true
	}
	legalT, wrongT, frT, err := settle.Evaluate(eng, tb.Context(seat, win, zimo, gang))
	if err != nil {
		return err
	}
	if !legalT || wrongT {
		return fmt.Errorf("table would reject hu legal=%v wrong=%v fan=%d flower=%d zimo=%v win=%v viewLast=%v viewDraw=%v meldsV=%d meldsT=%d handV=%d handT=%d",
			legalT, wrongT, frT.TotalFan, frT.FlowerFan, zimo, win, v.LastDiscard, v.LastDraw, len(v.Melds), len(tb.Seats[seat].Melds), len(v.Hand), len(tb.Seats[seat].Hand))
	}
	return nil
}

func newViews() [4]*View {
	var vs [4]*View
	for i := 0; i < 4; i++ {
		vs[i] = &View{Seat: i}
	}
	return vs
}

func judgeOf(eng rules.Engine) game.HuFunc {
	return func(ctx rules.HandContext) (bool, bool, rules.FanResult) {
		legal, wrong, fr, err := settle.Evaluate(eng, ctx)
		if err != nil {
			return false, true, fr
		}
		return legal, wrong, fr
	}
}
