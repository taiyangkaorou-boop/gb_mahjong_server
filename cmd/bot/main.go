package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/ai"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/game"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/netx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/rules"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/pkg/tile"
	"google.golang.org/protobuf/proto"
)

type bot struct {
	name, pass, token string
	uid               int64
	seat              int
	ws                *websocket.Conn
	roomID            string
	dealt             bool
	startSent         time.Time
	readyN            int
	pending           bool
	view              *ai.View
	eng               rules.Engine
	stats             ai.Stats
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:8080", "server")
	n := flag.Int("n", 4, "bots")
	flag.Parse()
	eng := rules.NewCGOEngine()
	var wg sync.WaitGroup
	roomCh := make(chan string, 8)
	errCh := make(chan error, *n)
	for i := 0; i < *n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := &bot{
				name: fmt.Sprintf("bot%d", i+1),
				pass: "botpass",
				seat: i,
				view: &ai.View{Seat: i},
				eng:  eng,
			}
			if err := b.run(*addr, i == 0, roomCh, *n); err != nil {
				log.Printf("%s: %v", b.name, err)
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	fails := 0
	for range errCh {
		fails++
	}
	if fails > 0 {
		log.Fatalf("%d bot(s) failed", fails)
	}
}

func (b *bot) run(addr string, leader bool, roomCh chan string, n int) error {
	_ = b.http(addr+"/v1/register", map[string]string{"user": b.name, "pass": b.pass})
	raw, err := post(addr+"/v1/login", map[string]string{"user": b.name, "pass": b.pass})
	if err != nil {
		return err
	}
	var lr struct {
		UID   int64  `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &lr); err != nil {
		return err
	}
	b.uid, b.token = lr.UID, lr.Token
	wsURL := "ws" + addr[len("http"):] + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	b.ws = ws
	defer ws.Close()

	if err := b.send(pb.Cmd_C2S_AUTH, &pb.C2SAuth{Token: b.token}); err != nil {
		return err
	}
	if err := b.readUntil(5*time.Second, func(env *pb.Envelope) bool {
		return env.Cmd == pb.Cmd_S2C_AUTH && env.Code == 0
	}); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	_ = b.send(pb.Cmd_C2S_ROOM_LEAVE, &pb.C2SRoomLeave{})

	if leader {
		if err := b.send(pb.Cmd_C2S_ROOM_CREATE, &pb.C2SRoomCreate{}); err != nil {
			return err
		}
		if err := b.readUntil(5*time.Second, func(env *pb.Envelope) bool {
			b.onEnv(env)
			return b.roomID != ""
		}); err != nil {
			return fmt.Errorf("create: %w", err)
		}
		log.Printf("%s room %s", b.name, b.roomID)
		for i := 1; i < n; i++ {
			roomCh <- b.roomID
		}
	} else {
		id := <-roomCh
		b.roomID = id
		if err := b.send(pb.Cmd_C2S_ROOM_JOIN, &pb.C2SRoomJoin{RoomId: id}); err != nil {
			return err
		}
		if err := b.readUntil(5*time.Second, func(env *pb.Envelope) bool {
			b.onEnv(env)
			return env.Cmd == pb.Cmd_S2C_ROOM_STATE && env.Code == 0
		}); err != nil {
			return fmt.Errorf("join: %w", err)
		}
	}

	if err := b.send(pb.Cmd_C2S_ROOM_SIT, &pb.C2SRoomSit{Seat: uint32(b.seat)}); err != nil {
		return err
	}
	if err := b.send(pb.Cmd_C2S_ROOM_READY, &pb.C2SRoomReady{Ready: true}); err != nil {
		return err
	}

	evs := 0
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Minute))
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		env, err := netx.Decode(msg)
		if err != nil {
			continue
		}
		if env.Cmd == pb.Cmd_S2C_GAME_EVENT {
			evs++
		}
		if env.Cmd == pb.Cmd_S2C_SETTLE {
			var s pb.S2CSettle
			_ = netx.UnmarshalBody(env, &s)
			log.Printf("%s settle ok kind=%d fan=%d scores=%v events=%d chi=%d peng=%d gang=%d hu=%d discard=%d pass=%d",
				b.name, s.HuKind, s.Fan, s.Scores, evs,
				b.stats.Chi, b.stats.Peng, b.stats.Gang, b.stats.Hu, b.stats.Discard, b.stats.Pass)
			return nil
		}
		if env.Code != 0 {
			log.Printf("%s err cmd=%v code=%d", b.name, env.Cmd, env.Code)
			if env.Cmd == pb.Cmd_S2C_ERROR || env.Cmd == pb.Cmd_S2C_GAME_EVENT {
				b.pending = false
			}
		}
		b.onEnv(env)
		if leader && !b.dealt && b.readyN >= 4 {
			if b.startSent.IsZero() || time.Since(b.startSent) > 400*time.Millisecond {
				b.startSent = time.Now()
				log.Printf("%s start ready=%d", b.name, b.readyN)
				_ = b.send(pb.Cmd_C2S_ROOM_START, &pb.C2SRoomStart{})
			}
		}
		b.maybeAct()
	}
}

func (b *bot) readUntil(d time.Duration, ok func(*pb.Envelope) bool) error {
	_ = b.ws.SetReadDeadline(time.Now().Add(d))
	defer b.ws.SetReadDeadline(time.Time{})
	for {
		_, msg, err := b.ws.ReadMessage()
		if err != nil {
			return err
		}
		env, err := netx.Decode(msg)
		if err != nil {
			continue
		}
		if ok(env) {
			return nil
		}
	}
}

func (b *bot) onEnv(env *pb.Envelope) {
	switch env.Cmd {
	case pb.Cmd_S2C_ROOM_STATE:
		var st pb.S2CRoomState
		_ = netx.UnmarshalBody(env, &st)
		if st.RoomId != "" {
			b.roomID = st.RoomId
		}
		nReady := 0
		for _, s := range st.Seats {
			if s != nil && s.Uid != 0 && s.Ready {
				nReady++
			}
		}
		b.readyN = nReady
	case pb.Cmd_S2C_DEAL:
		var d pb.S2CDeal
		_ = netx.UnmarshalBody(env, &d)
		b.seat = int(d.Seat)
		b.dealt = true
		b.pending = false
		b.view.Seat = int(d.Seat)
		ev := game.Event{
			IsDeal: true,
			Banker: int(d.Banker),
			Wind:   tile.Tile(uint8(d.Wind)),
			TurnID: d.TurnId,
		}
		ev.DealHands[b.view.Seat] = bytesToTiles(d.Hand)
		b.view.OnEvent(ev)
		log.Printf("%s deal seat=%d tiles=%d turn=%d", b.name, b.view.Seat, len(b.view.Hand), d.TurnId)
	case pb.Cmd_S2C_GAME_EVENT:
		var e pb.S2CGameEvent
		_ = netx.UnmarshalBody(env, &e)
		b.pending = false
		b.view.OnEvent(pbToGameEvent(&e))
	}
}

func (b *bot) maybeAct() {
	if b.pending {
		return
	}
	act := ai.Decide(b.view, b.eng)
	if act.Type == 0 {
		return
	}
	b.pending = true
	b.view.NoteSent(act)
	b.stats.Add(act.Type)
	_ = b.send(pb.Cmd_C2S_ACTION, &pb.C2SAction{
		TurnId: act.TurnID,
		Type:   gameToPBAct(act.Type),
		Tile:   uint32(act.Tile),
		ChiMid: uint32(act.ChiMid),
	})
}

func pbToGameEvent(e *pb.S2CGameEvent) game.Event {
	ev := game.Event{
		Type:        pbToGameAct(e.Type),
		Seat:        int(e.Seat),
		Tile:        tile.Tile(uint8(e.Tile)),
		Tiles:       bytesToTiles(e.Tiles),
		WallLeft:    int(e.WallLeft),
		TurnID:      e.TurnId,
		PrivateSeat: -1,
	}
	if e.Type == pb.ActionType_ACT_DRAW {
		ev.PrivateSeat = int(e.Seat)
	}
	return ev
}

func bytesToTiles(b []byte) []tile.Tile {
	out := make([]tile.Tile, len(b))
	for i, x := range b {
		out[i] = tile.Tile(x)
	}
	return out
}

func pbToGameAct(t pb.ActionType) game.ActType {
	switch t {
	case pb.ActionType_ACT_PASS:
		return game.ActPass
	case pb.ActionType_ACT_DISCARD:
		return game.ActDiscard
	case pb.ActionType_ACT_CHI:
		return game.ActChi
	case pb.ActionType_ACT_PENG:
		return game.ActPeng
	case pb.ActionType_ACT_GANG_MING:
		return game.ActGangMing
	case pb.ActionType_ACT_GANG_AN:
		return game.ActAnGang
	case pb.ActionType_ACT_GANG_JIA:
		return game.ActJiaGang
	case pb.ActionType_ACT_HU:
		return game.ActHu
	case pb.ActionType_ACT_DRAW:
		return game.ActDraw
	case pb.ActionType_ACT_BUHUA:
		return game.ActBuhua
	default:
		return 0
	}
}

func gameToPBAct(t game.ActType) pb.ActionType {
	switch t {
	case game.ActPass:
		return pb.ActionType_ACT_PASS
	case game.ActDiscard:
		return pb.ActionType_ACT_DISCARD
	case game.ActChi:
		return pb.ActionType_ACT_CHI
	case game.ActPeng:
		return pb.ActionType_ACT_PENG
	case game.ActGangMing:
		return pb.ActionType_ACT_GANG_MING
	case game.ActAnGang:
		return pb.ActionType_ACT_GANG_AN
	case game.ActJiaGang:
		return pb.ActionType_ACT_GANG_JIA
	case game.ActHu:
		return pb.ActionType_ACT_HU
	default:
		return pb.ActionType_ACT_UNSPECIFIED
	}
}

func (b *bot) send(cmd pb.Cmd, body proto.Message) error {
	raw, err := netx.Encode(1, cmd, 0, body)
	if err != nil {
		return err
	}
	return b.ws.WriteMessage(websocket.BinaryMessage, raw)
}

func post(url string, v interface{}) ([]byte, error) {
	buf, _ := json.Marshal(v)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s", resp.Status, out.String())
	}
	return out.Bytes(), nil
}

func (b *bot) http(url string, v interface{}) error {
	_, err := post(url, v)
	return err
}
