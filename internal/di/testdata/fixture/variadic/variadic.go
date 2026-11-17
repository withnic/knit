package variadic

type Option func(*Client)

type Client struct{}

func NewClient(opts ...Option) *Client { return &Client{} }
