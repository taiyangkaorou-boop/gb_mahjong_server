package gbmj

/*
#cgo CXXFLAGS: -std=c++11 -O3 -I${SRCDIR}/../../third_party/GB-Mahjong/mahjong -I${SRCDIR}/../../third_party/GB-Mahjong/console
#cgo LDFLAGS: -lstdc++
#include "cgbmj.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
)

// FanItem 是一条番种。
type FanItem struct {
	ID    int
	Score int
}

// FanResult 是 CountFan 的结果。
type FanResult struct {
	TotalFan int
	Items    []FanItem
}

func cstr(s string) (*C.char, func()) {
	p := C.CString(s)
	return p, func() { C.free(unsafe.Pointer(p)) }
}

// JudgeHu 判断手牌字符串是否成和型。
func JudgeHu(handStr string) (bool, error) {
	cs, free := cstr(handStr)
	defer free()
	var hu C.int
	rc := C.gbmj_judge_hu(cs, &hu)
	if rc != C.GBMJ_OK {
		err := fmt.Errorf("gbmj judge_hu rc=%d str=%q", int(rc), handStr)
		logx.Errorf("%v", err)
		return false, err
	}
	return hu != 0, nil
}

// CountFan 计算番种。
func CountFan(handStr string) (FanResult, error) {
	cs, free := cstr(handStr)
	defer free()
	var out C.gbmj_fan_result
	rc := C.gbmj_count_fan(cs, &out)
	if rc != C.GBMJ_OK {
		err := fmt.Errorf("gbmj count_fan rc=%d str=%q", int(rc), handStr)
		logx.Errorf("%v", err)
		return FanResult{}, err
	}
	n := int(out.n_fan)
	if n < 0 {
		n = 0
	}
	if n > 32 {
		n = 32
	}
	items := make([]FanItem, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, FanItem{ID: int(out.fan_ids[i]), Score: int(out.fan_scores[i])})
	}
	return FanResult{TotalFan: int(out.tot_fan), Items: items}, nil
}

// CalcTing 返回听牌（本系统编码）。
func CalcTing(handStr string) ([]tile.Tile, error) {
	cs, free := cstr(handStr)
	defer free()
	buf := make([]byte, 48)
	var n C.int
	rc := C.gbmj_calc_ting(cs, (*C.uchar)(unsafe.Pointer(&buf[0])), C.int(len(buf)), &n)
	if rc != C.GBMJ_OK {
		err := fmt.Errorf("gbmj calc_ting rc=%d str=%q", int(rc), handStr)
		logx.Errorf("%v", err)
		return nil, err
	}
	out := make([]tile.Tile, 0, int(n))
	lim := int(n)
	if lim > len(buf) {
		lim = len(buf)
	}
	for i := 0; i < lim; i++ {
		t := tile.FromLibID(int(buf[i]))
		if t != 0 && !t.IsHua() {
			out = append(out, t)
		}
	}
	return out, nil
}
