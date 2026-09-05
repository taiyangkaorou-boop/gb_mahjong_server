package game

import (
	"math/rand"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/settle"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseSelfAct
	PhaseResp
	PhaseQiang
	PhaseOver
)

type ActType int

const (
	ActPass ActType = iota + 1
	ActDiscard
	ActChi
	ActPeng
	ActGangMing
	ActAnGang
	ActJiaGang
	ActHu
	ActDraw
	ActBuhua
)

type Action struct {
	TurnID uint32
	Type   ActType
	Tile   tile.Tile
	ChiMid tile.Tile
}

type Event struct {
	Type        ActType
	Seat        int
	Tile        tile.Tile
	Tiles       []tile.Tile
	WallLeft    int
	PrivateSeat int // -1 广播
	Settle      *settle.Result
	DealHands   [4][]tile.Tile
	Banker      int
	Wind        tile.Tile
	TurnID      uint32
	IsDeal      bool
}

type Player struct {
	UID      int64
	Hand     []tile.Tile
	Melds    []rules.Meld
	Flowers  []tile.Tile
	Discards []tile.Tile
	Hosted   bool // 断线后由服务器代打/代过
}

type claim struct {
	got bool
	act Action
}

type HuFunc func(ctx rules.HandContext) (legal, wrong bool, fr rules.FanResult)

// Table 单局牌桌，非并发安全，由房间 Actor 串行调用。
type Table struct {
	Seats            [4]Player
	Wall             Wall
	Banker           int
	Prevailing       tile.Tile
	Phase            Phase
	TurnID           uint32
	Current          int
	LastDiscard      tile.Tile
	LastDiscarder    int
	LastDraw         tile.Tile
	AfterMeldNoKong  bool
	AfterKong        bool
	AfterKongThenHua bool
	QiangGangTile    tile.Tile
	JiaGangSeat      int
	HaidiDraw        bool
	Claims           [4]claim
	Deadline         time.Time
	Timeout          time.Duration
	Judge            HuFunc
	Finished         *settle.Result
	drawnThisTurn    bool
}

func NewTable(uids [4]int64, banker int, timeout time.Duration, judge HuFunc, rng *rand.Rand) *Table {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	t := &Table{
		Banker:     banker,
		Prevailing: tile.East,
		Timeout:    timeout,
		Judge:      judge,
		Current:    banker,
	}
	for i := 0; i < 4; i++ {
		t.Seats[i].UID = uids[i]
	}
	t.Wall = NewWall(rng)
	return t
}

func (t *Table) bump() {
	t.TurnID++
}

func (t *Table) wallLeft() int { return t.Wall.Left() }

func (t *Table) seatWind(seat int) tile.Tile {
	return tile.SeatWind(t.Banker, seat)
}

// WaitingSeats 当前必须提交动作的座位（自己回合或尚未表态的响应方）。
func (t *Table) WaitingSeats() []int {
	if t.Finished != nil || t.Phase == PhaseOver {
		return nil
	}
	switch t.Phase {
	case PhaseSelfAct:
		return []int{t.Current}
	case PhaseResp, PhaseQiang:
		var out []int
		for i := 0; i < 4; i++ {
			if !t.Claims[i].got {
				out = append(out, i)
			}
		}
		return out
	default:
		return nil
	}
}

func (t *Table) visibleCount(x tile.Tile) int {
	n := 0
	for i := 0; i < 4; i++ {
		p := &t.Seats[i]
		n += tile.Count(p.Discards, x)
		for _, m := range p.Melds {
			if m.Type == rules.MeldAnGang {
				continue
			}
			n += tile.Count(m.Tiles, x)
		}
	}
	return n
}

func (t *Table) Context(seat int, win tile.Tile, zimo, gang bool) rules.HandContext {
	p := t.Seats[seat]
	concealed := append([]tile.Tile(nil), p.Hand...)
	if win != 0 {
		if next, ok := tile.RemoveN(concealed, win, 1); ok {
			concealed = next
		}
	}
	return rules.HandContext{
		Concealed:      concealed,
		Melds:          append([]rules.Meld(nil), p.Melds...),
		Flowers:        append([]tile.Tile(nil), p.Flowers...),
		LastTile:       win,
		Zimo:           zimo,
		Juezhang:       t.visibleCount(win) >= 3,
		Haidi:          t.Wall.Left() == 0,
		Gang:           gang,
		PrevailingWind: t.Prevailing,
		SeatWind:       t.seatWind(seat),
	}
}
