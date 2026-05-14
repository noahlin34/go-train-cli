# gotrain

A Go-powered CLI and TUI for GO Transit live data from the Metrolinx Open Data API.

The tool has two personalities:

- Deterministic command output for scripts, agents, and future backend use.
- An interactive terminal dashboard for watching live train positions by line.

## Setup

Create a `.env` file or export the key in your shell:

```sh
GO_API_KEY=your-metrolinx-key
```

The CLI also accepts `--api-key`, but environment variables are safer for everyday use.

## Commands

```sh
go run ./cmd/gotrain stations oak
go run ./cmd/gotrain stations --json
go run ./cmd/gotrain departures UN
go run ./cmd/gotrain line-stops LW W
go run ./cmd/gotrain trains --line LW
go run ./cmd/gotrain trains --line LW --json
go run ./cmd/gotrain train 1031 --json
go run ./cmd/gotrain alerts --line LW
go run ./cmd/gotrain status --line LW
go run ./cmd/gotrain tui --line LW
go run ./cmd/gotrain serve --addr 127.0.0.1:8787
```

Machine-readable commands use stable JSON field names and explicit timestamps:

```sh
go run ./cmd/gotrain trains --line LW --json | jq '.data[0]'
```

The default human output is compact table text suitable for a terminal. The `departures`
command intentionally emits the raw normalized API payload under `data`, because the
Metrolinx next-service shape varies by stop and service type.

## Package Shape

```text
cmd/gotrain/       binary entrypoint
internal/cli/      Cobra command wiring and terminal table output
internal/tui/      Bubble Tea live dashboard
pkg/config/        .env and environment loading
pkg/metrolinx/     typed Metrolinx API client
pkg/transit/       normalized domain model and tracking inference
pkg/output/        JSON and table renderers
```

The core transit code is intentionally separate from the CLI. A future `gotrain serve`
or MCP/agent adapter can reuse the same packages without scraping terminal output.

## HTTP Mode

`gotrain serve` exposes deterministic JSON endpoints:

```text
GET /healthz
GET /stations?q=union
GET /departures/UN
GET /line-stops/LW/W
GET /trains?line=LW
GET /trains/{trip}
GET /alerts?line=LW
```

## Notes

- API key lookup order: `--api-key`, `GO_API_KEY`, `GO_TRAIN_API_KEY`.
- Default API base URL: `https://api.openmetrolinx.com/OpenDataAPI`.
- Override the base URL with `GO_API_BASE_URL` for tests or local mocks.
- Live train position is derived from the API's previous, next, and at-station fields.
