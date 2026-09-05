package netx

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
)

type Conn struct {
	UID  int64
	Name string
	ws   *websocket.Conn
	send chan []byte
	dead chan struct{}
}

func NewConn(ws *websocket.Conn) *Conn {
	return &Conn{ws: ws, send: make(chan []byte, 64), dead: make(chan struct{})}
}

func (c *Conn) Send(b []byte) {
	select {
	case c.send <- b:
	default:
		logx.Warnf("netx send queue full uid=%d", c.UID)
	}
}

func (c *Conn) Close() {
	select {
	case <-c.dead:
	default:
		close(c.dead)
	}
	_ = c.ws.Close()
}

func (c *Conn) WriteLoop() {
	defer logx.Recover("netx.WriteLoop")
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.dead:
			return
		case b := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.ws.WriteMessage(websocket.BinaryMessage, b); err != nil {
				logx.Warnf("netx write uid=%d: %v", c.UID, err)
				return
			}
		case <-t.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				logx.Warnf("netx ping uid=%d: %v", c.UID, err)
				return
			}
		}
	}
}

type Registry struct {
	mu    sync.RWMutex
	conns map[int64]*Conn
}

func NewRegistry() *Registry {
	return &Registry{conns: map[int64]*Conn{}}
}

func (r *Registry) Bind(uid int64, c *Conn) *Conn {
	r.mu.Lock()
	old := r.conns[uid]
	r.conns[uid] = c
	r.mu.Unlock()
	return old
}

func (r *Registry) Unbind(uid int64, c *Conn) {
	r.mu.Lock()
	if r.conns[uid] == c {
		delete(r.conns, uid)
	}
	r.mu.Unlock()
}

func (r *Registry) Get(uid int64) *Conn {
	r.mu.RLock()
	c := r.conns[uid]
	r.mu.RUnlock()
	return c
}

func (r *Registry) Count() int {
	r.mu.RLock()
	n := len(r.conns)
	r.mu.RUnlock()
	return n
}

func (r *Registry) ForEach(fn func(*Conn)) {
	r.mu.RLock()
	list := make([]*Conn, 0, len(r.conns))
	for _, c := range r.conns {
		list = append(list, c)
	}
	r.mu.RUnlock()
	for _, c := range list {
		fn(c)
	}
}

func (r *Registry) Push(uid int64, raw []byte) {
	if c := r.Get(uid); c != nil {
		c.Send(raw)
	}
}
