//go:build wireinject

package app

import (
	"example.com/fixture/gen/sets/store"
	"github.com/google/wire"
)

func InitializeServer() (*Server, func(), error) {
	panic(wire.Build(wire.NewSet(store.Set, LogSet), NewServer))
}
