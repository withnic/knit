// Package app composes provider sets from several packages.
package app

import (
	"example.com/fixture/gen/sets/store"
	"github.com/google/wire"
)

type (
	Logger struct{}
	Server struct{}
)

func NewLogger() *Logger { return &Logger{} }

func NewServer(store.Store, *Logger) *Server { return &Server{} }

var LogSet = wire.NewSet(NewLogger)
