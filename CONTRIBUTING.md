# Contributing

Issues and pull requests are welcome at https://github.com/MikeWills/aprsmonitor.

- **Test first.** Write the failing test, watch it fail, then write the code.
  `go test ./...` needs no network (APRS-IS and SMTP are faked in-process).
- Run `gofmt -l .` (must print nothing), `go vet ./...` and `go test ./...`
  before opening a PR; CI runs the same.
- Commits use [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`). One logical change per PR.
- Design rationale lives in [docs/design.md](docs/design.md). If your change
  alters behaviour, update the docs in the same PR.
- New alert channels implement `notify.Notifier` (see `internal/notify`).
