package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sonac/becky/internal/config"
	"github.com/sonac/becky/internal/metadata"
)

type Executor struct{}

type Result struct {
	Path      string
	SHA256    string
	SizeBytes int64
	Duration  time.Duration
}

func NewExecutor() Executor {
	return Executor{}
}

func (e Executor) Run(ctx context.Context, cfg config.Config, entry metadata.Entry) (Result, error) {
	start := time.Now().UTC()

	targetDir := filepath.Join(cfg.Backup.OutputDir, start.Format("2006"), start.Format("01"))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create target dir: %w", err)
	}

	tmpPath := filepath.Join(cfg.Backup.TempDir, fmt.Sprintf("%s.tmp", entry.ID))
	finalName := fmt.Sprintf("%s-%s.archive.gz", entry.Type, start.Format("20060102T150405Z"))
	finalPath := filepath.Join(targetDir, finalName)

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("open temp output file: %w", err)
	}
	defer out.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Backup.TimeoutSeconds)*time.Second)
	defer cancel()

	args := []string{"exec", cfg.Mongo.Container, cfg.Mongo.Mongodump, "--archive", "--gzip"}
	if cfg.Backup.IncludeOplog {
		args = append(args, "--oplog")
	}

	uri := strings.TrimSpace(cfg.Mongo.URI)
	if uri != "" {
		args = append(args, "--uri", uri)
	}
	if cfg.Mongo.AuthDB != "" {
		args = append(args, "--authenticationDatabase", cfg.Mongo.AuthDB)
	}
	if len(cfg.Mongo.ExtraArgs) > 0 {
		args = append(args, cfg.Mongo.ExtraArgs...)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = out
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start mongodump: %w", err)
	}

	stderrData, _ := io.ReadAll(stderrPipe)
	if err := cmd.Wait(); err != nil {
		_ = os.Remove(tmpPath)
		if len(stderrData) > 0 {
			return Result{}, fmt.Errorf("mongodump failed: %s", string(stderrData))
		}
		return Result{}, fmt.Errorf("mongodump failed: %w", err)
	}

	if err := out.Sync(); err != nil {
		return Result{}, fmt.Errorf("sync temp backup: %w", err)
	}
	if err := out.Close(); err != nil {
		return Result{}, fmt.Errorf("close temp backup: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return Result{}, fmt.Errorf("move backup file: %w", err)
	}

	hash, size, err := ChecksumFile(finalPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Path:      finalPath,
		SHA256:    hash,
		SizeBytes: size,
		Duration:  time.Since(start),
	}, nil
}

func ChecksumFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open backup for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("read backup for checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), n, nil
}
