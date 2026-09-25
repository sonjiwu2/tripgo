package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP HTTP
	Log  Log
	DB   DB
}

type HTTP struct {
	Addr            string
	ShutdownTimeout time.Duration
}

type Log struct {
	Level slog.Level
}

type DB struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

type settings struct {
	err error
}

func Load() (Config, error) {
	var src settings
	cfg := Config{
		HTTP: HTTP{
			Addr:            src.text("HTTP_ADDR"),
			ShutdownTimeout: src.duration("SHUTDOWN_TIMEOUT"),
		},
		Log: Log{
			Level: src.logLevel("LOG_LEVEL"),
		},
		DB: DB{
			URL:             src.text("DATABASE_URL"),
			MaxConns:        src.count("DATABASE_MAX_CONNS"),
			MinConns:        src.count("DATABASE_MIN_CONNS"),
			MaxConnLifetime: src.duration("DATABASE_MAX_CONN_LIFETIME"),
			ConnectTimeout:  src.duration("DATABASE_CONNECT_TIMEOUT"),
			QueryTimeout:    src.duration("DATABASE_QUERY_TIMEOUT"),
		},
	}
	if src.err != nil {
		return Config{}, src.err
	}
	if cfg.DB.MinConns > cfg.DB.MaxConns {
		return Config{}, fmt.Errorf(
			"DATABASE_MIN_CONNS (%d) больше DATABASE_MAX_CONNS (%d)",
			cfg.DB.MinConns,
			cfg.DB.MaxConns,
		)
	}
	return cfg, nil
}

func (s *settings) text(name string) string {
	if s.err != nil {
		return ""
	}
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		s.err = fmt.Errorf("не задана переменная %s", name)
		return ""
	}
	return v
}

func (s *settings) duration(name string) time.Duration {
	raw := s.text(name)
	if s.err != nil {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		s.err = fmt.Errorf("%s: не разобрать длительность: %w", name, err)
		return 0
	}
	if d <= 0 {
		s.err = fmt.Errorf("значение %s должно быть больше нуля", name)
		return 0
	}
	return d
}

func (s *settings) count(name string) int32 {
	raw := s.text(name)
	if s.err != nil {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		s.err = fmt.Errorf("%s: нужно целое число: %w", name, err)
		return 0
	}
	if n <= 0 {
		s.err = fmt.Errorf("значение %s должно быть больше нуля", name)
		return 0
	}
	return int32(n)
}

func (s *settings) logLevel(name string) slog.Level {
	raw := s.text(name)
	if s.err != nil {
		return slog.LevelInfo
	}
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		s.err = fmt.Errorf("%s=%q, ожидается debug, info, warn или error", name, raw)
		return slog.LevelInfo
	}
}
