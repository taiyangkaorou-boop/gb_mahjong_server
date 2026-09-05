package gbmj

import (
	"strings"
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func TestCountFanReadme18(t *testing.T) {
	s := "[123m,1][123p,1]123m12p44s3p"
	hu, err := JudgeHu(s)
	if err != nil {
		t.Fatal(err)
	}
	if !hu {
		t.Fatalf("expected hu: %s", s)
	}
	r, err := CountFan(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalFan != 18 {
		t.Fatalf("fan=%d want 18 items=%v", r.TotalFan, r.Items)
	}
}

func TestCountFanReadme332(t *testing.T) {
	s := "[CCCC][FFFF][PPPP][EEEE]NN|EE1011|8"
	r, err := CountFan(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalFan != 332 {
		t.Fatalf("fan=%d want 332 items=%v", r.TotalFan, r.Items)
	}
}

func TestParseError(t *testing.T) {
	if _, err := JudgeHu("not-a-hand"); err == nil {
		t.Fatal("want parse error")
	}
}

func TestConvertMatches18Fan(t *testing.T) {
	h := HandView{
		Concealed: []tile.Tile{
			tile.Of(tile.SuitWan, 1), tile.Of(tile.SuitWan, 2), tile.Of(tile.SuitWan, 3),
			tile.Of(tile.SuitBing, 1), tile.Of(tile.SuitBing, 2),
			tile.Of(tile.SuitTiao, 4), tile.Of(tile.SuitTiao, 4),
		},
		LastTile: tile.Of(tile.SuitBing, 3),
		Melds: []Meld{
			{Type: MeldChi, Tiles: []tile.Tile{tile.Of(tile.SuitWan, 1), tile.Of(tile.SuitWan, 2), tile.Of(tile.SuitWan, 3)}, Offer: 1},
			{Type: MeldChi, Tiles: []tile.Tile{tile.Of(tile.SuitBing, 1), tile.Of(tile.SuitBing, 2), tile.Of(tile.SuitBing, 3)}, Offer: 1},
		},
	}
	s := ToLibString(h)
	r, err := CountFan(s)
	if err != nil {
		t.Fatalf("str=%s err=%v", s, err)
	}
	if r.TotalFan != 18 {
		t.Fatalf("str=%s fan=%d want 18", s, r.TotalFan)
	}
}

func TestCalcTing(t *testing.T) {
	s := "3344455566667m "
	ting, err := CalcTing(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(ting) < 1 {
		t.Fatalf("want ting tiles, got %v", ting)
	}
	for _, x := range ting {
		if x.IsHua() {
			t.Fatalf("flower must not be a wait: %v", x)
		}
	}
}

func TestConvertAll42TilesParse(t *testing.T) {
	for _, x := range tile.AllStandard() {
		var s string
		if x.IsHua() {
			s = ToLibString(HandView{Flowers: []tile.Tile{x}})
		} else {
			s = ToLibString(HandView{LastTile: x})
		}
		if !strings.Contains(s, x.String()) {
			t.Errorf("%v missing from %s", x, s)
		}
	}
}

func TestMeldAndFlagStrings(t *testing.T) {
	p4 := tile.Of(tile.SuitBing, 4)
	s1 := tile.Of(tile.SuitTiao, 1)
	p3 := tile.Of(tile.SuitBing, 3)
	chi := []tile.Tile{tile.Of(tile.SuitWan, 3), tile.Of(tile.SuitWan, 4), tile.Of(tile.SuitWan, 5)}
	h := HandView{
		Melds: []Meld{
			{Type: MeldAnGang, Tiles: []tile.Tile{p4, p4, p4, p4}},
			{Type: MeldMingGang, Tiles: []tile.Tile{s1, s1, s1, s1}, Offer: 1},
			{Type: MeldJiaGang, Tiles: []tile.Tile{p3, p3, p3, p3}, Offer: 3},
			{Type: MeldChi, Tiles: chi, Offer: 1, Mid: tile.Of(tile.SuitWan, 4)},
		},
		Zimo:           true,
		Juezhang:       false,
		Haidi:          true,
		Gang:           true,
		PrevailingWind: tile.East,
		SeatWind:       tile.East,
	}
	s := ToLibString(h)
	for _, want := range []string{"[4444p]", "[1111s,1]", "[3333p,7]", "[345m,1]", "|EE1011|"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}
