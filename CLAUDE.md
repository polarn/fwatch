# fwatch

Single-binary Go daemon. Watches one directory with fsnotify and moves files into
destination directories by extension. All logic lives in `main.go`.

## Layout

- `main.go` — everything: config loading, watcher loop, routing, move logic.
  `moveFile` falls back to copy+delete on cross-device rename failures.
- `config.example.yaml` — reference config. Real config is
  `$XDG_CONFIG_HOME/fwatch/config.yaml`, else `~/.config/fwatch/config.yaml`.
- `fwatch.service` — the systemd **user** unit, single source of truth: shipped in the
  archives, installed live by the `.deb` and as docs by the AUR package. ExecStart is
  `/usr/bin/fwatch`, the path both packages install to.
- `.goreleaser.yml` — builds, archives, `.deb` packages (nfpms), AUR (`fwatch-bin`) publishing.
- `.github/workflows/` — `test.yml` (vet/test/build on PRs and main),
  `release.yml` (goreleaser, triggered by `X.Y.Z` tags).

## Commands

```bash
go build -o fwatch
go vet ./...
go test -race ./...                      # no tests exist yet
goreleaser release --snapshot --clean    # local packaging check
```

## Conventions

- Commit subjects: short imperative, no ticket prefix ("Add first version").
- Release = push a tag `X.Y.Z`; goreleaser builds, publishes and updates the AUR
  package. `main.version` is injected via ldflags — don't hardcode it.
- Personal repo, not Validio: no reviewers, no ticket references, no MR checklist.
- The built `fwatch` binary and `config.yaml` are gitignored; never commit them.
