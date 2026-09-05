package settle

import (
	"testing"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

type stubEng struct {
	hu bool
	fr rules.FanResult
}

func (s stubEng) JudgeHu(rules.HandContext) (bool, error) { return s.hu, nil }
func (s stubEng) CountFan(rules.HandContext) (rules.FanResult, error) {
	return s.fr, nil
}
func (s stubEng) CalcTing(rules.HandContext) ([]tile.Tile, error) { return nil, nil }

func TestFlowerNotCountAsStartFan(t *testing.T) {
	eng := stubEng{hu: true, fr: rules.FanResult{TotalFan: 8, FlowerFan: 8}}
	legal, wrong, _, err := Evaluate(eng, rules.HandContext{})
	if err != nil || legal || !wrong {
		t.Fatalf("flower-only must be wrong hu legal=%v wrong=%v err=%v", legal, wrong, err)
	}
}

func TestStartFanPassThenPayTotal(t *testing.T) {
	eng := stubEng{hu: true, fr: rules.FanResult{TotalFan: 10, FlowerFan: 2}}
	legal, wrong, fr, err := Evaluate(eng, rules.HandContext{})
	if err != nil || !legal || wrong {
		t.Fatalf("legal=%v wrong=%v err=%v", legal, wrong, err)
	}
	if fr.TotalFan-fr.FlowerFan < MinStartFan {
		t.Fatal("start fan")
	}
}

func TestNotHuShapeIsWrong(t *testing.T) {
	legal, wrong, _, err := Evaluate(stubEng{hu: false}, rules.HandContext{})
	if err != nil || legal || !wrong {
		t.Fatalf("legal=%v wrong=%v", legal, wrong)
	}
}
