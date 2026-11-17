//go:build wireinject

package variadic

import "github.com/google/wire"

func InitializeClient() *Client {
	panic(wire.Build(NewClient))
}

func InitializeClientWithOptions(opts ...Option) *Client {
	panic(wire.Build(wire.NewSet()))
}
