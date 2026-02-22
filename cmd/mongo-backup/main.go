package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/sonac/becky/internal/backup"
	"github.com/sonac/becky/internal/config"
	"github.com/sonac/becky/internal/lock"
	"github.com/sonac/becky/internal/metadata"
	"github.com/sonac/becky/internal/retention"
	"github.com/sonac/becky/internal/scheduler"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitConfig  = 2
	exitLock    = 3
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(exitConfig)
	}

	switch os.Args[1] {
	case "init":
		os.Exit(cmdInit(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "once":
		os.Exit(cmdOnce(os.Args[2:]))
	case "list":
		os.Exit(cmdList(os.Args[2:]))
	case "prune":
		os.Exit(cmdPrune(os.Args[2:]))
	case "verify":
		os.Exit(cmdVerify(os.Args[2:]))
	case "version":
		fmt.Printf("mongo-backup version=%s commit=%s built=%s\n", version, commit, date)
		os.Exit(exitOK)
	default:
		usage()
		os.Exit(exitConfig)
	}
}

func usage() {
	fmt.Println("Usage: mongo-backup <command> [flags]")
	fmt.Println("Commands: init, run, once, list, prune, verify, version")
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return exitConfig
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store := metadata.NewStore(cfg.Metadata.StateFile)
	be := backup.NewExecutor()
	runner := scheduler.NewRunner(cfg, store, be)

	if err := runner.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		return exitRuntime
	}

	return exitOK
}

func cmdOnce(args []string) int {
	fs := flag.NewFlagSet("once", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	backupType := fs.String("type", "daily", "Backup type: daily|weekly")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	t, err := metadata.ParseBackupType(*backupType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid type: %v\n", err)
		return exitConfig
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return exitConfig
	}

	ctx := context.Background()
	if err := withLock(cfg.LockFile(), func() error {
		return runSingle(ctx, cfg, t)
	}); err != nil {
		if errors.Is(err, lock.ErrLocked) {
			fmt.Fprintln(os.Stderr, "another backup process is already running")
			return exitLock
		}
		fmt.Fprintf(os.Stderr, "backup failed: %v\n", err)
		return exitRuntime
	}

	fmt.Println("backup completed")
	return exitOK
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	limit := fs.Int("limit", 50, "Max rows")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return exitConfig
	}

	store := metadata.NewStore(cfg.Metadata.StateFile)
	state, err := store.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load state: %v\n", err)
		return exitRuntime
	}

	entries := state.Entries
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedAt.After(entries[j].StartedAt)
	})

	if *limit > 0 && len(entries) > *limit {
		entries = entries[:*limit]
	}

	for _, e := range entries {
		fmt.Printf("%s | %s | %s | %s | %d bytes\n", e.ID, e.Type, e.Status, e.Path, e.SizeBytes)
	}

	return exitOK
}

func cmdPrune(args []string) int {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return exitConfig
	}

	if err := withLock(cfg.LockFile(), func() error {
		store := metadata.NewStore(cfg.Metadata.StateFile)
		state, err := store.Load()
		if err != nil {
			return err
		}
		next, deleted, err := retention.Apply(state, cfg.Retention, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := store.Save(next); err != nil {
			return err
		}
		for _, d := range deleted {
			fmt.Printf("pruned: %s\n", d)
		}
		return nil
	}); err != nil {
		if errors.Is(err, lock.ErrLocked) {
			fmt.Fprintln(os.Stderr, "another backup process is already running")
			return exitLock
		}
		fmt.Fprintf(os.Stderr, "prune failed: %v\n", err)
		return exitRuntime
	}

	return exitOK
}

func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath(), "Path to config.yaml")
	id := fs.String("id", "", "Backup ID")
	path := fs.String("path", "", "Backup file path")
	if err := fs.Parse(args); err != nil {
		return exitConfig
	}

	if *id == "" && *path == "" {
		fmt.Fprintln(os.Stderr, "either --id or --path is required")
		return exitConfig
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return exitConfig
	}

	store := metadata.NewStore(cfg.Metadata.StateFile)
	state, err := store.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load state: %v\n", err)
		return exitRuntime
	}

	var target metadata.Entry
	var found bool

	if *id != "" {
		for _, e := range state.Entries {
			if e.ID == *id {
				target = e
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "backup id not found: %s\n", *id)
			return exitRuntime
		}
	} else {
		target.Path = *path
	}

	if target.Path == "" {
		fmt.Fprintln(os.Stderr, "target path is empty")
		return exitRuntime
	}

	hash, size, err := backup.ChecksumFile(target.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify failed: %v\n", err)
		return exitRuntime
	}

	if target.SHA256 != "" && target.SHA256 != hash {
		fmt.Fprintf(os.Stderr, "checksum mismatch: expected=%s got=%s\n", target.SHA256, hash)
		return exitRuntime
	}

	fmt.Printf("ok path=%s size=%d sha256=%s\n", target.Path, size, hash)
	return exitOK
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".becky", "config.yaml")
}

func withLock(path string, fn func() error) error {
	l, err := lock.New(path)
	if err != nil {
		return err
	}
	defer l.Close()

	if err := l.TryLock(); err != nil {
		return err
	}
	defer l.Unlock()

	return fn()
}

func runSingle(ctx context.Context, cfg config.Config, t metadata.BackupType) error {
	store := metadata.NewStore(cfg.Metadata.StateFile)
	state, err := store.Load()
	if err != nil {
		return err
	}

	entry := metadata.NewRunningEntry(t)
	state.Entries = append(state.Entries, entry)
	if err := store.Save(state); err != nil {
		return err
	}

	exec := backup.NewExecutor()
	res, runErr := exec.Run(ctx, cfg, entry)

	now := time.Now().UTC()
	for i := range state.Entries {
		if state.Entries[i].ID == entry.ID {
			if runErr != nil {
				state.Entries[i].Status = metadata.StatusFailed
				state.Entries[i].Error = runErr.Error()
				state.Entries[i].FinishedAt = now
			} else {
				state.Entries[i].Status = metadata.StatusSuccess
				state.Entries[i].Path = res.Path
				state.Entries[i].SizeBytes = res.SizeBytes
				state.Entries[i].SHA256 = res.SHA256
				state.Entries[i].DurationMS = res.Duration.Milliseconds()
				state.Entries[i].FinishedAt = now
			}
			break
		}
	}

	state, _, rerr := retention.Apply(state, cfg.Retention, now)
	if rerr != nil {
		return fmt.Errorf("retention failed: %w", rerr)
	}

	if err := store.Save(state); err != nil {
		return err
	}

	if runErr != nil {
		return runErr
	}

	return nil
}
