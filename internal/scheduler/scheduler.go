package scheduler

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sonac/becky/internal/backup"
	"github.com/sonac/becky/internal/config"
	"github.com/sonac/becky/internal/lock"
	"github.com/sonac/becky/internal/metadata"
	"github.com/sonac/becky/internal/retention"
)

type Runner struct {
	cfg   config.Config
	store metadata.Store
	exec  backup.Executor
}

func NewRunner(cfg config.Config, store metadata.Store, exec backup.Executor) Runner {
	return Runner{cfg: cfg, store: store, exec: exec}
}

func (r Runner) Start(ctx context.Context) error {
	loc, err := time.LoadLocation(r.cfg.Schedule.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	c := cron.New(cron.WithLocation(loc))
	if _, err := c.AddFunc(r.cfg.Schedule.DailyCron, func() {
		if err := r.runWithType(context.Background(), metadata.BackupDaily); err != nil {
			fmt.Fprintf(os.Stderr, "daily backup failed: %v\n", err)
		}
	}); err != nil {
		return fmt.Errorf("register daily cron: %w", err)
	}

	if _, err := c.AddFunc(r.cfg.Schedule.WeeklyCron, func() {
		if err := r.runWithType(context.Background(), metadata.BackupWeekly); err != nil {
			fmt.Fprintf(os.Stderr, "weekly backup failed: %v\n", err)
		}
	}); err != nil {
		return fmt.Errorf("register weekly cron: %w", err)
	}

	c.Start()
	defer c.Stop()

	<-ctx.Done()
	return nil
}

func (r Runner) runWithType(ctx context.Context, t metadata.BackupType) error {
	l, err := lock.New(r.cfg.LockFile())
	if err != nil {
		return err
	}
	defer l.Close()

	if err := l.TryLock(); err != nil {
		if err == lock.ErrLocked {
			return nil
		}
		return err
	}
	defer l.Unlock()

	state, err := r.store.Load()
	if err != nil {
		return err
	}

	entry := metadata.NewRunningEntry(t)
	state.Entries = append(state.Entries, entry)
	if err := r.store.Save(state); err != nil {
		return err
	}

	res, runErr := r.exec.Run(ctx, r.cfg, entry)
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

	state, _, rerr := retention.Apply(state, r.cfg.Retention, now)
	if rerr != nil {
		return rerr
	}

	if err := r.store.Save(state); err != nil {
		return err
	}

	return runErr
}
