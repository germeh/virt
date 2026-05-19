package node

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.ConfigDir != "/etc/virt" {
		t.Fatalf("ConfigDir = %q", cfg.ConfigDir)
	}
	if cfg.DataDir != "/var/lib/virt" {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestConfigFromEnvOverridesDefaults(t *testing.T) {
	env := map[string]string{
		"WARM_MIGRATION_LISTEN_ADDR": "0.0.0.0:9090",
		"WARM_MIGRATION_CONFIG_DIR":  "/tmp/virt-config",
		"WARM_MIGRATION_DATA_DIR":    "/tmp/virt-data",
		"WARM_MIGRATION_LOG_LEVEL":   "debug",
	}

	cfg := ConfigFromEnv(func(key string) string {
		return env[key]
	})

	if cfg.ListenAddr != "0.0.0.0:9090" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.ConfigDir != "/tmp/virt-config" {
		t.Fatalf("ConfigDir = %q", cfg.ConfigDir)
	}
	if cfg.DataDir != "/tmp/virt-data" {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestConfigValidateRejectsInvalidListenAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{name: "blank", addr: ""},
		{name: "missing port", addr: "127.0.0.1"},
		{name: "invalid port", addr: "127.0.0.1:nope"},
		{name: "port out of range", addr: "127.0.0.1:70000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.ListenAddr = tt.addr

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() error = nil")
			}
		})
	}
}

func TestConfigValidateAcceptsHostPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ListenAddr = "0.0.0.0:8080"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
