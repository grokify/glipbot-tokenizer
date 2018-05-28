package redis

import (
	redisLib "github.com/go-redis/redis"
	"github.com/grokify/gostor"
)

type Client struct {
	redisClient *redisLib.Client
}

func NewClient(cfg gostor.Config) *Client {
	return &Client{
		redisClient: redisLib.NewClient(NewRedisOptions(cfg))}
}

func (client Client) SetString(key, val string) error {
	return client.redisClient.Set(key, val, 0).Err()
}

func (client Client) GetOrEmptyString(key string) string {
	if val, err := client.redisClient.Get(key).Result(); err == nil {
		return val
	}
	return ""
}

func NewRedisOptions(cfg gostor.Config) *redisLib.Options {
	return &redisLib.Options{
		Addr:     cfg.HostPort(),
		Password: cfg.Password,
		DB:       cfg.CustomIndex}
}
