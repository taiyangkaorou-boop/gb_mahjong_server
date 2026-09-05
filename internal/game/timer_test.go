package game

import (
	"testing"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
)

func discardCurrent(tb *Table, now time.Time) error {
	p := &tb.Seats[tb.Current]
	if len(p.Hand) == 0 {
		return ErrBadAction
	}
	x := p.Hand[len(p.Hand)-1]
	_, err := tb.Apply(tb.Current, Action{TurnID: tb.TurnID, Type: ActDiscard, Tile: x}, now)
	return err
}

func TestExtraUntouchedWithinTurn(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 100*time.Millisecond, neverHu, nil)
	tb.SetExtra(time.Second)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	if err := discardCurrent(tb, now.Add(50*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if tb.ExtraLeft[0] != time.Second {
		t.Fatalf("extra=%s want 1s", tb.ExtraLeft[0])
	}
}

func TestExtraConsumedAfterTurn(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 100*time.Millisecond, neverHu, nil)
	tb.SetExtra(time.Second)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	if err := discardCurrent(tb, now.Add(150*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	want := time.Second - 50*time.Millisecond
	if tb.ExtraLeft[0] != want {
		t.Fatalf("extra=%s want %s", tb.ExtraLeft[0], want)
	}
	for i := 1; i < 4; i++ {
		if tb.ExtraLeft[i] != time.Second {
			t.Fatalf("seat %d extra=%s should be untouched", i, tb.ExtraLeft[i])
		}
	}
}

func TestTimeoutWaitsForExtraThenAuto(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 100*time.Millisecond, neverHu, nil)
	tb.SetExtra(200 * time.Millisecond)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Tick(now.Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseSelfAct {
		t.Fatalf("still in extra time, phase=%s", tb.Phase)
	}
	if _, err := tb.Tick(now.Add(300 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseResp {
		t.Fatalf("extra used up should auto-discard, phase=%s", tb.Phase)
	}
	if tb.ExtraLeft[0] != 0 {
		t.Fatalf("extra after full timeout: %s", tb.ExtraLeft[0])
	}
}

func TestRespTimeoutOnlyBurnsTimedOutSeat(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 100*time.Millisecond, neverHu, nil)
	tb.SetExtra(time.Second)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	if err := discardCurrent(tb, now); err != nil {
		t.Fatal(err)
	}
	tb.ExtraLeft[1] = 0
	if _, err := tb.Tick(now.Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if !tb.Claims[1].got || tb.Claims[1].act.Type != ActPass {
		t.Fatalf("seat1 should auto-pass: %+v", tb.Claims[1])
	}
	if tb.Claims[2].got || tb.Claims[3].got {
		t.Fatal("seats with extra should still wait")
	}
	if tb.Phase != PhaseResp {
		t.Fatalf("phase=%s", tb.Phase)
	}
	if tb.ExtraLeft[2] != time.Second || tb.ExtraLeft[3] != time.Second {
		t.Fatalf("other extras=%v", tb.ExtraLeft)
	}
}

func TestTimeoutWithExtraStillEndsHuang(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 20*time.Millisecond, neverHu, nil)
	tb.SetExtra(20 * time.Millisecond)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 800 && tb.Finished == nil; i++ {
		now = now.Add(50 * time.Millisecond)
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

func TestWaitMSUsesOwnExtra(t *testing.T) {
	tb := NewTable([4]int64{1, 2, 3, 4}, 0, 100*time.Millisecond, neverHu, nil)
	tb.SetExtra(200 * time.Millisecond)
	now := time.Now()
	if _, err := tb.Deal(now); err != nil {
		t.Fatal(err)
	}
	if got := tb.WaitMS(0, now); got < 250 || got > 300 {
		t.Fatalf("banker wait_ms=%d want ~300", got)
	}
	if got := tb.TurnMS(); got != 100 {
		t.Fatalf("turn_ms=%d", got)
	}
	left := tb.ExtraLeftMS()
	if len(left) != 4 || left[0] != 200 {
		t.Fatalf("extra_left=%v", left)
	}
}
