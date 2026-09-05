package gbmj

import (
	"strings"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// MeldType 与规则层对齐的副露类型。
type MeldType int

const (
	MeldChi MeldType = iota + 1
	MeldPeng
	MeldMingGang
	MeldAnGang
	MeldJiaGang
)

// Meld 是一组吃/碰/杠。
type Meld struct {
	Type  MeldType
	Tiles []tile.Tile
	Offer int
	Mid   tile.Tile
}

// HandView 用于拼装库字符串。
type HandView struct {
	Concealed       []tile.Tile
	Melds           []Meld
	Flowers         []tile.Tile
	LastTile        tile.Tile
	Zimo            bool
	Juezhang        bool
	Haidi           bool
	Gang            bool
	PrevailingWind  tile.Tile
	SeatWind        tile.Tile
}

func windChar(t tile.Tile) byte {
	if !t.IsFeng() {
		return 'E'
	}
	return "ESWN"[t.Rank()-1]
}

func tileChars(t tile.Tile) (num byte, suit byte, honor bool) {
	switch t.Suit() {
	case tile.SuitWan:
		return '0' + t.Rank(), 'm', false
	case tile.SuitTiao:
		return '0' + t.Rank(), 's', false
	case tile.SuitBing:
		return '0' + t.Rank(), 'p', false
	case tile.SuitFeng:
		return "ESWN"[t.Rank()-1], 0, true
	case tile.SuitJian:
		return "CFP"[t.Rank()-1], 0, true
	case tile.SuitHua:
		return "abcdefgh"[t.Rank()-1], 0, true
	default:
		return '?', 0, true
	}
}

func encodeGroup(tiles []tile.Tile) string {
	if len(tiles) == 0 {
		return ""
	}
	var b strings.Builder
	i := 0
	for i < len(tiles) {
		t := tiles[i]
		if t.IsShu() {
			j := i
			for j < len(tiles) && tiles[j].IsShu() && tiles[j].Suit() == t.Suit() {
				num, _, _ := tileChars(tiles[j])
				b.WriteByte(num)
				j++
			}
			_, suit, _ := tileChars(t)
			b.WriteByte(suit)
			i = j
			continue
		}
		ch, _, _ := tileChars(t)
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func encodeOne(t tile.Tile) string {
	if t.IsShu() {
		num, suit, _ := tileChars(t)
		return string([]byte{num, suit})
	}
	ch, _, _ := tileChars(t)
	return string([]byte{ch})
}

func meldString(m Meld) string {
	ts := append([]tile.Tile(nil), m.Tiles...)
	tile.Sort(ts)
	body := encodeGroup(ts)
	offer := m.Offer
	switch m.Type {
	case MeldAnGang:
		offer = 0
	case MeldJiaGang:
		if offer < 5 {
			offer += 4
		}
	case MeldChi, MeldPeng, MeldMingGang:
		if offer == 0 {
			offer = 1
		}
	}
	if offer == 0 {
		return "[" + body + "]"
	}
	return "[" + body + "," + itoa(offer) + "]"
}

func itoa(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return "7"
}

// ToLibString 生成库可解析的手牌串。
func ToLibString(h HandView) string {
	var b strings.Builder
	for _, m := range h.Melds {
		b.WriteString(meldString(m))
	}
	standing := append([]tile.Tile(nil), h.Concealed...)
	tile.Sort(standing)
	if h.LastTile != 0 {
		b.WriteString(encodeGroup(standing))
		b.WriteString(encodeOne(h.LastTile))
	} else {
		b.WriteString(encodeGroup(standing))
	}
	b.WriteByte('|')
	pw, sw := h.PrevailingWind, h.SeatWind
	if pw == 0 {
		pw = tile.East
	}
	if sw == 0 {
		sw = tile.East
	}
	b.WriteByte(windChar(pw))
	b.WriteByte(windChar(sw))
	if h.Zimo {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	if h.Juezhang {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	if h.Haidi {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	if h.Gang {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteByte('|')
	flowers := append([]tile.Tile(nil), h.Flowers...)
	tile.Sort(flowers)
	for _, f := range flowers {
		ch, _, _ := tileChars(f)
		b.WriteByte(ch)
	}
	return b.String()
}
