package retention

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/sonac/becky/internal/config"
	"github.com/sonac/becky/internal/metadata"
)

func Apply(state metadata.State, cfg config.RetentionConfig, now time.Time) (metadata.State, []string, error) {
	if state.Entries == nil {
		state.Entries = []metadata.Entry{}
	}

	var deleted []string
	keep := make(map[string]struct{})

	keepByType := func(t metadata.BackupType, n int) {
		var items []metadata.Entry
		for _, e := range state.Entries {
			if e.Type == t && e.Status == metadata.StatusSuccess && e.Path != "" {
				items = append(items, e)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].FinishedAt.After(items[j].FinishedAt) })
		if n > len(items) {
			n = len(items)
		}
		for i := 0; i < n; i++ {
			keep[items[i].ID] = struct{}{}
		}
	}

	keepByType(metadata.BackupDaily, cfg.DailyKeep)
	keepByType(metadata.BackupWeekly, cfg.WeeklyKeep)

	for i := range state.Entries {
		e := &state.Entries[i]
		if e.Status != metadata.StatusSuccess || e.Path == "" {
			continue
		}
		if _, ok := keep[e.ID]; !ok {
			if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
				return state, deleted, fmt.Errorf("delete old backup %s: %w", e.Path, err)
			}
			deleted = append(deleted, e.Path)
			e.Status = metadata.StatusPruned
			e.Path = ""
		}
	}

	if cfg.MaxAgeDays > 0 {
		cutoff := now.AddDate(0, 0, -cfg.MaxAgeDays)
		for i := range state.Entries {
			e := &state.Entries[i]
			if e.Status != metadata.StatusSuccess || e.Path == "" {
				continue
			}
			if e.FinishedAt.Before(cutoff) {
				if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
					return state, deleted, fmt.Errorf("delete aged backup %s: %w", e.Path, err)
				}
				deleted = append(deleted, e.Path)
				e.Status = metadata.StatusPruned
				e.Path = ""
			}
		}
	}

	return state, deleted, nil
}
