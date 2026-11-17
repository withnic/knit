package service

import "example.com/fixture/missing/repo"

type UserService struct{ repo repo.UserRepository }

func NewUserService(r repo.UserRepository) UserService { return UserService{repo: r} }
