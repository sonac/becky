package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
)

func Validate(cfg Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	if cfg.Mongo.Container == "" {
		return errors.New("mongo.container is required")
	}
	if !filepath.IsAbs(cfg.Backup.OutputDir) {
		return errors.New("backup.output_dir must be absolute")
	}
	if !filepath.IsAbs(cfg.Backup.TempDir) {
		return errors.New("backup.temp_dir must be absolute")
	}
	if !filepath.IsAbs(cfg.Metadata.StateFile) {
		return errors.New("metadata.state_file must be absolute")
	}
	if cfg.Retention.DailyKeep < 0 || cfg.Retention.WeeklyKeep < 0 {
		return errors.New("retention keep values must be >= 0")
	}
	if cfg.Retention.DailyKeep == 0 && cfg.Retention.WeeklyKeep == 0 {
		return errors.New("at least one retention keep value must be > 0")
	}
	if cfg.Backup.TimeoutSeconds <= 0 {
		return errors.New("backup.timeout_seconds must be > 0")
	}
	if _, err := time.LoadLocation(cfg.Schedule.Timezone); err != nil {
		return fmt.Errorf("invalid schedule.timezone: %w", err)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cfg.Schedule.DailyCron); err != nil {
		return fmt.Errorf("invalid schedule.daily_cron: %w", err)
	}
	if _, err := parser.Parse(cfg.Schedule.WeeklyCron); err != nil {
		return fmt.Errorf("invalid schedule.weekly_cron: %w", err)
	}

	return nil
}
