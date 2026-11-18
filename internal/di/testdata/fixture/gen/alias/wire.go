//go:build wireinject

package alias

import "github.com/google/wire"

func InitializeRoot() *Root {
	panic(wire.Build(NewConfig, NewConfigs, NewBox, NewRoot))
}
