package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// 压测入口。四种模式共用登录/WebSocket：
//
//	login  注册+登录后立刻断开
//	idle   鉴权后挂着心跳
//	play   N 间房 × 4 个笨机器人打完一局（打最后一张 / 一律过）
//	chat   挂机连接上刷世界聊天并互加好友
func main() {
	addr := flag.String("addr", "http://127.0.0.1:18080", "server")
	mode := flag.String("mode", "play", "login | idle | play | chat")
	n := flag.Int("n", 100, "users for login/idle/chat")
	rooms := flag.Int("rooms", 10, "rooms for play (each room 4 dumb bots)")
	sec := flag.Int("sec", 60, "seconds for idle/chat")
	qps := flag.Int("qps", 20, "chat messages per second (total)")
	prefix := flag.String("prefix", "st", "username prefix (3-16 letters/digits)")
	flag.Parse()

	*addr = strings.TrimRight(*addr, "/")
	var err error
	switch *mode {
	case "login":
		err = runLogin(*addr, *prefix, *n)
	case "idle":
		err = runIdle(*addr, *prefix, *n, *sec)
	case "play":
		err = runPlay(*addr, *prefix, *rooms)
	case "chat":
		err = runChat(*addr, *prefix, *n, *qps, *sec)
	default:
		fmt.Fprintf(os.Stderr, "unknown -mode %s (login|idle|play|chat)\n", *mode)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}
