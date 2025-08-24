package livekit

type Client struct{}

func NewClient(url, apiKey, apiSecret string) *Client { return &Client{} }
func (c *Client) JoinRoom(room, identity string) error { return nil }
func (c *Client) Close() error { return nil }
