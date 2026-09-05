package game

import (
	"math"
	"time"
)

// SetExtra 设置四人各自的整局储备时间。开局 Deal 前调用；0 表示没有储备，超时即代打/代过。
func (t *Table) SetExtra(extra time.Duration) {
	if extra < 0 {
		extra = 0
	}
	t.Extra = extra
	for i := 0; i < 4; i++ {
		t.ExtraLeft[i] = extra
	}
}

func (t *Table) armDeadline(now time.Time) {
	t.TurnEnd = now.Add(t.Timeout)
	t.Deadline = t.nextDeadline()
}

func (t *Table) nextDeadline() time.Time {
	waiting := t.WaitingSeats()
	if len(waiting) == 0 {
		return t.TurnEnd
	}
	end := t.TurnEnd.Add(t.ExtraLeft[waiting[0]])
	for _, s := range waiting[1:] {
		cand := t.TurnEnd.Add(t.ExtraLeft[s])
		if cand.Before(end) {
			end = cand
		}
	}
	return end
}

// consumeExtra 只扣「超过本回合免费时间」的那一段。10 秒内出手不扣储备。
func (t *Table) consumeExtra(seat int, now time.Time) {
	if seat < 0 || seat > 3 || t.TurnEnd.IsZero() || !now.After(t.TurnEnd) {
		return
	}
	used := now.Sub(t.TurnEnd)
	if used >= t.ExtraLeft[seat] {
		t.ExtraLeft[seat] = 0
		return
	}
	t.ExtraLeft[seat] -= used
}

func (t *Table) seatTimedOut(seat int, now time.Time) bool {
	return !now.Before(t.TurnEnd.Add(t.ExtraLeft[seat]))
}

func (t *Table) waiting(seat int) bool {
	for _, s := range t.WaitingSeats() {
		if s == seat {
			return true
		}
	}
	return false
}

func msOf(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	ms := d / time.Millisecond
	if ms > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(ms)
}

// TurnMS 每回合免费决策时长（毫秒）。
func (t *Table) TurnMS() uint32 {
	return msOf(t.Timeout)
}

// ExtraLeftMS 四个座位各自还剩多少储备（毫秒），固定长度 4。
func (t *Table) ExtraLeftMS() []uint32 {
	out := make([]uint32, 4)
	for i := 0; i < 4; i++ {
		out[i] = msOf(t.ExtraLeft[i])
	}
	return out
}

// WaitMS 该座位还剩多久会被服务器代打/代过。不是他操作时，返回当前最紧急那位的剩余时间。
func (t *Table) WaitMS(seat int, now time.Time) uint32 {
	if t.Finished != nil || t.Phase == PhaseOver || t.Phase == PhaseIdle || t.TurnEnd.IsZero() {
		return 0
	}
	end := t.Deadline
	if t.waiting(seat) {
		end = t.TurnEnd.Add(t.ExtraLeft[seat])
	}
	if !end.After(now) {
		return 0
	}
	return msOf(end.Sub(now))
}
