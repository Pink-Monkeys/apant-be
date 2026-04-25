package queue

import "strings"

type RedisClient struct {
	Addr string
}

func NewRedisClient(addr string) *RedisClient {
	return &RedisClient{Addr: strings.TrimSpace(addr)}
}
