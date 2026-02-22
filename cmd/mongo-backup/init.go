package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	username := fs.String("user", "", "Linux user for paths and service")
	serviceScope := fs.String("service", "none", "Service scope: system|user|none")
	systemService := fs.Bool("system", false, "Shortcut for --service system")
	force := fs.Bool("force", false, "Overwrite existing config.yaml")
	dryRun := fs.Bool("dry-run", false, "Print planned changes without writing files or changing services")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	configExplicit := false
	serviceExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			configExplicit = true
		}
		if f.Name == "service" {
			serviceExplicit = true
		}
	})

	if *systemService {
		if serviceExplicit && strings.ToLower(strings.TrimSpace(*serviceScope)) != "system" {
			fmt.Fprintln(os.Stderr, "--system conflicts with --service when --service is not 'system'")
			return exitConfig
		}
		*serviceScope = "system"
	}

	if *username == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to detect current user: %v\n", err)
			return exitRuntime
		}
		*username = u.Username
	}

	homeDir, err := homeForUser(*username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve user home: %v\n", err)
		return exitRuntime
	}

	if !configExplicit {
		*configPath = filepath.Join(homeDir, ".becky", "config.yaml")
	}

	if !filepath.IsAbs(*configPath) {
		fmt.Fprintln(os.Stderr, "--config must be an absolute path")
		return exitConfig
	}

	appDir := filepath.Dir(*configPath)
	backupDir := filepath.Join(appDir, "storage", "backups")
	tempDir := filepath.Join(appDir, "storage", "tmp")
	lockDir := filepath.Join(appDir, "storage", "lock")
	stateFile := filepath.Join(appDir, "storage", "backups.json")

	for _, p := range []string{appDir, backupDir, tempDir, lockDir} {
		if *dryRun {
			fmt.Printf("[dry-run] mkdir -p %s\n", p)
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create directory %s: %v\n", p, err)
			return exitRuntime
		}
	}

	if _, err := os.Stat(*configPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "config already exists at %s (use --force to overwrite)\n", *configPath)
		return exitConfig
	}

	configContent := fmt.Sprintf(`version: 1

mongo:
  container: "mongo"
  mongodump_path: "mongodump"
  uri_env_var: "MONGO_URI"
  auth_db: "admin"
  extra_args: []

schedule:
  timezone: "UTC"
  daily_cron: "0 2 * * *"
  weekly_cron: "0 3 * * 0"

backup:
  output_dir: %q
  temp_dir: %q
  timeout_seconds: 3600
  include_oplog: true

retention:
  daily_keep: 10
  weekly_keep: 10
  max_age_days: 0

metadata:
  state_file: %q
  sidecar_per_backup: true

logging:
  level: "info"
  format: "json"
`, backupDir, tempDir, stateFile)

	if *dryRun {
		fmt.Printf("[dry-run] write %s\n", *configPath)
	} else {
		if err := os.WriteFile(*configPath, []byte(configContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write config: %v\n", err)
			return exitRuntime
		}
	}

	scope := strings.ToLower(strings.TrimSpace(*serviceScope))
	switch scope {
	case "none":
	case "system":
		if err := setupSystemService(*username, homeDir, *configPath, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "service setup failed: %v\n", err)
			return exitRuntime
		}
	case "user":
		if err := setupUserService(homeDir, *configPath, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "user service setup failed: %v\n", err)
			return exitRuntime
		}
	default:
		fmt.Fprintln(os.Stderr, "--service must be one of: system, user, none")
		return exitConfig
	}

	fmt.Printf("initialized config at %s\n", *configPath)
	if *dryRun {
		fmt.Println("dry-run mode: no files or services were changed")
	}
	if scope != "none" {
		fmt.Printf("systemd service configured (%s)\n", scope)
	}
	if runtime.GOOS != "linux" {
		fmt.Println("note: service setup is intended for Linux/systemd hosts")
	}

	return exitOK
}

func homeForUser(name string) (string, error) {
	u, err := user.Lookup(name)
	if err == nil && u.HomeDir != "" {
		return u.HomeDir, nil
	}
	if name == "" {
		return "", fmt.Errorf("empty username")
	}
	return filepath.Join("/home", name), nil
}

func setupSystemService(username, homeDir, configPath string, dryRun bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("resolve absolute executable path: %w", err)
	}

	unit := fmt.Sprintf(`[Unit]
Description=Mongo backup scheduler
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
Environment=HOME=%s
WorkingDirectory=%s
ExecStart=%s run --config %s
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, username, username, homeDir, homeDir, execPath, configPath)

	servicePath := "/etc/systemd/system/mongo-backup.service"
	if dryRun {
		fmt.Printf("[dry-run] write %s\n", servicePath)
		fmt.Println("[dry-run] run systemctl daemon-reload")
		fmt.Println("[dry-run] run systemctl enable --now mongo-backup")
		return nil
	}

	if err := os.WriteFile(servicePath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write %s (run as root/sudo): %w", servicePath, err)
	}

	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := runCommand("systemctl", "enable", "--now", "mongo-backup"); err != nil {
		return err
	}

	return nil
}

func setupUserService(homeDir, configPath string, dryRun bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("resolve absolute executable path: %w", err)
	}

	unitDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if dryRun {
		fmt.Printf("[dry-run] mkdir -p %s\n", unitDir)
	} else {
		if err := os.MkdirAll(unitDir, 0o755); err != nil {
			return fmt.Errorf("create user systemd dir: %w", err)
		}
	}

	unit := fmt.Sprintf(`[Unit]
Description=Mongo backup scheduler
After=default.target

[Service]
Type=simple
ExecStart=%s run --config %s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, execPath, configPath)

	servicePath := filepath.Join(unitDir, "mongo-backup.service")
	if dryRun {
		fmt.Printf("[dry-run] write %s\n", servicePath)
		fmt.Println("[dry-run] run systemctl --user daemon-reload")
		fmt.Println("[dry-run] run systemctl --user enable --now mongo-backup")
		return nil
	}

	if err := os.WriteFile(servicePath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write user service file: %w", err)
	}

	if err := runCommand("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if err := runCommand("systemctl", "--user", "enable", "--now", "mongo-backup"); err != nil {
		return err
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
