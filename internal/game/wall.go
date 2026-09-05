package game

import (
	"math/rand"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// Wall 头摸尾补双指针牌墙。
type Wall struct {
	tiles     []tile.Tile
	drawAt    int
	replaceAt int
}

func NewWall(rng *rand.Rand) Wall {
	ts := tile.FullWall()
	rng.Shuffle(len(ts), func(i, j int) { ts[i], ts[j] = ts[j], ts[i] })
	return Wall{tiles: ts, drawAt: 0, replaceAt: len(ts) - 1}
}

func WallFrom(ts []tile.Tile) Wall {
	cp := append([]tile.Tile(nil), ts...)
	return Wall{tiles: cp, drawAt: 0, replaceAt: len(cp) - 1}
}

func (w *Wall) Left() int {
	if w.drawAt > w.replaceAt {
		return 0
	}
	return w.replaceAt - w.drawAt + 1
}

func (w *Wall) Draw() (tile.Tile, bool) {
	if w.Left() == 0 {
		return 0, false
	}
	x := w.tiles[w.drawAt]
	w.drawAt++
	return x, true
}

func (w *Wall) Replace() (tile.Tile, bool) {
	if w.Left() == 0 {
		return 0, false
	}
	x := w.tiles[w.replaceAt]
	w.replaceAt--
	return x, true
}
