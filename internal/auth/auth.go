package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

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
	if !nameRe.MatchString(user) || len(pass) < 4 || len(pass) > 64 {
		return 0, ErrBadUser
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	id, err := s.db.CreateUser(user, hash)
	if err != nil {
		return 0, ErrDup
	}
	return id, nil
}

func (s *Service) Login(user, pass string) (uid int64, token string, err error) {
	u, err := s.db.UserByName(user)
	if err != nil {
		return 0, "", ErrAuth
	}
	if bcrypt.CompareHashAndPassword(u.PassHash, []byte(pass)) != nil {
		return 0, "", ErrAuth
	}
	_ = s.db.DeleteSessionsOf(u.ID)
	tok, err := randomToken()
	if err != nil {
		return 0, "", err
	}
	if err := s.db.PutSession(tok, u.ID, time.Now().Add(s.ttl)); err != nil {
		return 0, "", err
	}
	return u.ID, tok, nil
}

func (s *Service) Resolve(token string) (int64, error) {
	uid, exp, err := s.db.Session(token)
	if err != nil || time.Now().After(exp) {
		return 0, ErrAuth
	}
	return uid, nil
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
