package app

import (
	"example.com/fixture/samename/pkga"
	"example.com/fixture/samename/pkgb"
)

type App struct {
	A *pkga.Config
	B *pkgb.Config
}

func NewApp(a *pkga.Config, b *pkgb.Config) *App { return &App{A: a, B: b} }
