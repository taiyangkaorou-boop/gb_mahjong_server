package ai

import (
	"math/rand"
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func TestBankerHuOnStackedWall(t *testing.T) {
	eng := rules.NewCGOEngine()
	tb := game.NewTable([4]int64{1, 2, 3, 4}, 0, time.Minute, judgeOf(eng), rand.New(rand.NewSource(1)))
	tb.Wall = game.WallFrom(stackedZimoWall())
	now := time.Unix(1, 0)
	evs, err := tb.Deal(now)
	if err != nil {
		t.Fatal(err)
	}
	views := newViews()
	for i := 0; i < 4; i++ {
		views[i].OnEvents(evs)
	}
	if tb.Phase == game.PhaseOver {
		t.Fatal("deal ended the game")
	}
	act := Decide(views[0], eng)
	if act.Type != game.ActHu {
		t.Fatalf("banker should hu, got type=%d tile=%v hand=%v", act.Type, act.Tile, views[0].Hand)
	}
	act.TurnID = tb.TurnID
	evs, err = tb.Apply(0, act, now)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil || tb.Finished.Kind != settle.KindZimo {
		t.Fatalf("want zimo, got %+v evs=%d", tb.Finished, len(evs))
	}
}

func TestFourBotsOneHand(t *testing.T) {
	eng := rules.NewCGOEngine()
	for _, seed := range []int64{1, 7, 42, 99, 2026} {
		tb := game.NewTable([4]int64{1, 2, 3, 4}, 0, time.Minute, judgeOf(eng), rand.New(rand.NewSource(seed)))
		now := time.Unix(1, 0)
		evs, err := tb.Deal(now)
		if err != nil {
			t.Fatal(err)
		}
		views := newViews()
		for i := 0; i < 4; i++ {
			views[i].OnEvents(evs)
		}
		res, err := PlayUntilSettle(tb, views, eng, now)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res == nil {
			t.Fatal("no settle")
		}
		if res.Kind == settle.KindWrong {
			t.Fatalf("seed %d wrong hu", seed)
		}
		t.Logf("seed=%d kind=%d fan=%d scores=%v", seed, res.Kind, res.Fan, res.Scores)
	}
}

func stackedZimoWall() []tile.Tile {
	banker := bankerPphHand()
	pool := tile.FullWall()
	for _, x := range banker {
		ok := false
		pool, ok = tile.RemoveN(pool, x, 1)
		if !ok {
			panic("wall missing tile")
		}
	}
	wall := make([]tile.Tile, 144)
	bi, oi := 0, 0
	for i := 0; i < 53; i++ {
		if i%4 == 0 && bi < len(banker) {
			wall[i] = banker[bi]
			bi++
			continue
		}
		wall[i] = pool[oi]
		oi++
	}
	for i := 53; i < 144; i++ {
		wall[i] = pool[oi]
		oi++
	}
	return wall
}
