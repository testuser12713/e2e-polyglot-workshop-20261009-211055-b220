// Package queue is the API's client for the Valkey list that carries invoice
// jobs. The API only pushes; the Python worker consumes from the other end of
// the list.
package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Valkey connection. Valkey speaks the Redis protocol, so the
// standard Redis client parses the redis:// URL unchanged.
type Client struct {
	rdb *redis.Client
}

// New connects to the Valkey instance addressed by url
// (for example redis://localhost:6379/0).
func New(url string) (*Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse VALKEY_URL: %w", err)
	}
	return &Client{rdb: redis.NewClient(opt)}, nil
}

// Ping verifies that Valkey is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Push appends payload to the head of the named list. The worker pops from the
// tail, which makes the queue first-in-first-out.
func (c *Client) Push(ctx context.Context, list string, payload string) error {
	return c.rdb.LPush(ctx, list, payload).Err()
}

// Close releases the client.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
