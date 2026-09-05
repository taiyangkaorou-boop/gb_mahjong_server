package game

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

func TestWallDrawReplaceMeet(t *testing.T) {
	a := tile.Of(tile.SuitWan, 1)
	b := tile.Of(tile.SuitWan, 9)
	hua := tile.Of(tile.SuitHua, 1)
	w := WallFrom([]tile.Tile{a, b, hua})
	if w.Left() != 3 {
		t.Fatalf("left=%d", w.Left())
	}
	d, ok := w.Draw()
	if !ok || d != a {
		t.Fatalf("draw %v %v", d, ok)
	}
	r, ok := w.Replace()
	if !ok || r != hua {
		t.Fatalf("replace %v %v", r, ok)
	}
	if w.Left() != 1 {
		t.Fatalf("left after both pointers %d", w.Left())
	}
	mid, ok := w.Draw()
	if !ok || mid != b {
		t.Fatalf("last %v %v", mid, ok)
	}
	if w.Left() != 0 {
		t.Fatal("want empty")
	}
	if _, ok := w.Draw(); ok {
		t.Fatal("draw empty")
	}
	if _, ok := w.Replace(); ok {
		t.Fatal("replace empty")
	}
}

func TestWallEmptySlice(t *testing.T) {
	w := WallFrom(nil)
	if w.Left() != 0 {
		t.Fatalf("left=%d", w.Left())
	}
}
