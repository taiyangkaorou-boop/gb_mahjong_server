package game

import (
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func alwaysHu(ctx rules.HandContext) (bool, bool, rules.FanResult) {
	return true, false, rules.FanResult{TotalFan: 12, FlowerFan: 0}
}

func neverHu(ctx rules.HandContext) (bool, bool, rules.FanResult) {
	return false, true, rules.FanResult{}
}

func TestChiOptions(t *testing.T) {
	hand := []tile.Tile{tile.Of(tile.SuitWan, 2), tile.Of(tile.SuitWan, 3)}
	mids := ChiOptions(hand, tile.Of(tile.SuitWan, 1))
	if len(mids) != 1 || mids[0] != tile.Of(tile.SuitWan, 2) {
		t.Fatalf("%v", mids)
	}
}

func TestBankerDiscardsFirst(t *testing.T) {
	wall := make([]tile.Tile, 0, 144)
	one := tile.Of(tile.SuitWan, 1)
	for i := 0; i < 144; i++ {
		wall = append(wall, one)
	}
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	tb.Wall = WallFrom(wall)
	now := time.Now()
	evs, err := tb.Deal(now)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseSelfAct || tb.Current != 0 {
		t.Fatalf("phase=%d current=%d", tb.Phase, tb.Current)
	}
	if len(tb.Seats[0].Hand) != 14 || len(tb.Seats[1].Hand) != 13 {
		t.Fatalf("hands %d %d", len(tb.Seats[0].Hand), len(tb.Seats[1].Hand))
	}
	foundDeal := false
	for _, e := range evs {
		if e.IsDeal {
			foundDeal = true
		}
	}
	if !foundDeal {
		t.Fatal("missing deal event")
	}
	_, err = tb.Apply(1, Action{TurnID: tb.TurnID, Type: ActDiscard, Tile: one}, now)
	if err == nil {
		t.Fatal("non-banker should not discard first")
	}
}

func TestHuPriorityOverPeng(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, alwaysHu, nil)
	one := tile.Of(tile.SuitWan, 1)
	tb.Wall = WallFrom([]tile.Tile{one, one, one, one})
	tb.Phase = PhaseResp
	tb.TurnID = 1
	tb.LastDiscard = one
	tb.LastDiscarder = 0
	tb.Seats[1].Hand = []tile.Tile{one, one, tile.Of(tile.SuitWan, 9)}
	tb.Seats[2].Hand = []tile.Tile{one, one, tile.Of(tile.SuitWan, 8)}
	tb.resetClaims(0)
	now := time.Now()
	if _, err := tb.Apply(1, Action{TurnID: 1, Type: ActPeng, Tile: one}, now); err != nil {
		t.Fatal(err)
	}
	evs, err := tb.Apply(2, Action{TurnID: 1, Type: ActHu, Tile: one}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(3, Action{TurnID: 1, Type: ActPass}, now); err != nil && tb.Finished == nil {
		t.Fatal(err)
	}
	if tb.Finished == nil {
		for _, e := range evs {
			if e.Settle != nil {
				tb.Finished = e.Settle
			}
		}
	}
	if tb.Finished == nil || tb.Finished.Winner != 2 {
		t.Fatalf("want seat2 hu, got %+v", tb.Finished)
	}
}

func TestMeldForbidsKong(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitWan, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 1
	tb.TurnID = 3
	tb.AfterMeldNoKong = true
	tb.Seats[1].Hand = []tile.Tile{one, one, one, one, tile.Of(tile.SuitWan, 2)}
	_, err := tb.Apply(1, Action{TurnID: 3, Type: ActAnGang, Tile: one}, time.Now())
	if err != ErrBadAction {
		t.Fatalf("err=%v", err)
	}
}

func TestTurnIDRejected(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitWan, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 5
	tb.Seats[0].Hand = []tile.Tile{one, one, one, one, one, one, one, one, one, one, one, one, one, one}
	_, err := tb.Apply(0, Action{TurnID: 4, Type: ActDiscard, Tile: one}, time.Now())
	if err != ErrBadTurn {
		t.Fatalf("err=%v", err)
	}
	if _, err := tb.Apply(0, Action{TurnID: 0, Type: ActDiscard, Tile: one}, time.Now()); err != ErrBadTurn {
		t.Fatalf("zero turn_id err=%v", err)
	}
}

func TestContextStripsWinTile(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	win := tile.Of(tile.SuitWan, 3)
	hand := make([]tile.Tile, 13)
	for i := range hand {
		hand[i] = tile.Of(tile.SuitWan, 1)
	}
	hand = append(hand, win)
	tb.Seats[0].Hand = hand
	ctx := tb.Context(0, win, true, false)
	if len(ctx.Concealed) != 13 || ctx.LastTile != win {
		t.Fatalf("concealed=%d last=%v", len(ctx.Concealed), ctx.LastTile)
	}
	if tile.Count(ctx.Concealed, win) != 0 {
		t.Fatalf("win tile still in concealed")
	}
}

func TestAnGangDoesNotOpenQiang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitBing, 2)
	fill := tile.Of(tile.SuitWan, 9)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 1
	tb.LastDraw = one
	h := []tile.Tile{one, one, one, one}
	for len(h) < 14 {
		h = append(h, fill)
	}
	tb.Seats[0].Hand = h
	tb.Wall = WallFrom([]tile.Tile{fill, fill, fill, fill, fill, fill, fill, fill})
	evs, err := tb.Apply(0, Action{TurnID: 1, Type: ActAnGang, Tile: one}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if tb.Phase == PhaseQiang {
		t.Fatalf("an-gang must not open qiang window, evs=%d phase=%d", len(evs), tb.Phase)
	}
}

func TestJieHuNearest(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, alwaysHu, nil)
	one := tile.Of(tile.SuitBing, 5)
	tb.Phase = PhaseResp
	tb.TurnID = 1
	tb.LastDiscard = one
	tb.LastDiscarder = 0
	tb.resetClaims(0)
	now := time.Now()
	_, _ = tb.Apply(2, Action{TurnID: 1, Type: ActHu}, now)
	_, _ = tb.Apply(3, Action{TurnID: 1, Type: ActHu}, now)
	evs, err := tb.Apply(1, Action{TurnID: 1, Type: ActHu}, now)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil {
		for _, e := range evs {
			if e.Settle != nil {
				tb.Finished = e.Settle
			}
		}
	}
	if tb.Finished == nil || tb.Finished.Winner != 1 {
		t.Fatalf("want xiajia seat1, got %+v", tb.Finished)
	}
}

func TestTimeoutPlaysToHuang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400 && tb.Finished == nil; i++ {
		now = now.Add(2 * time.Second)
		if _, err := tb.Tick(now); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if tb.Finished == nil {
		t.Fatal("game did not end")
	}
	if tb.Finished.Kind != settle.KindHuang && tb.Finished.Kind != settle.KindWrong {
		t.Fatalf("kind=%d want huang/wrong", tb.Finished.Kind)
	}
}

func TestDealDoesNotEmitDraw(t *testing.T) {
	wall := make([]tile.Tile, 0, 144)
	one := tile.Of(tile.SuitWan, 1)
	for i := 0; i < 144; i++ {
		wall = append(wall, one)
	}
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	tb.Wall = WallFrom(wall)
	evs, err := tb.Deal(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Type == ActDraw {
			t.Fatal("opening flower replace must not emit DRAW")
		}
	}
}

func TestWrongHuEndsGame(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitWan, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 1
	tb.LastDraw = one
	h := make([]tile.Tile, 14)
	for i := range h {
		h[i] = one
	}
	tb.Seats[0].Hand = h
	evs, err := tb.Apply(0, Action{TurnID: 1, Type: ActHu}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil {
		for _, e := range evs {
			if e.Settle != nil {
				tb.Finished = e.Settle
			}
		}
	}
	if tb.Finished == nil || tb.Finished.Kind != settle.KindWrong {
		t.Fatalf("%+v", tb.Finished)
	}
	if tb.Finished.Scores[0] != -30 || tb.Finished.Scores[1] != 10 {
		t.Fatalf("scores %v", tb.Finished.Scores)
	}
}

func TestChiOnlyXiajia(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	w1 := tile.Of(tile.SuitWan, 1)
	w2 := tile.Of(tile.SuitWan, 2)
	w3 := tile.Of(tile.SuitWan, 3)
	tb.Phase = PhaseResp
	tb.TurnID = 1
	tb.LastDiscard = w1
	tb.LastDiscarder = 0
	tb.Seats[1].Hand = []tile.Tile{w2, w3}
	tb.Seats[2].Hand = []tile.Tile{w2, w3}
	tb.resetClaims(0)
	now := time.Now()
	if _, err := tb.Apply(2, Action{TurnID: 1, Type: ActChi, ChiMid: w2}, now); err != ErrBadAction {
		t.Fatalf("duijia chi err=%v", err)
	}
	if _, err := tb.Apply(2, Action{TurnID: 1, Type: ActPass}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(1, Action{TurnID: 1, Type: ActChi, ChiMid: w2}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(3, Action{TurnID: 1, Type: ActPass}, now); err != nil {
		t.Fatal(err)
	}
	m := tb.Seats[1].Melds
	if len(m) != 1 || len(m[0].Tiles) != 3 || m[0].Tiles[0] != w1 || m[0].Tiles[1] != w2 || m[0].Tiles[2] != w3 {
		t.Fatalf("chi tiles %v", m)
	}
}

func TestPengBeatsChi(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	w1 := tile.Of(tile.SuitWan, 1)
	w2 := tile.Of(tile.SuitWan, 2)
	w3 := tile.Of(tile.SuitWan, 3)
	tb.Phase = PhaseResp
	tb.TurnID = 1
	tb.LastDiscard = w1
	tb.LastDiscarder = 0
	tb.Seats[0].Discards = []tile.Tile{w1}
	tb.Seats[1].Hand = []tile.Tile{w2, w3}
	tb.Seats[2].Hand = []tile.Tile{w1, w1, tile.Of(tile.SuitBing, 9)}
	tb.resetClaims(0)
	now := time.Now()
	if _, err := tb.Apply(1, Action{TurnID: 1, Type: ActChi, ChiMid: w2}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(2, Action{TurnID: 1, Type: ActPeng, Tile: w1}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(3, Action{TurnID: 1, Type: ActPass}, now); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseSelfAct || tb.Current != 2 {
		t.Fatalf("phase=%d current=%d", tb.Phase, tb.Current)
	}
}

func TestHaidiForbidsChiPengGang(t *testing.T) {
	x := tile.Of(tile.SuitWan, 2)
	now := time.Now()
	mk := func() *Table {
		tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, alwaysHu, nil)
		tb.Wall = WallFrom(nil)
		tb.Phase = PhaseResp
		tb.TurnID = 1
		tb.LastDiscard = x
		tb.LastDiscarder = 0
		tb.Seats[1].Hand = []tile.Tile{tile.Of(tile.SuitWan, 1), tile.Of(tile.SuitWan, 3), x, x, x}
		tb.resetClaims(0)
		return tb
	}
	if _, err := mk().Apply(1, Action{TurnID: 1, Type: ActChi, ChiMid: x}, now); err != ErrBadAction {
		t.Fatalf("chi err=%v", err)
	}
	if _, err := mk().Apply(1, Action{TurnID: 1, Type: ActPeng, Tile: x}, now); err != ErrBadAction {
		t.Fatalf("peng err=%v", err)
	}
	if _, err := mk().Apply(1, Action{TurnID: 1, Type: ActGangMing, Tile: x}, now); err != ErrBadAction {
		t.Fatalf("ming gang err=%v", err)
	}
	tb := mk()
	if _, err := tb.Apply(1, Action{TurnID: 1, Type: ActHu}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(2, Action{TurnID: 1, Type: ActPass}, now); err != nil && tb.Finished == nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(3, Action{TurnID: 1, Type: ActPass}, now); err != nil && tb.Finished == nil {
		t.Fatal(err)
	}
	if tb.Finished == nil || tb.Finished.Winner != 1 {
		t.Fatalf("haidi hu %+v", tb.Finished)
	}
}

func TestHaidiForbidsAnGang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	x := tile.Of(tile.SuitWan, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 2
	tb.HaidiDraw = true
	tb.LastDraw = x
	tb.Seats[0].Hand = []tile.Tile{x, x, x, x, tile.Of(tile.SuitBing, 2)}
	if _, err := tb.Apply(0, Action{TurnID: 2, Type: ActAnGang, Tile: x}, time.Now()); err != ErrBadAction {
		t.Fatalf("err=%v", err)
	}
	if _, err := tb.Apply(0, Action{TurnID: 2, Type: ActJiaGang, Tile: x}, time.Now()); err != ErrBadAction {
		t.Fatalf("jia err=%v", err)
	}
}

func TestWallEmptyAllPassHuang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitWan, 9)
	tb.Wall = WallFrom(nil)
	tb.Phase = PhaseResp
	tb.TurnID = 1
	tb.LastDiscard = one
	tb.LastDiscarder = 0
	tb.resetClaims(0)
	now := time.Now()
	_, _ = tb.Apply(1, Action{TurnID: 1, Type: ActPass}, now)
	_, _ = tb.Apply(2, Action{TurnID: 1, Type: ActPass}, now)
	evs, err := tb.Apply(3, Action{TurnID: 1, Type: ActPass}, now)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil {
		for _, e := range evs {
			if e.Settle != nil {
				tb.Finished = e.Settle
			}
		}
	}
	if tb.Finished == nil || tb.Finished.Kind != settle.KindHuang {
		t.Fatalf("%+v", tb.Finished)
	}
}

func TestLastFlowerReplaceHuang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	hua := tile.Of(tile.SuitHua, 1)
	tb.Seats[0].Hand = []tile.Tile{hua, tile.Of(tile.SuitWan, 1)}
	tb.Wall = WallFrom(nil)
	evs, err := tb.replaceFlowers(0, false)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseOver {
		t.Fatalf("phase=%d evs=%d", tb.Phase, len(evs))
	}
	if tb.Finished == nil || tb.Finished.Kind != settle.KindHuang {
		t.Fatalf("%+v", tb.Finished)
	}
}

func TestQiangGangHu(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, alwaysHu, nil)
	x := tile.Of(tile.SuitWan, 5)
	fill := tile.Of(tile.SuitBing, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 2
	tb.LastDraw = x
	tb.Seats[0].Melds = []rules.Meld{{Type: rules.MeldPeng, Tiles: []tile.Tile{x, x, x}, Offer: 1, Mid: x}}
	h := []tile.Tile{x}
	for len(h) < 11 {
		h = append(h, fill)
	}
	tb.Seats[0].Hand = h
	now := time.Now()
	if _, err := tb.Apply(0, Action{TurnID: 2, Type: ActJiaGang, Tile: x}, now); err != nil {
		t.Fatal(err)
	}
	tid := tb.TurnID
	_, _ = tb.Apply(2, Action{TurnID: tid, Type: ActPass}, now)
	_, _ = tb.Apply(3, Action{TurnID: tid, Type: ActPass}, now)
	evs, err := tb.Apply(1, Action{TurnID: tid, Type: ActHu}, now)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil {
		for _, e := range evs {
			if e.Settle != nil {
				tb.Finished = e.Settle
			}
		}
	}
	if tb.Finished == nil || tb.Finished.Winner != 1 || tb.Finished.Kind != settle.KindQiang {
		t.Fatalf("%+v", tb.Finished)
	}
}

func TestJuezhangIgnoresAnGang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	x := tile.Of(tile.SuitWan, 2)
	tb.Seats[0].Melds = []rules.Meld{{Type: rules.MeldAnGang, Tiles: []tile.Tile{x, x, x, x}, Mid: x}}
	tb.Seats[1].Discards = []tile.Tile{x, x}
	if tb.visibleCount(x) != 2 {
		t.Fatalf("visible=%d want 2 (discards only, an-gang hidden)", tb.visibleCount(x))
	}
	tb.Seats[3].Melds = []rules.Meld{{Type: rules.MeldPeng, Tiles: []tile.Tile{x, x, x}, Mid: x}}
	ctx := tb.Context(0, x, true, false)
	if !ctx.Juezhang {
		t.Fatal("peng + discards should make juezhang")
	}
}

func TestJiaGangOpensQiang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	x := tile.Of(tile.SuitWan, 5)
	fill := tile.Of(tile.SuitBing, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 2
	tb.LastDraw = x
	tb.Seats[0].Melds = []rules.Meld{{Type: rules.MeldPeng, Tiles: []tile.Tile{x, x, x}, Offer: 1, Mid: x}}
	h := []tile.Tile{x}
	for len(h) < 11 {
		h = append(h, fill)
	}
	tb.Seats[0].Hand = h
	if _, err := tb.Apply(0, Action{TurnID: 2, Type: ActJiaGang, Tile: x}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseQiang {
		t.Fatalf("phase=%d", tb.Phase)
	}
}

func TestHostedAutoDiscard(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	tb.Seats[tb.Current].Hosted = true
	if _, err := tb.Tick(now); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseResp {
		t.Fatalf("hosted banker should auto-discard, phase=%d", tb.Phase)
	}
}

func TestWrongHuEnds(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, time.Second, neverHu, nil)
	one := tile.Of(tile.SuitWan, 1)
	tb.Phase = PhaseSelfAct
	tb.Current = 0
	tb.TurnID = 1
	tb.LastDraw = one
	h := make([]tile.Tile, 14)
	for i := range h {
		h[i] = one
	}
	tb.Seats[0].Hand = h
	if _, err := tb.Apply(0, Action{TurnID: 1, Type: ActHu}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if tb.Finished == nil || tb.Finished.Kind != settle.KindWrong || tb.Finished.Scores[0] != -30 {
		t.Fatalf("want wrong hu -30, got %+v", tb.Finished)
	}
}
