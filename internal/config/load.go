package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse yaml: %w", err)
	}

	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	if err := os.MkdirAll(cfg.Backup.OutputDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create backup output dir: %w", err)
	}
	if err := os.MkdirAll(cfg.Backup.TempDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create backup temp dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Metadata.StateFile), 0o755); err != nil {
		return Config{}, fmt.Errorf("create metadata dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.LockFile()), 0o755); err != nil {
		return Config{}, fmt.Errorf("create lock dir: %w", err)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Mongo.Mongodump == "" {
		cfg.Mongo.Mongodump = "mongodump"
	}
	if cfg.Schedule.Timezone == "" {
		cfg.Schedule.Timezone = "UTC"
	}
	if cfg.Backup.TimeoutSeconds <= 0 {
		cfg.Backup.TimeoutSeconds = 3600
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

func (c Config) LockFile() string {
	base := filepath.Dir(c.Metadata.StateFile)
	return filepath.Join(base, "lock", "mongo-backup.lock")
}
