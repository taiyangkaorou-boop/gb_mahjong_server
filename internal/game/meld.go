package game

import (
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// ChiOptions 返回可吃的顺子中间牌。
func ChiOptions(hand []tile.Tile, disc tile.Tile) []tile.Tile {
	if !disc.IsShu() {
		return nil
	}
	s, r := disc.Suit(), disc.Rank()
	var mids []tile.Tile
	has := func(rank uint8) bool {
		return tile.Count(hand, tile.Of(s, rank)) > 0
	}
	add := func(mid uint8) {
		mids = append(mids, tile.Of(s, mid))
	}
	if r <= 7 && has(r+1) && has(r+2) {
		add(r + 1)
	}
	if r >= 2 && r <= 8 && has(r-1) && has(r+1) {
		add(r)
	}
	if r >= 3 && has(r-2) && has(r-1) {
		add(r - 1)
	}
	return mids
}

// ChiTiles 由中间牌与打出牌得到要从手牌拿掉的两张。
func ChiTiles(mid, disc tile.Tile) (a, b tile.Tile, ok bool) {
	if !mid.IsShu() || mid.Suit() != disc.Suit() {
		return 0, 0, false
	}
	left := mid.Pred()
	right := mid.Succ()
	if left == 0 || right == 0 {
		return 0, 0, false
	}
	switch disc {
	case left:
		return mid, right, true
	case mid:
		return left, right, true
	case right:
		return left, mid, true
	default:
		return 0, 0, false
	}
}

// CanPeng 手牌至少两张与弃牌相同。
func CanPeng(hand []tile.Tile, disc tile.Tile) bool {
	return tile.Count(hand, disc) >= 2
}

// CanMingGang 手牌至少三张与弃牌相同。
func CanMingGang(hand []tile.Tile, disc tile.Tile) bool {
	return tile.Count(hand, disc) >= 3
}

// AnGangTiles 手牌里可暗杠的牌面。
func AnGangTiles(hand []tile.Tile) []tile.Tile {
	cnt := map[tile.Tile]int{}
	for _, x := range hand {
		if x.IsHua() {
			continue
		}
		cnt[x]++
	}
	var out []tile.Tile
	for x, n := range cnt {
		if n >= 4 {
			out = append(out, x)
		}
	}
	return out
}

// JiaGangTiles 已碰且手里还有一张的牌，可加杠。
func JiaGangTiles(hand []tile.Tile, melds []rules.Meld) []tile.Tile {
	var out []tile.Tile
	for _, m := range melds {
		if m.Type == rules.MeldPeng && m.Mid != 0 && tile.Count(hand, m.Mid) > 0 {
			out = append(out, m.Mid)
		}
	}
	return out
}

// OfferRel 副露来源：1上家 2对家 3下家。
func OfferRel(claimant, discarder int) int {
	switch (discarder - claimant + 4) % 4 {
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return 1
	}
}

func ChiOffer(mid, disc tile.Tile) int {
	if disc == mid.Pred() {
		return 1
	}
	if disc == mid {
		return 2
	}
	return 3
}
