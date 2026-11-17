package pkga

import "github.com/google/wire"

type Config struct{ A string }

func NewConfig() *Config { return &Config{} }

var Set = wire.NewSet(NewConfig)
