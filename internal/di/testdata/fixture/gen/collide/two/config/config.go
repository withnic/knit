package config

type Config struct{ Name string }

func New() (*Config, error) { return &Config{Name: "two"}, nil }
