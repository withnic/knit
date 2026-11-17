//go:build wireinject

package di

import (
	"example.com/fixture/samename/app"
	"example.com/fixture/samename/pkga"
	"example.com/fixture/samename/pkgb"
	"github.com/google/wire"
)

func InitializeApp() *app.App {
	panic(wire.Build(pkga.Set, pkgb.NewConfig, app.NewApp))
}
