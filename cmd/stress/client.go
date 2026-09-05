package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"google.golang.org/protobuf/proto"
)

// 压测账号密码固定；重跑时注册重名会 409，随后直接登录。
const pass = "stpass"

var httpClient = &http.Client{
	Timeout: 45 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
	},
}

type client struct {
	name string
	uid  int64
	ws   *websocket.Conn
	mu   sync.Mutex
}

// uname 生成 3–16 位字母数字用户名，例如 st00001。
func uname(prefix string, i int) string {
	p := prefix
	if p == "" {
		p = "st"
	}
	s := fmt.Sprintf("%s%05d", p, i)
	if len(s) > 16 {
		s = s[:16]
	}
	for len(s) < 3 {
		s += "x"
	}
	return s
}

func wsURL(addr string) string {
	if strings.HasPrefix(addr, "https") {
		return "wss" + addr[len("https"):] + "/ws"
	}
	if strings.HasPrefix(addr, "http") {
		return "ws" + addr[len("http"):] + "/ws"
	}
	return "ws://" + addr + "/ws"
}

func postJSON(url string, v interface{}) (int, []byte, error) {
	buf, _ := json.Marshal(v)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	return resp.StatusCode, out.Bytes(), nil
}

func registerLogin(addr, name string) (uid int64, token string, err error) {
	_, _, _ = postJSON(addr+"/v1/register", map[string]string{"user": name, "pass": pass})
	code, raw, err := postJSON(addr+"/v1/login", map[string]string{"user": name, "pass": pass})
	if err != nil {
		return 0, "", err
	}
	if code >= 300 {
		return 0, "", fmt.Errorf("login %s: %d %s", name, code, raw)
	}
	var lr struct {
		UID   int64  `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &lr); err != nil {
		return 0, "", err
	}
	if lr.Token == "" {
		return 0, "", fmt.Errorf("login %s: empty token", name)
	}
	return lr.UID, lr.Token, nil
}

func dialAuth(addr, token string) (*client, error) {
	ws, _, err := websocket.DefaultDialer.Dial(wsURL(addr), nil)
	if err != nil {
		return nil, err
	}
	c := &client{ws: ws}
	if err := c.send(pb.Cmd_C2S_AUTH, &pb.C2SAuth{Token: token}); err != nil {
		ws.Close()
		return nil, err
	}
	_ = ws.SetReadDeadline(time.Now().Add(8 * time.Second))
	for {
		env, err := c.read()
		if err != nil {
			_ = ws.Close()
			return nil, fmt.Errorf("auth: %w", err)
		}
		if env.Cmd == pb.Cmd_S2C_AUTH && env.Code == 0 {
			var a pb.S2CAuth
			_ = netx.UnmarshalBody(env, &a)
			c.uid = a.Uid
			c.name = a.Name
			_ = ws.SetReadDeadline(time.Time{})
			return c, nil
		}
		if env.Cmd == pb.Cmd_S2C_AUTH && env.Code != 0 {
			ws.Close()
			return nil, fmt.Errorf("auth code=%d", env.Code)
		}
	}
}

func (c *client) close() {
	if c == nil || c.ws == nil {
		return
	}
	_ = c.ws.Close()
}

func (c *client) send(cmd pb.Cmd, body proto.Message) error {
	raw, err := netx.Encode(1, cmd, 0, body)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.ws.WriteMessage(websocket.BinaryMessage, raw)
}

func (c *client) read() (*pb.Envelope, error) {
	_, msg, err := c.ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	return netx.Decode(msg)
}

func pct(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	i := int(p * float64(len(s)-1))
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		i = len(s) - 1
	}
	return s[i]
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	return d.Truncate(time.Millisecond).String()
}
