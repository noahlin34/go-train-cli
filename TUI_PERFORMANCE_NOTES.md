# TUI Performance Follow-Up Notes

This file records remaining TUI performance and memory opportunities from the May 2026 scan. The overlapping automatic refresh issue was fixed in commit `a4acc1d` by guarding tick-triggered fetches while `loading` is true.

## Goals

- Reduce steady-state memory churn while the Bubble Tea TUI is open.
- Preserve all current TUI functionality and appearance.
- Prefer small, measurable changes with focused benchmarks.

## Remaining Findings

### Full body rebuild on every blink tick

- Location: `internal/tui/tui.go`, `Update` handling for `tickMsg`, around the `m.viewport.SetContent(m.bodyView())` call.
- Current behavior: every 750ms blink tick rebuilds the entire body string and resets viewport content.
- Cost: repeated allocation and Lip Gloss rendering for alerts, train rows, track strings, timing labels, and refreshed-age text.
- Constraint: the blinking train dot and refreshed-age label still need to update visually.
- Possible direction: benchmark first, then consider caching mostly-static row fragments or separating static snapshot-derived content from tick-derived fragments. Be careful not to change scroll behavior or visual output.

### Trip stop conversion allocates during render

- Location: `renderTripTrack` and `tripStopsAsLineStops` in `internal/tui/tui.go`.
- Current behavior: each render converts `[]transit.TripStop` into `[]transit.LineStop`.
- Cost: per-train slice allocation every redraw, especially noticeable because redraw happens every 750ms.
- Possible direction: precompute trip-derived line stops when building `snapshotMsg`, or add a model-side cache keyed by trip number that is replaced on every successful fetch.
- Preserve behavior: public stop inference and private railway code fallback must remain identical.

### `extendStopsForTrip` copies stops even when unchanged

- Location: `extendStopsForTrip` in `internal/tui/tui.go`.
- Current behavior: starts with `append([]transit.LineStop(nil), stops...)` before checking whether first or last endpoint insertion is needed.
- Cost: avoidable slice copy on most render paths where endpoints are already present.
- Low-risk direction: check whether `FirstStop` or `LastStop` are missing first; return `stops` directly if no extension is needed.
- Watchpoint: only copy before prepending/appending so callers' stored topology/trip slices are never mutated.

### `fetch` closure captures the whole model

- Location: `func (m model) fetch() tea.Cmd` in `internal/tui/tui.go`.
- Current behavior: the returned closure references `m.service` and `m.line`, which means it captures the full model value.
- Cost: slow in-flight fetches can retain previous trains, alerts, topology, trips, and viewport content longer than necessary.
- Possible direction: copy the needed fields into locals before returning the closure:

```go
service := m.service
line := m.line
return func() tea.Msg {
    snap, err := service.Trains(ctx, line)
    // ...
}
```

- Preserve behavior: manual line switching should still fetch the line value active when the command was created.

### Sequential trip schedule fetches per train

- Location: `fetch`, second train loop calling `m.service.TripStops`.
- Current behavior: trip stops are fetched one trip at a time for every visible train on every refresh.
- Cost: refresh latency, API pressure, and in-flight memory retention can grow in all-lines mode.
- Possible direction: consider bounded concurrency, caching by trip number/day, or fetching trip details only for visible/needed trains.
- Watchpoint: changing fetch timing or partial data availability could affect displayed track labels; benchmark and test carefully.

### Minor render-time churn

- Location: `publicStopTime` in `internal/tui/tui.go`.
- Current behavior: creates a temporary `[]string` candidate list each time it checks a stop's schedule fields.
- Low-risk direction: replace the slice literal loop with straight-line checks or a small helper that does not allocate.

- Location: `bodyView`, train row footer rendering.
- Current behavior: calls `time.Now()` per train when rendering refreshed-age text.
- Low-risk direction: capture `now := time.Now()` once at the top of `bodyView` and reuse it for all rows.

- Location: `headerView`.
- Current behavior: rebuilt in both `View` and window-size handling.
- Possible direction: cache header text/height when `line` or `refresh` changes. This is likely lower impact than body/track render work.

## Suggested Benchmarks

Add benchmarks in `internal/tui/tui_test.go` or a new `internal/tui/tui_benchmark_test.go`:

- `BenchmarkBodyViewBigSnapshot`: model with several alerts, 20-50 trains, realistic trip stops, and `b.ReportAllocs()`.
- `BenchmarkRenderTripTrack`: realistic trip stop list and private telemetry code path.
- `BenchmarkRenderFullTrack`: representative line stop list with and without endpoint extension.
- `BenchmarkTripSegmentByTime`: schedule inference across typical and overnight trips.

Useful command once benchmarks exist:

```sh
go test ./internal/tui -run '^$' -bench 'Benchmark(BodyView|Render|Trip)' -benchmem
```

Always run the full suite afterward:

```sh
go test ./...
```

