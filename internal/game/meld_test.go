package game

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func TestChiOptionsAndTiles(t *testing.T) {
	w1 := tile.Of(tile.SuitWan, 1)
	w2 := tile.Of(tile.SuitWan, 2)
	w3 := tile.Of(tile.SuitWan, 3)
	hand := []tile.Tile{w2, w3}
	mids := ChiOptions(hand, w1)
	if len(mids) != 1 || mids[0] != w2 {
		t.Fatalf("chi left %v", mids)
	}
	a, b, ok := ChiTiles(w2, w1)
	if !ok || a != w2 || b != w3 {
		t.Fatalf("ChiTiles %v %v %v", a, b, ok)
	}
	if ChiOptions(hand, tile.East) != nil {
		t.Fatal("cannot chi honor")
	}
	if _, _, ok := ChiTiles(w2, tile.Of(tile.SuitBing, 1)); ok {
		t.Fatal("cross-suit chi")
	}
}

func TestPengGangHelpers(t *testing.T) {
	x := tile.Of(tile.SuitBing, 5)
	hand := []tile.Tile{x, x, x, tile.Of(tile.SuitWan, 9)}
	if !CanPeng(hand, x) || !CanMingGang(hand, x) {
		t.Fatal("peng/gang")
	}
	if CanPeng(hand, tile.Of(tile.SuitBing, 6)) {
		t.Fatal("no peng")
	}
	four := []tile.Tile{x, x, x, x, tile.Of(tile.SuitHua, 1)}
	ags := AnGangTiles(four)
	if len(ags) != 1 || ags[0] != x {
		t.Fatalf("an gang %v", ags)
	}
	melds := []rules.Meld{{Type: rules.MeldPeng, Mid: x, Tiles: []tile.Tile{x, x, x}}}
	one := []tile.Tile{x, tile.Of(tile.SuitWan, 9)}
	jg := JiaGangTiles(one, melds)
	if len(jg) != 1 || jg[0] != x {
		t.Fatalf("jia gang %v", jg)
	}
}

func TestOfferRelAndChiOffer(t *testing.T) {
	if OfferRel(1, 0) != 1 || OfferRel(2, 0) != 2 || OfferRel(3, 0) != 3 {
		t.Fatal("OfferRel")
	}
	mid := tile.Of(tile.SuitWan, 2)
	if ChiOffer(mid, mid.Pred()) != 1 || ChiOffer(mid, mid) != 2 || ChiOffer(mid, mid.Succ()) != 3 {
		t.Fatal("ChiOffer")
	}
}
