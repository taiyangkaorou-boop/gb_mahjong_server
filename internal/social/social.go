package social

import (
	"errors"
	"sync"
	"time"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/memory"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/user"
)

var ErrRate = errors.New("rate limited")

type Service struct {
	DB    *sqlite.Store
	Users *user.Service
	On    *memory.Presence
	mu    sync.Mutex
	buck  map[int64]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func New(db *sqlite.Store, users *user.Service, on *memory.Presence) *Service {
	return &Service{DB: db, Users: users, On: on, buck: map[int64]*bucket{}}
}

func (s *Service) AllowChat(uid int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buck[uid]
	if b == nil {
		b = &bucket{tokens: 4, last: now}
		s.buck[uid] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * 2
	if b.tokens > 4 {
		b.tokens = 4
	}
	b.last = now
	if b.tokens < 1 {
		logx.Warnf("social chat rate limited uid=%d", uid)
		return false
	}
	b.tokens--
	return true
}

func (s *Service) Ask(from, to int64) error {
	logx.Tracef("social Ask from=%d to=%d", from, to)
	if from == to || to <= 0 {
		logx.Warnf("social Ask rejected from=%d to=%d", from, to)
		return errors.New("bad peer")
	}
	if s.DB.AreFriends(from, to) {
		logx.Tracef("social Ask already friends from=%d to=%d", from, to)
		return nil
	}
	if err := s.DB.AddFriendRequest(from, to); err != nil {
		return err
	}
	logx.Infof("social friend request from=%d to=%d", from, to)
	return nil
}

func (s *Service) Respond(uid, from int64, accept bool) error {
	logx.Tracef("social Respond uid=%d from=%d accept=%v", uid, from, accept)
	if accept {
		if err := s.DB.AddFriends(from, uid); err != nil {
			return err
		}
		logx.Infof("social friend accepted uid=%d peer=%d", uid, from)
		return nil
	}
	if err := s.DB.DeleteFriendRequest(from, uid); err != nil {
		return err
	}
	logx.Infof("social friend rejected uid=%d peer=%d", uid, from)
	return nil
}

type Item struct {
	UID    int64
	Name   string
	Online bool
}

func (s *Service) Snapshot(uid int64) (friends, pending []Item, err error) {
	ids, err := s.DB.FriendsOf(uid)
	if err != nil {
		return nil, nil, err
	}
	for _, id := range ids {
		friends = append(friends, Item{UID: id, Name: s.Users.Name(id), Online: s.On.Online(id)})
	}
	ps, err := s.DB.PendingTo(uid)
	if err != nil {
		return nil, nil, err
	}
	for _, id := range ps {
		pending = append(pending, Item{UID: id, Name: s.Users.Name(id), Online: s.On.Online(id)})
	}
	return friends, pending, nil
}

func (s *Service) FriendIDs(uid int64) []int64 {
	ids, _ := s.DB.FriendsOf(uid)
	return ids
}
