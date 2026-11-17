// Package app is a knit-mode fixture: injectors are declared with the
// github.com/withnic/knit marker package.
package app

import "context"

// Log records resource lifecycle events in order.
var Log []string

type Store interface{ Get() string }

type SQLStore struct{ dsn string }

func (s *SQLStore) Get() string { return s.dsn }

type DSN string

func NewDSN(ctx context.Context) DSN {
	if v, ok := ctx.Value(DSN("key")).(string); ok {
		return DSN(v)
	}
	return "default"
}

func NewSQLStore(dsn DSN) (*SQLStore, func(), error) {
	Log = append(Log, "open")
	return &SQLStore{dsn: string(dsn)}, func() { Log = append(Log, "close") }, nil
}

type Server struct{ Store Store }

func NewServer(s Store) *Server { return &Server{Store: s} }
