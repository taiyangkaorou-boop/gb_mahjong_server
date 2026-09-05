package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

func runLogin(addr, prefix string, n int) error {
	if n <= 0 {
		return fmt.Errorf("-n must be > 0")
	}
	var fail int
	var mu sync.Mutex
	var okDur []time.Duration
	var wg sync.WaitGroup
	wg.Add(n)
	t0 := time.Now()
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			start := time.Now()
			_, _, err := registerLogin(addr, uname(prefix, i))
			d := time.Since(start)
			mu.Lock()
			if err != nil {
				fail++
				mu.Unlock()
				log.Printf("login %s: %v", uname(prefix, i), err)
				return
			}
			okDur = append(okDur, d)
			mu.Unlock()
		}()
	}
	wg.Wait()
	ok := n - fail
	log.Printf("stress login n=%d ok=%d fail=%d wall=%s p50=%s p95=%s max=%s",
		n, ok, fail, fmtDur(time.Since(t0)), fmtDur(pct(okDur, 0.50)), fmtDur(pct(okDur, 0.95)), fmtDur(pct(okDur, 1)))
	if fail > 0 {
		return fmt.Errorf("%d login(s) failed", fail)
	}
	return nil
}
