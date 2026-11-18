//go:build wireinject

package duplicate

import "github.com/google/wire"

func InitializeService() *Service {
	panic(wire.Build(NewService, NewRepo, NewStore, NewSQLStore, wire.Bind(new(Store), new(*SQLStore))))
}
