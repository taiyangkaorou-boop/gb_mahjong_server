package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT UNIQUE NOT NULL,
  pass_hash BLOB NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  uid INTEGER NOT NULL,
  expire_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS friend_requests (
  from_uid INTEGER NOT NULL,
  to_uid INTEGER NOT NULL,
  PRIMARY KEY (from_uid, to_uid)
);
CREATE TABLE IF NOT EXISTS friends (
  uid_a INTEGER NOT NULL,
  uid_b INTEGER NOT NULL,
  PRIMARY KEY (uid_a, uid_b)
);
CREATE INDEX IF NOT EXISTS idx_sessions_uid ON sessions(uid);
`)
	return err
}

type User struct {
	ID       int64
	Name     string
	PassHash []byte
}

func (s *Store) CreateUser(name string, hash []byte) (int64, error) {
	r, err := s.db.Exec(`INSERT INTO users(name, pass_hash, created_at) VALUES(?,?,?)`, name, hash, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (s *Store) UserByName(name string) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, name, pass_hash FROM users WHERE name=?`, name).Scan(&u.ID, &u.Name, &u.PassHash)
	return u, err
}

func (s *Store) UserByID(id int64) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, name, pass_hash FROM users WHERE id=?`, id).Scan(&u.ID, &u.Name, &u.PassHash)
	return u, err
}

func (s *Store) PutSession(token string, uid int64, exp time.Time) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sessions(token, uid, expire_at) VALUES(?,?,?)`, token, uid, exp.Unix())
	return err
}

func (s *Store) Session(token string) (uid int64, exp time.Time, err error) {
	var unix int64
	err = s.db.QueryRow(`SELECT uid, expire_at FROM sessions WHERE token=?`, token).Scan(&uid, &unix)
	exp = time.Unix(unix, 0)
	return
}

func (s *Store) DeleteSessionsOf(uid int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE uid=?`, uid)
	return err
}

func pair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func (s *Store) AddFriendRequest(from, to int64) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO friend_requests(from_uid, to_uid) VALUES(?,?)`, from, to)
	return err
}

func (s *Store) DeleteFriendRequest(from, to int64) error {
	_, err := s.db.Exec(`DELETE FROM friend_requests WHERE from_uid=? AND to_uid=?`, from, to)
	return err
}

func (s *Store) AddFriends(a, b int64) error {
	x, y := pair(a, b)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO friends(uid_a, uid_b) VALUES(?,?)`, x, y); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM friend_requests WHERE (from_uid=? AND to_uid=?) OR (from_uid=? AND to_uid=?)`, a, b, b, a); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) AreFriends(a, b int64) bool {
	x, y := pair(a, b)
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM friends WHERE uid_a=? AND uid_b=?`, x, y).Scan(&n)
	return n > 0
}

func (s *Store) FriendsOf(uid int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT uid_a, uid_b FROM friends WHERE uid_a=? OR uid_b=?`, uid, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var a, b int64
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		if a == uid {
			out = append(out, b)
		} else {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

func (s *Store) PendingTo(uid int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT from_uid FROM friend_requests WHERE to_uid=?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
