package rules

import "github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"

// MeldType 副露种类。
type MeldType int

const (
	MeldChi MeldType = iota + 1
	MeldPeng
	MeldMingGang
	MeldAnGang
	MeldJiaGang
)

// Meld 一组吃碰杠。
type Meld struct {
	Type  MeldType
	Tiles []tile.Tile
	Offer int
	Mid   tile.Tile
}

// HandContext 是算番输入。Concealed 为立牌（不含 LastTile）。
type HandContext struct {
	Concealed      []tile.Tile
	Melds          []Meld
	Flowers        []tile.Tile
	LastTile       tile.Tile
	Zimo           bool
	Juezhang       bool
	Haidi          bool
	Gang           bool
	PrevailingWind tile.Tile
	SeatWind       tile.Tile
}

// FanItem 单条番种。
type FanItem struct {
	ID    int
	Score int
	Name  string
}

// FanResult 算番输出。
type FanResult struct {
	TotalFan  int
	FlowerFan int
	Items     []FanItem
}

// Engine 由 CGO 绑定实现。game 包不得直接依赖 CGO。
type Engine interface {
	JudgeHu(ctx HandContext) (bool, error)
	CountFan(ctx HandContext) (FanResult, error)
	CalcTing(ctx HandContext) ([]tile.Tile, error)
}
