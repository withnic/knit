// Package store declares nested provider sets used from another package.
package store

import "github.com/google/wire"

type (
	Store    interface{ Get() string }
	DB       struct{}
	SQLStore struct{ DB *DB }
)

func (*SQLStore) Get() string { return "" }

func NewDB() (*DB, func(), error) { return &DB{}, func() {}, nil }

func NewSQLStore(db *DB) *SQLStore { return &SQLStore{DB: db} }

var (
	DBSet = wire.NewSet(NewDB)
	Set   = wire.NewSet(DBSet, NewSQLStore, wire.Bind(new(Store), new(*SQLStore)))
)
