package user

import "github.com/taiyangkaorou-boop/GB_mahjong_server/internal/persist/sqlite"

type Service struct{ DB *sqlite.Store }

func (s *Service) Name(id int64) string {
	if s == nil || s.DB == nil {
		return ""
	}
	u, err := s.DB.UserByID(id)
	if err != nil {
		return ""
	}
	return u.Name
}
