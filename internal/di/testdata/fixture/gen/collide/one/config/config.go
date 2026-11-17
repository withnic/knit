package config

type Config struct{ Name string }

func New() *Config { return &Config{Name: "one"} }
