package basic

import "github.com/google/wire"

type Config struct{ DSN string }

type DB struct{ cfg *Config }

type Store interface{ Get(id int) string }

type sqlStore struct{ db *DB }

func (s *sqlStore) Get(id int) string { return "" }

type Server struct {
	Store Store
	Name  string
}

func NewDB(cfg *Config) (*DB, func(), error) { return &DB{cfg: cfg}, func() {}, nil }

func NewSQLStore(db *DB) *sqlStore { return &sqlStore{db: db} }

func NewServer(s Store, name string) (*Server, error) { return &Server{Store: s, Name: name}, nil }

var StoreSet = wire.NewSet(NewDB, NewSQLStore, wire.Bind(new(Store), new(*sqlStore)))
