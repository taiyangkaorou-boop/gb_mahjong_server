package tile

// zheng-fan/GB-Mahjong 内部牌号：1-9万, 10-18条, 19-27饼, 28-31风, 32-34箭, 35-42花。

// ToLibID 把 1 字节编码转为库牌号，无效返回 0。
func ToLibID(t Tile) int {
	if !t.Valid() {
		return 0
	}
	s, r := t.Suit(), t.Rank()
	switch s {
	case SuitWan:
		return int(r)
	case SuitTiao:
		return 9 + int(r)
	case SuitBing:
		return 18 + int(r)
	case SuitFeng:
		return 27 + int(r)
	case SuitJian:
		return 31 + int(r)
	case SuitHua:
		return 34 + int(r)
	default:
		return 0
	}
}

// FromLibID 把库牌号转为 1 字节编码。
func FromLibID(id int) Tile {
	switch {
	case id >= 1 && id <= 9:
		return Of(SuitWan, uint8(id))
	case id >= 10 && id <= 18:
		return Of(SuitTiao, uint8(id-9))
	case id >= 19 && id <= 27:
		return Of(SuitBing, uint8(id-18))
	case id >= 28 && id <= 31:
		return Of(SuitFeng, uint8(id-27))
	case id >= 32 && id <= 34:
		return Of(SuitJian, uint8(id-31))
	case id >= 35 && id <= 42:
		return Of(SuitHua, uint8(id-34))
	default:
		return 0
	}
}
