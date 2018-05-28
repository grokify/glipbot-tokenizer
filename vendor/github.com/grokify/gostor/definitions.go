package gostor

import (
	"encoding/json"
	"strconv"
	"strings"
)

type Config struct {
	Host        string `json:"host,omitempty`
	Port        int    `json:"port,omitempty`
	Password    string `json:"password,omitempty`
	CustomIndex int    `json:"customIndex,omitempty` // Redis Database Index
}

func ParseConfig(jsonBytes []byte) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(jsonBytes, &cfg)
	return &cfg, err
}

func (cfg *Config) HostPort() string {
	parts := []string{}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if len(cfg.Host) > 0 {
		parts = append(parts, cfg.Host)
	}
	if (cfg.Port) > 0 {
		parts = append(parts, strconv.Itoa(cfg.Port))
	}
	return strings.Join(parts, ":")
}

type Client interface {
	SetString(key, val string) error
	GetOrEmptyString(key string) string
}
