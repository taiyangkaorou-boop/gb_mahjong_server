package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

func runIdle(addr, prefix string, n, sec int) error {
	if n <= 0 || sec <= 0 {
		return fmt.Errorf("-n and -sec must be > 0")
	}
	var failLogin int32
	var drop int32
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
				log.Printf("idle login %s: %v", uname(prefix, i), err)
				return
			}
			c, err := dialAuth(addr, tok)
			if err != nil {
				atomic.AddInt32(&failLogin, 1)
				log.Printf("idle auth %s: %v", uname(prefix, i), err)
				return
			}
			defer c.close()
			deadline := time.Now().Add(time.Duration(sec) * time.Second)
			_ = c.ws.SetReadDeadline(deadline)
			for {
				if _, err := c.read(); err != nil {
					if time.Now().After(deadline.Add(-200 * time.Millisecond)) {
						return
					}
					atomic.AddInt32(&drop, 1)
					log.Printf("idle drop %s: %v", uname(prefix, i), err)
					return
				}
			}
		}()
	}
	wg.Wait()
	fl := int(atomic.LoadInt32(&failLogin))
	dr := int(atomic.LoadInt32(&drop))
	ok := n - fl - dr
	log.Printf("stress idle n=%d connected=%d drop=%d login_fail=%d wall=%s",
		n, ok, dr, fl, fmtDur(time.Since(t0)))
	if fl > 0 || dr > 0 {
		return fmt.Errorf("idle fail login=%d drop=%d", fl, dr)
	}
	return nil
}
