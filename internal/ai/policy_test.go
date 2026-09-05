package ai

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

type stubEng struct {
	hu     bool
	fan    int
	flower int
	ting   []tile.Tile
}

func (s stubEng) JudgeHu(rules.HandContext) (bool, error) { return s.hu, nil }
func (s stubEng) CountFan(rules.HandContext) (rules.FanResult, error) {
	return rules.FanResult{TotalFan: s.fan, FlowerFan: s.flower}, nil
}
func (s stubEng) CalcTing(rules.HandContext) ([]tile.Tile, error) { return s.ting, nil }

func wan(r uint8) tile.Tile  { return tile.Of(tile.SuitWan, r) }
func tiao(r uint8) tile.Tile { return tile.Of(tile.SuitTiao, r) }
func bing(r uint8) tile.Tile { return tile.Of(tile.SuitBing, r) }

func TestPolicyHuWhenEightFan(t *testing.T) {
	eng := rules.NewCGOEngine()
	v := &View{
		Seat:        0,
		Banker:      0,
		Wind:        tile.East,
		TurnID:      1,
		NeedSelfAct: true,
		LastDraw:    bing(2),
		Hand:        bankerPphHand(),
	}
	act := Decide(v, eng)
	if act.Type != game.ActHu {
		t.Fatalf("want hu, got type=%d tile=%v", act.Type, act.Tile)
	}
}

func TestPolicyNoHuBelowEight(t *testing.T) {
	v := &View{
		Seat:        0,
		Banker:      0,
		Wind:        tile.East,
		TurnID:      1,
		NeedSelfAct: true,
		LastDraw:    wan(9),
		Hand: []tile.Tile{
			wan(1), wan(2), wan(3), wan(4), wan(5), wan(6),
			tiao(1), tiao(2), tiao(3), bing(7), bing(8), bing(9), wan(9), wan(9),
		},
	}
	act := Decide(v, stubEng{hu: true, fan: 6})
	if act.Type == game.ActHu {
		t.Fatal("must not hu below 8 fan")
	}
	if act.Type != game.ActDiscard {
		t.Fatalf("want discard, got %d", act.Type)
	}
}

func TestPolicyDiscardIsolatedHonor(t *testing.T) {
	west := tile.West
	v := &View{
		Seat:        0,
		Banker:      0,
		Wind:        tile.East,
		TurnID:      1,
		NeedSelfAct: true,
		Hand: []tile.Tile{
			wan(1), wan(2), wan(3), wan(4), wan(5), wan(6),
			bing(7), bing(8), bing(9), tiao(2), tiao(3), tiao(4), tiao(5), west,
		},
	}
	act := Decide(v, stubEng{})
	if act.Type != game.ActDiscard || act.Tile != west {
		t.Fatalf("want discard west, got type=%d tile=%v", act.Type, act.Tile)
	}
}

func TestPolicyChiXiajia(t *testing.T) {
	v := &View{
		Seat:          1,
		Banker:        0,
		Wind:          tile.East,
		TurnID:        2,
		NeedClaim:     true,
		LastDiscard:   wan(1),
		LastDiscarder: 0,
		Hand: []tile.Tile{
			wan(2), wan(3),
			bing(4), bing(5), bing(6),
			tiao(7), tiao(8), tiao(9),
			tiao(1), tiao(1), bing(9), bing(9), tile.South,
		},
	}
	act := Decide(v, stubEng{})
	if act.Type != game.ActChi || act.ChiMid != wan(2) {
		t.Fatalf("want chi 123m, got type=%d mid=%v", act.Type, act.ChiMid)
	}
}

func TestPolicyNoChiNotXiajia(t *testing.T) {
	v := &View{
		Seat:          1,
		Banker:        0,
		Wind:          tile.East,
		TurnID:        2,
		NeedClaim:     true,
		LastDiscard:   wan(1),
		LastDiscarder: 2,
		Hand: []tile.Tile{
			wan(2), wan(3),
			bing(4), bing(5), bing(6),
			tiao(7), tiao(8), tiao(9),
			tiao(1), tiao(1), bing(9), bing(9), tile.South,
		},
	}
	act := Decide(v, stubEng{})
	if act.Type != game.ActPass {
		t.Fatalf("not xiajia must pass, got %d", act.Type)
	}
}

func TestPolicyPengPphShape(t *testing.T) {
	x := bing(4)
	v := &View{
		Seat:          1,
		Banker:        0,
		Wind:          tile.East,
		TurnID:        2,
		NeedClaim:     true,
		LastDiscard:   x,
		LastDiscarder: 0,
		Hand: []tile.Tile{
			wan(1), wan(1), wan(2), wan(2), wan(3), wan(3),
			x, x, tiao(5), tiao(5), tiao(6), tiao(7), tile.South,
		},
	}
	act := Decide(v, stubEng{})
	if act.Type != game.ActPeng {
		t.Fatalf("want peng, got %d", act.Type)
	}
}

func TestPolicyPassWhenNothing(t *testing.T) {
	v := &View{
		Seat:          2,
		NeedClaim:     true,
		LastDiscard:   wan(9),
		LastDiscarder: 0,
		TurnID:        3,
		Hand: []tile.Tile{
			wan(1), wan(2), wan(3), bing(1), bing(2), bing(3),
			tiao(1), tiao(2), tiao(3), tile.South, tile.West, tile.North, wan(5),
		},
	}
	act := Decide(v, stubEng{})
	if act.Type != game.ActPass {
		t.Fatalf("want pass, got %d", act.Type)
	}
}

func TestNoZimoAfterPeng(t *testing.T) {
	x := tile.Zhong
	v := &View{
		Seat:            0,
		Banker:          0,
		Wind:            tile.East,
		TurnID:          3,
		NeedSelfAct:     true,
		AfterMeldNoKong: true,
		LastDraw:        0,
		Melds:           []rules.Meld{{Type: rules.MeldPeng, Mid: x, Tiles: []tile.Tile{x, x, x}}},
		Hand: []tile.Tile{
			tile.East, tile.East, tile.East,
			tile.Fa, tile.Fa, tile.Fa,
			wan(1), wan(1), wan(1),
			bing(2), bing(2),
		},
	}
	act := Decide(v, rules.NewCGOEngine())
	if act.Type == game.ActHu {
		t.Fatal("must discard after peng, not zimo")
	}
	if act.Type != game.ActDiscard {
		t.Fatalf("want discard, got %d", act.Type)
	}
}

func TestNeedClaimClearedOnOtherDraw(t *testing.T) {
	v := &View{
		Seat:          1,
		NeedClaim:     true,
		LastDiscard:   wan(1),
		LastDiscarder: 0,
		TurnID:        2,
		Hand:          []tile.Tile{wan(2), wan(3), bing(5), bing(6), bing(7), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tiao(6), tile.South, tile.West},
	}
	v.OnEvent(game.Event{Type: game.ActDraw, Seat: 2, Tile: 0, TurnID: 3, WallLeft: 80, PrivateSeat: 2})
	if v.NeedClaim {
		t.Fatal("draw should close claim window")
	}
	act := Decide(v, stubEng{})
	if act.Type != 0 {
		t.Fatalf("should not act, got %d", act.Type)
	}
}

func TestPolicyRonRequiresTing(t *testing.T) {
	win := wan(9)
	v := &View{
		Seat:          2,
		NeedClaim:     true,
		LastDiscard:   win,
		LastDiscarder: 0,
		TurnID:        4,
		Hand: []tile.Tile{
			wan(1), wan(2), wan(3), bing(1), bing(2), bing(3),
			tiao(1), tiao(2), tiao(3), wan(4), wan(5), wan(6), wan(7),
		},
	}
	act := Decide(v, stubEng{hu: true, fan: 12, ting: nil})
	if act.Type == game.ActHu {
		t.Fatal("must not ron when not in ting")
	}
	act = Decide(v, stubEng{hu: true, fan: 12, ting: []tile.Tile{win}})
	if act.Type != game.ActHu {
		t.Fatalf("want ron, got %d", act.Type)
	}
}

func bankerPphHand() []tile.Tile {
	e, c, f := tile.East, tile.Zhong, tile.Fa
	w1, w2 := wan(1), bing(2)
	return []tile.Tile{
		e, e, e, c, c, c, f, f, f, w1, w1, w1, w2, w2,
	}
}
