package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type AppConfig struct {
	k *koanf.Koanf
}

func NewAppConfig() *AppConfig {
	c := &AppConfig{k: koanf.New(".")}
	setDefaults(c.k)

	return c
}

func (c *AppConfig) Load(filename ...string) bool {
	loaded := false

	for _, name := range filename {
		if !FileExists(name) {
			continue
		}

		if err := c.k.Load(file.Provider(name), yaml.Parser()); err != nil {
			slog.Info(fmt.Sprintf("error loading config: %s", err.Error()))
		} else {
			loaded = true
		}
	}

	return loaded
}

func (c *AppConfig) LoadEnv(prefix string) error {
	return c.k.Load(env.Provider(prefix, ".", func(s string) string {
		s1 := strings.ToLower(strings.TrimPrefix(s, prefix))
		s1 = strings.Replace(s1, "_", ".", -1)
		s1 = strings.Replace(s1, "..", "_", -1)

		slog.Info(fmt.Sprintf("env param %s -> %s", s, s1))

		return s1
	}), nil)
}

func (c *AppConfig) Set(key string, v any) error {
	return c.k.Set(key, v)
}

func (c *AppConfig) Unmarshal(key string, v any) error {
	return c.k.Unmarshal(key, v)
}

func (c *AppConfig) Exists(key string) bool {
	return c.k.Exists(key)
}

func (c *AppConfig) Bool(key string) bool {
	return c.k.Bool(key)
}

func (c *AppConfig) String(key string) string {
	return c.k.String(key)
}

func (c *AppConfig) Float64(key string) float64 {
	return c.k.Float64(key)
}

func (c *AppConfig) Int(key string) int {
	return c.k.Int(key)
}

func (c *AppConfig) Duration(key string) time.Duration {
	return c.k.Duration(key)
}

func setDefaults(k *koanf.Koanf) {
	k.Set("show_my", false)
}

func FileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err)
	}

	return true
}
