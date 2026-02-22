# mongo-backup

Small Go daemon/CLI for scheduled MongoDB backups when Mongo runs inside Docker.

## Features

- Runs on Linux host and executes `docker exec <mongo-container> mongodump --archive --gzip`
- Supports independent daily and weekly schedules
- Keeps latest N daily and latest N weekly backups independently
- Stores backup metadata in JSON state file
- YAML configuration
- Cross-compiled linux/amd64 release artifacts in GitHub Releases

## Configuration paths

Recommended runtime layout:

- Config: `/home/<user>/.becky/config.yaml`
- Backups: `/home/<user>/.becky/storage/backups/`
- Metadata: `/home/<user>/.becky/storage/backups.json`
- Temp: `/home/<user>/.becky/storage/tmp/`

See `configs/config.example.yaml`.

## Commands

```bash
mongo-backup init --user <user> --system
mongo-backup init --dry-run --service none
mongo-backup run --config /home/<user>/.becky/config.yaml
mongo-backup once --config /home/<user>/.becky/config.yaml --type daily
mongo-backup list --config /home/<user>/.becky/config.yaml --limit 20
mongo-backup prune --config /home/<user>/.becky/config.yaml
mongo-backup verify --config /home/<user>/.becky/config.yaml --id <backup-id>
mongo-backup version
```

`init` scaffolds config and storage directories on the local machine and can optionally configure systemd (`--service system|user|none` or `--system`). If `--config` is omitted it defaults to `<user-home>/.becky/config.yaml`. Use `--dry-run` to preview changes.

## Install from GitHub Release

```bash
curl -fsSL https://raw.githubusercontent.com/sonac/becky/main/scripts/install.sh | bash -s -- --version v0.1.0
```

The installer:

- Downloads binary tarball + checksum
- Verifies SHA256
- Installs binary to `/usr/local/bin/mongo-backup`

Then scaffold runtime files with:

```bash
mongo-backup init --user <linux-user> --system
```

## Local development

```bash
make tidy
make test
make build
```

## Notes

- Restore operations are intentionally manual.
- Ensure the service user has permission to run Docker commands.
