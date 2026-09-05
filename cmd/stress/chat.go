package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
)

func runChat(addr, prefix string, n, qps, sec int) error {
	if n <= 0 || sec <= 0 {
		return fmt.Errorf("-n and -sec must be > 0")
	}
	if qps < 0 {
		return fmt.Errorf("-qps must be >= 0")
	}
	type slot struct {
		c   *client
		uid int64
	}
	slots := make([]*slot, n)
	var failLogin int32
	var drop int32
	var chatSent, chatLimited, chatRecv int32
	var friendAsk, friendAcc int32

	var ready sync.WaitGroup
	ready.Add(n)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	t0 := time.Now()

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, tok, err := registerLogin(addr, uname(prefix, i))
			if err != nil {
				atomic.AddInt32(&failLogin, 1)
				ready.Done()
				log.Printf("chat login %s: %v", uname(prefix, i), err)
				return
			}
			c, err := dialAuth(addr, tok)
			if err != nil {
				atomic.AddInt32(&failLogin, 1)
				ready.Done()
				log.Printf("chat auth %s: %v", uname(prefix, i), err)
				return
			}
			slots[i] = &slot{c: c, uid: c.uid}
			ready.Done()
			defer c.close()
			for {
				env, err := c.read()
				if err != nil {
					select {
					case <-stop:
						return
					default:
					}
					atomic.AddInt32(&drop, 1)
					log.Printf("chat drop %s: %v", uname(prefix, i), err)
					return
				}
				switch env.Cmd {
				case pb.Cmd_S2C_CHAT:
					if env.Code == int32(pb.Code_CODE_RATE_LIMITED) {
						atomic.AddInt32(&chatLimited, 1)
					} else if env.Code == 0 {
						atomic.AddInt32(&chatRecv, 1)
					}
				case pb.Cmd_S2C_FRIEND_SYNC:
					var m pb.S2CFriendSync
					_ = netx.UnmarshalBody(env, &m)
					for _, p := range m.Pending {
						if p == nil || p.Uid == 0 {
							continue
						}
						if err := c.send(pb.Cmd_C2S_FRIEND_RESP, &pb.C2SFriendResp{Peer: p.Uid, Accept: true}); err == nil {
							atomic.AddInt32(&friendAcc, 1)
						}
					}
				}
			}
		}()
	}
	ready.Wait()
	end := time.Now().Add(time.Duration(sec) * time.Second)

	// 互相申请好友：每个人向下一家发一条。
	for i := 0; i < n; i++ {
		a := slots[i]
		b := slots[(i+1)%n]
		if a == nil || b == nil || a.uid == 0 || b.uid == 0 || a.uid == b.uid {
			continue
		}
		if err := a.c.send(pb.Cmd_C2S_FRIEND_ASK, &pb.C2SFriendAsk{Peer: b.uid}); err == nil {
			atomic.AddInt32(&friendAsk, 1)
		}
	}

	if qps > 0 {
		interval := time.Second / time.Duration(qps)
		if interval < time.Millisecond {
			interval = time.Millisecond
		}
		tick := time.NewTicker(interval)
		go func() {
			defer tick.Stop()
			idx := 0
			for {
				select {
				case <-stop:
					return
				case <-tick.C:
					if time.Now().After(end) {
						return
					}
					for k := 0; k < n; k++ {
						idx = (idx + 1) % n
						s := slots[idx]
						if s == nil || s.c == nil {
							continue
						}
						if err := s.c.send(pb.Cmd_C2S_CHAT, &pb.C2SChat{Text: []byte("hi")}); err == nil {
							atomic.AddInt32(&chatSent, 1)
						}
						break
					}
				}
			}
		}()
	}

	time.Sleep(time.Until(end))
	close(stop)
	for _, s := range slots {
		if s != nil && s.c != nil {
			s.c.close()
		}
	}
	wg.Wait()

	fl := int(atomic.LoadInt32(&failLogin))
	dr := int(atomic.LoadInt32(&drop))
	log.Printf("stress chat n=%d qps=%d sec=%d login_fail=%d drop=%d sent=%d recv=%d limited=%d friend_ask=%d friend_acc=%d wall=%s",
		n, qps, sec, fl, dr,
		atomic.LoadInt32(&chatSent), atomic.LoadInt32(&chatRecv), atomic.LoadInt32(&chatLimited),
		atomic.LoadInt32(&friendAsk), atomic.LoadInt32(&friendAcc),
		fmtDur(time.Since(t0)))
	if fl > 0 || dr > 0 {
		return fmt.Errorf("chat fail login=%d drop=%d", fl, dr)
	}
	return nil
}
