package repo

import "example.com/fixture/missing/db"

type UserRepository struct{ pool *db.Pool }

func NewUserRepository(p *db.Pool) UserRepository { return UserRepository{pool: p} }
