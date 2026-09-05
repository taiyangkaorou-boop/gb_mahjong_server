// Package tile 定义国标麻将 1 字节紧凑编码。
//
//	byte = (suit << 4) | rank
//	suit: 1万 2条 3饼 4风 5箭 6花
//	rank: 数牌 1-9；风 1东 2南 3西 4北；箭 1中 2发 3白；花 1-8 梅兰竹菊春夏秋冬
package tile

import "fmt"

// Tile 是线上/内存使用的单字节牌编码。0 表示无效。
type Tile uint8

const (
	SuitNone   uint8 = 0
	SuitWan    uint8 = 1
	SuitTiao   uint8 = 2
	SuitBing   uint8 = 3
	SuitFeng   uint8 = 4
	SuitJian   uint8 = 5
	SuitHua    uint8 = 6

	RankNone uint8 = 0

	// 风/箭/花的点数
	FengEast, FengSouth, FengWest, FengNorth = 1, 2, 3, 4
	JianZhong, JianFa, JianBai               = 1, 2, 3
)

// 常用牌常量，便于测试与风圈标记。
var (
	East  = Of(SuitFeng, FengEast)
	South = Of(SuitFeng, FengSouth)
	West  = Of(SuitFeng, FengWest)
	North = Of(SuitFeng, FengNorth)
	Zhong = Of(SuitJian, JianZhong)
	Fa    = Of(SuitJian, JianFa)
	Bai   = Of(SuitJian, JianBai)
)

// Of 按花色与点数构造牌。
func Of(suit, rank uint8) Tile {
	return Tile((suit << 4) | (rank & 0x0F))
}

func (t Tile) Suit() uint8 { return uint8(t >> 4) }
func (t Tile) Rank() uint8 { return uint8(t & 0x0F) }
func (t Tile) Valid() bool {
	s, r := t.Suit(), t.Rank()
	switch s {
	case SuitWan, SuitTiao, SuitBing:
		return r >= 1 && r <= 9
	case SuitFeng:
		return r >= 1 && r <= 4
	case SuitJian:
		return r >= 1 && r <= 3
	case SuitHua:
		return r >= 1 && r <= 8
	default:
		return false
	}
}

func (t Tile) IsShu() bool {
	s := t.Suit()
	return (s == SuitWan || s == SuitTiao || s == SuitBing) && t.Rank() >= 1 && t.Rank() <= 9
}

func (t Tile) IsHonor() bool {
	return t.Suit() == SuitFeng || t.Suit() == SuitJian
}

func (t Tile) IsHua() bool { return t.Suit() == SuitHua && t.Valid() }

func (t Tile) IsFeng() bool { return t.Suit() == SuitFeng && t.Valid() }

// Same 比较牌面（不含任何摸打标记，本编码本身无标记）。
func (t Tile) Same(o Tile) bool { return t == o }

// Pred/Succ 仅对数牌有意义。
func (t Tile) Pred() Tile {
	if !t.IsShu() || t.Rank() <= 1 {
		return 0
	}
	return Of(t.Suit(), t.Rank()-1)
}
func (t Tile) Succ() Tile {
	if !t.IsShu() || t.Rank() >= 9 {
		return 0
	}
	return Of(t.Suit(), t.Rank()+1)
}

// String 调试用，例如 3m、E、C、a。
func (t Tile) String() string {
	if t == 0 {
		return "?"
	}
	s, r := t.Suit(), t.Rank()
	switch s {
	case SuitWan:
		return fmt.Sprintf("%dm", r)
	case SuitTiao:
		return fmt.Sprintf("%ds", r)
	case SuitBing:
		return fmt.Sprintf("%dp", r)
	case SuitFeng:
		return string("ESWN"[r-1 : r])
	case SuitJian:
		return string("CFP"[r-1 : r])
	case SuitHua:
		return string("abcdefgh"[r-1 : r])
	default:
		return fmt.Sprintf("0x%02x", uint8(t))
	}
}

// AllStandard 返回 42 种牌面各一张（不含重复张）。
func AllStandard() []Tile {
	out := make([]Tile, 0, 42)
	for s := SuitWan; s <= SuitBing; s++ {
		for r := uint8(1); r <= 9; r++ {
			out = append(out, Of(s, r))
		}
	}
	for r := uint8(1); r <= 4; r++ {
		out = append(out, Of(SuitFeng, r))
	}
	for r := uint8(1); r <= 3; r++ {
		out = append(out, Of(SuitJian, r))
	}
	for r := uint8(1); r <= 8; r++ {
		out = append(out, Of(SuitHua, r))
	}
	return out
}

// FullWall 生成 144 张牌墙：序数/字牌各 4 张，花牌各 1 张。
func FullWall() []Tile {
	out := make([]Tile, 0, 144)
	for s := SuitWan; s <= SuitBing; s++ {
		for r := uint8(1); r <= 9; r++ {
			t := Of(s, r)
			for i := 0; i < 4; i++ {
				out = append(out, t)
			}
		}
	}
	for r := uint8(1); r <= 4; r++ {
		t := Of(SuitFeng, r)
		for i := 0; i < 4; i++ {
			out = append(out, t)
		}
	}
	for r := uint8(1); r <= 3; r++ {
		t := Of(SuitJian, r)
		for i := 0; i < 4; i++ {
			out = append(out, t)
		}
	}
	for r := uint8(1); r <= 8; r++ {
		out = append(out, Of(SuitHua, r))
	}
	return out
}

// Count 统计手牌中某张的数量。
func Count(tiles []Tile, t Tile) int {
	n := 0
	for _, x := range tiles {
		if x == t {
			n++
		}
	}
	return n
}

// RemoveN 从切片中移除 n 张 t，返回新切片。找不到足够张则 ok=false。
func RemoveN(tiles []Tile, t Tile, n int) ([]Tile, bool) {
	if n <= 0 {
		return tiles, true
	}
	out := make([]Tile, 0, len(tiles))
	left := n
	for _, x := range tiles {
		if left > 0 && x == t {
			left--
			continue
		}
		out = append(out, x)
	}
	if left != 0 {
		return tiles, false
	}
	return out, true
}

// Sort 按编码值排序（花色再点数）。
func Sort(tiles []Tile) {
	for i := 1; i < len(tiles); i++ {
		j := i
		for j > 0 && tiles[j] < tiles[j-1] {
			tiles[j], tiles[j-1] = tiles[j-1], tiles[j]
			j--
		}
	}
}

// NextWind 逆时针下一门风：东南西北。
func NextWind(w Tile) Tile {
	if !w.IsFeng() {
		return East
	}
	r := w.Rank() + 1
	if r > 4 {
		r = 1
	}
	return Of(SuitFeng, r)
}

// SeatWind 庄家为东，seat 相对庄家的门风。
func SeatWind(banker, seat int) Tile {
	d := (seat - banker + 4) % 4
	return Of(SuitFeng, uint8(d+1))
}
