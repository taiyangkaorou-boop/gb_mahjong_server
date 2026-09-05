package rules

import (
	"sync"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/cgo/gbmj"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// CGOEngine 通过手牌字符串调用 C++ 库。四个机器人会并发调用，加锁。
type CGOEngine struct{ mu sync.Mutex }

func NewCGOEngine() *CGOEngine { return &CGOEngine{} }

func toView(ctx HandContext) gbmj.HandView {
	melds := make([]gbmj.Meld, 0, len(ctx.Melds))
	for _, m := range ctx.Melds {
		melds = append(melds, gbmj.Meld{
			Type:  gbmj.MeldType(m.Type),
			Tiles: m.Tiles,
			Offer: m.Offer,
			Mid:   m.Mid,
		})
	}
	return gbmj.HandView{
		Concealed:      ctx.Concealed,
		Melds:          melds,
		Flowers:        ctx.Flowers,
		LastTile:       ctx.LastTile,
		Zimo:           ctx.Zimo,
		Juezhang:       ctx.Juezhang,
		Haidi:          ctx.Haidi,
		Gang:           ctx.Gang,
		PrevailingWind: ctx.PrevailingWind,
		SeatWind:       ctx.SeatWind,
	}
}

func (e *CGOEngine) JudgeHu(ctx HandContext) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return gbmj.JudgeHu(gbmj.ToLibString(toView(ctx)))
}

func (e *CGOEngine) CountFan(ctx HandContext) (FanResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	raw, err := gbmj.CountFan(gbmj.ToLibString(toView(ctx)))
	if err != nil {
		return FanResult{}, err
	}
	out := FanResult{TotalFan: raw.TotalFan}
	flower := 0
	for _, it := range raw.Items {
		if it.ID == FanHuapai {
			flower += it.Score
		}
		out.Items = append(out.Items, FanItem{ID: it.ID, Score: it.Score, Name: NameOf(it.ID)})
	}
	if flower == 0 {
		flower = len(ctx.Flowers)
	}
	out.FlowerFan = flower
	return out, nil
}

func (e *CGOEngine) CalcTing(ctx HandContext) ([]tile.Tile, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v := toView(ctx)
	v.LastTile = 0
	return gbmj.CalcTing(gbmj.ToLibString(v))
}
