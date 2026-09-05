package rules

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func TestCGOEngineReadmeHands(t *testing.T) {
	eng := NewCGOEngine()
	ctx18 := HandContext{
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
		PrevailingWind: tile.East,
		SeatWind:       tile.East,
	}
	ok, err := eng.JudgeHu(ctx18)
	if err != nil || !ok {
		t.Fatalf("hu %v %v", ok, err)
	}
	fr, err := eng.CountFan(ctx18)
	if err != nil {
		t.Fatal(err)
	}
	if fr.TotalFan != 18 {
		t.Fatalf("fan=%d items=%v", fr.TotalFan, fr.Items)
	}

	var flowers []tile.Tile
	for r := uint8(1); r <= 8; r++ {
		flowers = append(flowers, tile.Of(tile.SuitHua, r))
	}
	quad := func(x tile.Tile) []tile.Tile { return []tile.Tile{x, x, x, x} }
	ctx332 := HandContext{
		Concealed: []tile.Tile{tile.North, tile.North},
		Melds: []Meld{
			{Type: MeldAnGang, Tiles: quad(tile.Zhong)},
			{Type: MeldAnGang, Tiles: quad(tile.Fa)},
			{Type: MeldAnGang, Tiles: quad(tile.Bai)},
			{Type: MeldAnGang, Tiles: quad(tile.East)},
		},
		Flowers:        flowers,
		Zimo:           true,
		Haidi:          true,
		Gang:           true,
		PrevailingWind: tile.East,
		SeatWind:       tile.East,
	}
	fr, err = eng.CountFan(ctx332)
	if err != nil {
		t.Fatal(err)
	}
	if fr.TotalFan != 332 {
		t.Fatalf("fan=%d flower=%d items=%v", fr.TotalFan, fr.FlowerFan, fr.Items)
	}
	if fr.FlowerFan != 8 {
		t.Fatalf("flower fan=%d", fr.FlowerFan)
	}
}
