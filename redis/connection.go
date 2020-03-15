package redis

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

// Pool is the gloabl pool of Redis connections opened in main
var Pool *redis.Pool

// NewPool creates a new pool of Redis connections
func NewPool(address string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,

		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", address)
			if err != nil {
				return nil, err
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		}}
}
