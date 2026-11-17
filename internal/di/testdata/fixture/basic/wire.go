//go:build wireinject

package basic

import "github.com/google/wire"

func InitializeServer(cfg *Config, name string) (*Server, func(), error) {
	panic(wire.Build(wire.NewSet(StoreSet), NewServer))
}
