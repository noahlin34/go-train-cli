package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/noah/go-train-cli/pkg/transit"
)

func TestRenderFullTrackUsesOrderedStops(t *testing.T) {
	track := renderTrack(transit.TrainPosition{
		Line:         "LW",
		Direction:    "W",
		PreviousStop: "MI",
		NextStop:     "LO",
		InMotion:     true,
	}, []transit.LineStop{
		{Code: "UN", Order: 1},
		{Code: "EX", Order: 2},
		{Code: "MI", Order: 3},
		{Code: "LO", Order: 4},
		{Code: "PO", Order: 5},
	}, true)

	for _, code := range []string{"UN", "EX", "MI", "LO", "PO"} {
		if !strings.Contains(track, code) {
			t.Fatalf("expected full track to include %s: %q", code, track)
		}
	}
	if !strings.Contains(track, "●") {
		t.Fatalf("expected track to include train dot: %q", track)
	}
}

func TestViewSplitsHeaderAndScrollableBody(t *testing.T) {
	m := model{
		line:    "LW",
		refresh: 20,
		alerts: []transit.Alert{{
			Category: "Service Disruption",
			Subject:  "Track work",
			Lines:    []string{"LW"},
		}},
		trains: []transit.TrainPosition{{
			Line:          "LW",
			TripNumber:    "1234",
			Display:       "LW - Aldershot GO",
			PreviousStop:  "UN",
			NextStop:      "EX",
			PositionLabel: "between UN and EX",
		}},
	}

	if !strings.Contains(m.headerView(), "scroll") {
		t.Fatal("expected header to expose scroll controls")
	}
	body := m.bodyView()
	if !strings.Contains(body, "Active alerts") || !strings.Contains(body, "Live trains") {
		t.Fatalf("expected body to include alerts and trains: %q", body)
	}
}

func TestUpdatedAgoUsesCompactRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 14, 19, 45, 30, 0, time.Local)

	if got := updatedAgo("2026-05-14 19:45:18", now); got != "12s ago" {
		t.Fatalf("expected seconds, got %q", got)
	}
	if got := updatedAgo("2026-05-14 19:42:00", now); got != "3m ago" {
		t.Fatalf("expected minutes, got %q", got)
	}
	if got := updatedAgo("not a timestamp", now); got != "just now" {
		t.Fatalf("expected fallback, got %q", got)
	}
}
