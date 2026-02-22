package config

type Config struct {
	Version   int             `yaml:"version"`
	Mongo     MongoConfig     `yaml:"mongo"`
	Schedule  ScheduleConfig  `yaml:"schedule"`
	Backup    BackupConfig    `yaml:"backup"`
	Retention RetentionConfig `yaml:"retention"`
	Metadata  MetadataConfig  `yaml:"metadata"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type MongoConfig struct {
	Container string   `yaml:"container"`
	Mongodump string   `yaml:"mongodump_path"`
	URI       string   `yaml:"uri"`
	AuthDB    string   `yaml:"auth_db"`
	ExtraArgs []string `yaml:"extra_args"`
}

type ScheduleConfig struct {
	Timezone   string `yaml:"timezone"`
	DailyCron  string `yaml:"daily_cron"`
	WeeklyCron string `yaml:"weekly_cron"`
}

type BackupConfig struct {
	OutputDir      string `yaml:"output_dir"`
	TempDir        string `yaml:"temp_dir"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	IncludeOplog   bool   `yaml:"include_oplog"`
}

type RetentionConfig struct {
	DailyKeep  int `yaml:"daily_keep"`
	WeeklyKeep int `yaml:"weekly_keep"`
	MaxAgeDays int `yaml:"max_age_days"`
}

type MetadataConfig struct {
	StateFile        string `yaml:"state_file"`
	SidecarPerBackup bool   `yaml:"sidecar_per_backup"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}
