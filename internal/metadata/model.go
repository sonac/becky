package metadata

import (
	"fmt"
	"strings"
	"time"
)

type BackupType string

const (
	BackupDaily  BackupType = "daily"
	BackupWeekly BackupType = "weekly"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusPruned  Status = "pruned"
)

type State struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Entries       []Entry   `json:"entries"`
}

type Entry struct {
	ID         string     `json:"id"`
	Type       BackupType `json:"type"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at"`
	Status     Status     `json:"status"`
	Path       string     `json:"path"`
	SizeBytes  int64      `json:"size_bytes"`
	SHA256     string     `json:"sha256"`
	DurationMS int64      `json:"duration_ms"`
	Error      string     `json:"error"`
}

func ParseBackupType(v string) (BackupType, error) {
	s := strings.ToLower(strings.TrimSpace(v))
	switch BackupType(s) {
	case BackupDaily, BackupWeekly:
		return BackupType(s), nil
	default:
		return "", fmt.Errorf("unsupported backup type %q", v)
	}
}

func NewState() State {
	return State{SchemaVersion: 1, UpdatedAt: time.Now().UTC(), Entries: []Entry{}}
}

func NewRunningEntry(t BackupType) Entry {
	ts := time.Now().UTC()
	id := fmt.Sprintf("%s-%s", ts.Format("20060102T150405Z"), t)
	return Entry{
		ID:        id,
		Type:      t,
		StartedAt: ts,
		Status:    StatusRunning,
	}
}
