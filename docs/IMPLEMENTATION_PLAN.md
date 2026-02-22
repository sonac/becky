# Mongo Backup Daemon - Implementation Plan

## Locked decisions

- Language/runtime: Go, single Linux host binary
- Mongo is in Docker; backups run through `docker exec`
- Backup schedules: independent `daily` and `weekly`
- Retention policy: keep `10` latest daily and `10` latest weekly
- Restore verification: manual only, out of scope for automation
- Local storage only under `/home/<user>/.becky/`
- Release target: `linux/amd64`
- Distribution: GitHub Releases + install script

## Runtime layout

- Config: `/home/<user>/.becky/config.yaml`
- Backup artifacts: `/home/<user>/.becky/storage/backups/YYYY/MM/*.archive.gz`
- Metadata: `/home/<user>/.becky/storage/backups.json`
- Temp: `/home/<user>/.becky/storage/tmp`
- Lock: `/home/<user>/.becky/storage/lock/mongo-backup.lock`

## Commands

- `init`: scaffold config/storage and optionally configure systemd (`--dry-run` supported; defaults config path to `<home>/.becky/config.yaml`)
- `run`: daemon scheduler mode
- `once`: one immediate backup (`--type daily|weekly`)
- `list`: inspect metadata entries
- `prune`: apply retention immediately
- `verify`: recompute checksum from path/id
- `version`: build metadata

## Backup flow

1. Acquire lock
2. Add metadata entry with `running` status
3. Execute `docker exec <container> mongodump --archive --gzip`
4. Stream output to temp file
5. Move temp to final archive name
6. Compute SHA256 and file size
7. Mark metadata `success` or `failed`
8. Apply retention
9. Save metadata atomically

## Retention behavior

- Keep latest `retention.daily_keep` successful `daily` archives
- Keep latest `retention.weekly_keep` successful `weekly` archives
- Optional additional age pruning via `max_age_days`
- Pruned entries are retained in metadata with status `pruned`

## Metadata schema

Stored in JSON as:

- `schema_version`
- `updated_at`
- `entries[]` with id/type/status/timestamps/path/size/checksum/duration/error

State writes use temporary file + atomic rename.

## Config schema summary

- `mongo`: container/tool/auth options
- `schedule`: timezone + daily/weekly cron expressions
- `backup`: output/temp/timeout/oplog settings
- `retention`: keep counts and max age
- `metadata`: state path + sidecar toggle
- `logging`: level/format

## CI and release

- CI workflow runs `go vet`, `go test`, and a build
- Release workflow triggers on tag `v*`
- Release job builds static `linux/amd64` binary and packages:
  - `mongo-backup`
  - `config.example.yaml`
  - `mongo-backup.service`
- Generates `sha256sums.txt`
- Publishes assets to GitHub Release

## Deployment model

1. Create release tag (for example `v0.1.0`)
2. CI publishes release assets
3. Server runs installer script (binary only)
4. Server runs `mongo-backup init --user <user> --system`
5. Edit `/home/<user>/.becky/config.yaml`
6. Ensure service user can run Docker commands

## Why release artifact install over clone/make

- Reproducible and pinned by version tag
- No compiler/toolchain needed on server
- Easier rollback
- Includes checksum verification

## Post-MVP hardening backlog

- Add structured logger integration (`log/slog` wiring)
- Add backup sidecar metadata output
- Add unit tests for retention/config validation/metadata atomic writes
- Add optional notifications on failures
- Add safe whitelist validation for `mongo.extra_args`
