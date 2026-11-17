//go:build wireinject

package di

import (
	"example.com/fixture/missing/repo"
	"example.com/fixture/missing/service"
	"github.com/google/wire"
)

func InitializeUserService() service.UserService {
	panic(wire.Build(service.NewUserService, repo.NewUserRepository))
}
