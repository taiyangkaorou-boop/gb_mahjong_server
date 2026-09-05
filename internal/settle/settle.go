package settle

import (
	"fmt"
	"strings"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
)

const (
	KindHuang = 0
	KindZimo  = 1
	KindRon   = 2
	KindQiang = 3
	KindWrong = 4
)

const (
	MinStartFan = 8
	BasePay     = 8
)

// KindName 把结算种类写成短英文，给日志用。
func KindName(k int) string {
	switch k {
	case KindHuang:
		return "huang"
	case KindZimo:
		return "zimo"
	case KindRon:
		return "ron"
	case KindQiang:
		return "qiang"
	case KindWrong:
		return "wrong"
	default:
		return "unknown"
	}
}

// FormatFans 把番种列表写成日志用的一串，例如 七对(19):24,自摸(80):1。没有番种时为 "-"。
func FormatFans(items []rules.FanItem) string {
	if len(items) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		name := it.Name
		if name == "" {
			name = rules.NameOf(it.ID)
		}
		parts = append(parts, fmt.Sprintf("%s(%d):%d", name, it.ID, it.Score))
	}
	return strings.Join(parts, ",")
}

// Result 一局结算。
type Result struct {
	Kind      int
	Winner    int
	Discarder int
	Fan       int
	StartFan  int
	Items     []rules.FanItem
	Scores    [4]int32
}

// Evaluate 检查报和是否合法。legal=成牌且起和番够；wrong=报了但不能和。
func Evaluate(eng rules.Engine, ctx rules.HandContext) (legal, wrong bool, fr rules.FanResult, err error) {
	if eng == nil {
		logx.Tracef("settle Evaluate no engine")
		return false, true, fr, nil
	}
	ok, err := eng.JudgeHu(ctx)
	if err != nil {
		return false, false, fr, err
	}
	if !ok {
		logx.Tracef("settle Evaluate not hu")
		return false, true, fr, nil
	}
	fr, err = eng.CountFan(ctx)
	if err != nil {
		return false, false, fr, err
	}
	start := fr.TotalFan - fr.FlowerFan
	if start < MinStartFan {
		logx.Tracef("settle Evaluate below min start=%d fan=%d", start, fr.TotalFan)
		return false, true, fr, nil
	}
	logx.Tracef("settle Evaluate legal fan=%d start=%d", fr.TotalFan, start)
	return true, false, fr, nil
}

// PayHu 自摸三家付 8+fan；点炮/抢杠点炮者付 8+fan，另两家付 8。
func PayHu(zimo bool, fan, winner, discarder int) [4]int32 {
	var s [4]int32
	if winner < 0 || winner > 3 {
		return s
	}
	if zimo {
		pay := int32(BasePay + fan)
		for i := 0; i < 4; i++ {
			if i == winner {
				continue
			}
			s[i] -= pay
			s[winner] += pay
		}
		return s
	}
	for i := 0; i < 4; i++ {
		if i == winner {
			continue
		}
		if i == discarder {
			pay := int32(BasePay + fan)
			s[i] -= pay
			s[winner] += pay
		} else {
			s[i] -= BasePay
			s[winner] += BasePay
		}
	}
	return s
}

// PayWrong 错和者 -30，其余各 +10。
func PayWrong(seat int) [4]int32 {
	var s [4]int32
	if seat < 0 || seat > 3 {
		return s
	}
	s[seat] = -30
	for i := 0; i < 4; i++ {
		if i != seat {
			s[i] = 10
		}
	}
	return s
}

func Huang() Result {
	return Result{Kind: KindHuang, Winner: -1, Discarder: -1}
}
