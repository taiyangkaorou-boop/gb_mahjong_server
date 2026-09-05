package memory

import "sync"

// Presence 在线连接登记。
type Presence struct {
	mu    sync.RWMutex
	online map[int64]struct{}
}

func NewPresence() *Presence {
	return &Presence{online: map[int64]struct{}{}}
}

func (p *Presence) Set(uid int64, on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if on {
		p.online[uid] = struct{}{}
	} else {
		delete(p.online, uid)
	}
}

func (p *Presence) Online(uid int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.online[uid]
	return ok
}

func (p *Presence) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.online)
}
