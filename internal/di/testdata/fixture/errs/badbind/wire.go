//go:build wireinject

package badbind

import "github.com/google/wire"

func InitializeService() *Service {
	panic(wire.Build(NewService, NewSQLStore, wire.Bind(new(Store), new(SQLStore))))
}

func InitializeClosing() *Service {
	panic(wire.Build(NewClosing, NewOther, wire.Bind(new(Closer), new(Other))))
}
