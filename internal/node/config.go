package node

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr string
	ConfigDir  string
	DataDir    string
	LogLevel   string
}

func DefaultConfig() Config {
	return Config{
		ListenAddr: "127.0.0.1:8080",
		ConfigDir:  "/etc/virt",
		DataDir:    "/var/lib/virt",
		LogLevel:   "info",
	}
}

func ConfigFromEnv(getenv func(string) string) Config {
	cfg := DefaultConfig()
	overrideString(&cfg.ListenAddr, getenv("WARM_MIGRATION_LISTEN_ADDR"))
	overrideString(&cfg.ConfigDir, getenv("WARM_MIGRATION_CONFIG_DIR"))
	overrideString(&cfg.DataDir, getenv("WARM_MIGRATION_DATA_DIR"))
	overrideString(&cfg.LogLevel, getenv("WARM_MIGRATION_LOG_LEVEL"))
	return cfg
}

func (c Config) Validate() error {
	if err := validateListenAddr(c.ListenAddr); err != nil {
		return err
	}
	if strings.TrimSpace(c.ConfigDir) == "" {
		return fmt.Errorf("config dir is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data dir is required")
	}
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("log level must be one of debug, info, warn, error")
	}
}

func validateListenAddr(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("listen address is required")
	}
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address must be host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("listen port must be numeric: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535")
	}
	return nil
}

func overrideString(target *string, value string) {
	if strings.TrimSpace(value) != "" {
		*target = value
	}
}
