# Repository Guidelines

## Project Structure & Module Organization

This is a Go CLI/TUI for GO Transit data. The binary entrypoint lives in `cmd/gotrain/`. Command wiring and terminal output are in `internal/cli/`, while the Bubble Tea interactive dashboard is in `internal/tui/`.

Reusable packages live under `pkg/`: `pkg/metrolinx/` contains the API client and response types, `pkg/transit/` normalizes domain data and tracking state, `pkg/config/` loads `.env` and environment settings, and `pkg/output/` handles JSON/table rendering. Tests sit beside the package they cover, for example `pkg/transit/service_test.go`.

## Build, Test, and Development Commands

Use standard Go tooling:

```sh
go test ./...                  # run all tests
go build -o gotrain ./cmd/gotrain
go run ./cmd/gotrain --help
go run ./cmd/gotrain trains --line LW --json
go run ./cmd/gotrain tui --line LW
```

For live API commands, create `.env` with `GO_API_KEY=...` or export `GO_API_KEY` in the shell. Do not commit `.env`.

## Coding Style & Naming Conventions

Format Go code with `gofmt -w`. Keep package names short and lowercase (`transit`, `metrolinx`, `output`). Use exported names only for APIs intended outside the package. Prefer small structs with JSON tags for deterministic machine-readable output. Keep CLI presentation code in `internal/cli` or `internal/tui`; do not mix UI formatting into `pkg/transit`.

## Testing Guidelines

Use Go’s built-in `testing` package. Name tests `TestThingBehavior`, and place them in `*_test.go` files beside the code under test. Prefer unit tests for normalization, filtering, and formatting logic. Live API calls are useful for smoke testing but should not be required for `go test ./...`.

## Commit & Pull Request Guidelines

This repository has no commit history yet, so no existing convention is established. Use short imperative commit messages, such as `Add live train tracking` or `Document API configuration`.

Pull requests should include a brief description, commands run, and notes about API-impacting changes. For TUI changes, include a screenshot or short terminal recording when practical. Mention any new environment variables or API endpoints.

## Security & Configuration Tips

Never print or commit API keys. `.env` is intentionally ignored. Configuration lookup order is `--api-key`, `GO_API_KEY`, then `GO_TRAIN_API_KEY`. Use `GO_API_BASE_URL` only for mocks or controlled test environments.
