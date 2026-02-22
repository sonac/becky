# Agent Notes

## Release behavior

- Every commit merged into `main` must produce a downloadable release artifact for `linux/amd64`.
- Do not rely only on manual version tags to publish installable binaries.
- Keep the release pipeline and installer flow aligned so `scripts/install.sh` can fetch and install the latest artifact from GitHub Releases.
