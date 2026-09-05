package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"
)

var (
	ErrBadUser = errors.New("invalid username or password")
	ErrDup     = errors.New("user exists")
	ErrAuth    = errors.New("auth failed")
	nameRe     = regexp.MustCompile(`^[a-zA-Z0-9_]{3,16}$`)
)

type Service struct {
	db  *sqlite.Store
	ttl time.Duration
}

func New(db *sqlite.Store, ttl time.Duration) *Service {
	return &Service{db: db, ttl: ttl}
}

func (s *Service) Register(user, pass string) (int64, error) {
	logx.Tracef("auth Register user=%s", user)
	if !nameRe.MatchString(user) || len(pass) < 4 || len(pass) > 64 {
		logx.Warnf("auth register rejected user=%s reason=invalid", user)
		return 0, ErrBadUser
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		logx.Errorf("auth register hash user=%s: %v", user, err)
		return 0, err
	}
	id, err := s.db.CreateUser(user, hash)
	if err != nil {
		logx.Warnf("auth register duplicate user=%s", user)
		return 0, ErrDup
	}
	logx.Infof("auth registered uid=%d user=%s", id, user)
	return id, nil
}

func (s *Service) Login(user, pass string) (uid int64, token string, err error) {
	logx.Tracef("auth Login user=%s", user)
	u, err := s.db.UserByName(user)
	if err != nil {
		logx.Warnf("auth login failed user=%s reason=unknown_user", user)
		return 0, "", ErrAuth
	}
	if bcrypt.CompareHashAndPassword(u.PassHash, []byte(pass)) != nil {
		logx.Warnf("auth login failed user=%s reason=bad_password", user)
		return 0, "", ErrAuth
	}
	_ = s.db.DeleteSessionsOf(u.ID)
	tok, err := randomToken()
	if err != nil {
		logx.Errorf("auth login token user=%s: %v", user, err)
		return 0, "", err
	}
	if err := s.db.PutSession(tok, u.ID, time.Now().Add(s.ttl)); err != nil {
		logx.Errorf("auth login session uid=%d: %v", u.ID, err)
		return 0, "", err
	}
	logx.Infof("auth login uid=%d user=%s", u.ID, user)
	return u.ID, tok, nil
}

func (s *Service) Resolve(token string) (int64, error) {
	logx.Tracef("auth Resolve")
	uid, exp, err := s.db.Session(token)
	if err != nil || time.Now().After(exp) {
		logx.Warnf("auth token rejected")
		return 0, ErrAuth
	}
	logx.Tracef("auth Resolve uid=%d", uid)
	return uid, nil
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
