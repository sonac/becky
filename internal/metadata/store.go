package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	path string
}

func NewStore(path string) Store {
	return Store{path: path}
}

func (s Store) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(), nil
		}
		return State{}, fmt.Errorf("read state file: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("decode state file: %w", err)
	}

	if st.SchemaVersion == 0 {
		st.SchemaVersion = 1
	}
	if st.Entries == nil {
		st.Entries = []Entry{}
	}

	return st, nil
}

func (s Store) Save(st State) error {
	st.UpdatedAt = time.Now().UTC()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("ensure state dir: %w", err)
	}

	tmpPath := s.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open temp state file: %w", err)
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		_ = f.Close()
		return fmt.Errorf("encode state file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp state file: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}

	return nil
}
