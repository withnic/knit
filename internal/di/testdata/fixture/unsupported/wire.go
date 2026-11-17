//go:build wireinject

package unsupported

import "github.com/google/wire"

func InitializeService() *Service {
	panic(wire.Build(NewService, wire.Value(Config{Name: "x"})))
}
