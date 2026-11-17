package unsupported

type Config struct{ Name string }

type Service struct{ cfg Config }

func NewService(cfg Config) *Service { return &Service{cfg: cfg} }
