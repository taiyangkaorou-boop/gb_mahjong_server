package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/room"
)

//go:embed gm.html
var gmHTML []byte

// #region agent log
func agentLog(hypothesisId, location, message string, data map[string]interface{}) {
	f, err := os.OpenFile("/home/ros/work/GB_mahjong_server/.cursor/debug-da195a.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(map[string]interface{}{
		"sessionId":    "da195a",
		"runId":        "pre-fix",
		"hypothesisId": hypothesisId,
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	})
	_, _ = f.Write(append(b, '\n'))
}

// #endregion

func (a *App) registerGM(mux *http.ServeMux) {
	tokenLen := len(strings.TrimSpace(a.cfg.GMToken))
	if tokenLen == 0 {
		// #region agent log
		agentLog("B", "gm.go:registerGM", "gm routes skipped, token empty", map[string]interface{}{"enabled": false, "tokenLen": 0})
		// #endregion
		logx.Infof("gm disabled")
		return
	}
	mux.HandleFunc("/gm", a.handleGMPage)
	mux.HandleFunc("/gm/api/rooms", a.handleGMRooms)
	mux.HandleFunc("/gm/api/addbot", a.handleGMAddBot)
	// #region agent log
	agentLog("B", "gm.go:registerGM", "gm routes registered", map[string]interface{}{"enabled": true, "tokenLen": tokenLen, "htmlBytes": len(gmHTML)})
	// #endregion
	logx.Infof("gm page enabled path=/gm")
}

func (a *App) gmAuthorized(r *http.Request) bool {
	want := strings.TrimSpace(a.cfg.GMToken)
	if want == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-GM-Token"))
	if got == "" {
		got = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	return got == want
}

func (a *App) handleGMPage(w http.ResponseWriter, r *http.Request) {
	// #region agent log
	agentLog("C", "gm.go:handleGMPage", "gm page request", map[string]interface{}{"path": r.URL.Path, "method": r.Method, "htmlBytes": len(gmHTML)})
	// #endregion
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(gmHTML)
}

func (a *App) handleGMRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	if !a.gmAuthorized(r) {
		logx.Warnf("gm unauthorized path=%s", r.URL.Path)
		writeJSON(w, 401, map[string]string{"error": "auth failed"})
		return
	}
	if a.rooms == nil {
		writeJSON(w, 200, map[string]interface{}{"rooms": []room.RoomSnap{}})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"rooms": a.rooms.List()})
}

func (a *App) handleGMAddBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	if !a.gmAuthorized(r) {
		logx.Warnf("gm unauthorized path=%s", r.URL.Path)
		writeJSON(w, 401, map[string]string{"error": "auth failed"})
		return
	}
	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	id := strings.TrimSpace(req.RoomID)
	if len(id) != 6 {
		writeJSON(w, 400, map[string]string{"error": "bad room_id"})
		return
	}
	if a.rooms == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	snap, err := a.rooms.AddBots(id)
	if err != nil {
		logx.Warnf("gm addbot room=%s: %v", id, err)
		writeJSON(w, gmHTTPStatus(err), map[string]string{"error": gmErrText(err)})
		return
	}
	logx.Infof("gm addbot room=%s added=%d", id, snap.Added)
	writeJSON(w, 200, snap)
}

func gmHTTPStatus(err error) int {
	switch err {
	case room.ErrNotFound:
		return 404
	case room.ErrState, room.ErrFull:
		return 409
	default:
		return 400
	}
}

func gmErrText(err error) string {
	switch err {
	case room.ErrNotFound:
		return "not found"
	case room.ErrState:
		return "bad state"
	case room.ErrFull:
		return "room full"
	default:
		return err.Error()
	}
}
