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

func TestRenderFullTrackInfersSegmentForRailwayTelemetryCodes(t *testing.T) {
	track := renderTrack(transit.TrainPosition{
		Line:         "ST",
		Direction:    "N",
		PreviousStop: "DA",
		NextStop:     "SCAJ",
		InMotion:     true,
	}, []transit.LineStop{
		{Code: "UN", Order: 1},
		{Code: "KE", Order: 2},
		{Code: "AG", Order: 3},
	}, true)

	if !strings.Contains(track, "UN━━●━━KE") {
		t.Fatalf("expected dot between public stops UN and KE, got %q", track)
	}
}

func TestRenderFullTrackAddsMissingTripEndpoint(t *testing.T) {
	track := renderTrack(transit.TrainPosition{
		Line:         "LW",
		Direction:    "W",
		PreviousStop: "BAYV",
		NextStop:     "HAMJ",
		LastStop:     "WR",
		InMotion:     true,
	}, []transit.LineStop{
		{Code: "BU", Order: 10},
		{Code: "AL", Order: 11},
	}, true)

	if !strings.Contains(track, "AL━━●━━WR") {
		t.Fatalf("expected dot between Aldershot and West Harbour, got %q", track)
	}
}

func TestRenderTripTrackUsesTripScheduleWhenTelemetryIsOffMap(t *testing.T) {
	now := time.Date(2026, 5, 14, 22, 19, 0, 0, time.Local)
	track := renderTripTrack(transit.TrainPosition{
		Line:         "ST",
		Direction:    "N",
		PreviousStop: "DA",
		NextStop:     "SCAJ",
		InMotion:     true,
	}, []transit.TripStop{
		{Code: "UN", Order: 1, DepartureComputed: "22:00"},
		{Code: "KE", Order: 2, ArrivalComputed: "22:18", DepartureComputed: "22:18"},
		{Code: "AG", Order: 3, ArrivalComputed: "22:28"},
		{Code: "MK", Order: 4, ArrivalComputed: "22:34"},
	}, true, now)

	if !strings.Contains(track, "KE━━●━━AG") {
		t.Fatalf("expected dot between public stops KE and AG, got %q", track)
	}
}

func TestRenderTripTrackIncludesVariantEndpoint(t *testing.T) {
	now := time.Date(2026, 5, 14, 22, 45, 0, 0, time.Local)
	track := renderTripTrack(transit.TrainPosition{
		Line:         "LW",
		Direction:    "W",
		PreviousStop: "BAYV",
		NextStop:     "HAMJ",
		InMotion:     true,
	}, []transit.TripStop{
		{Code: "BU", Order: 1, DepartureComputed: "22:20"},
		{Code: "AL", Order: 2, DepartureComputed: "22:40"},
		{Code: "WR", Order: 3, ArrivalComputed: "22:50"},
	}, true, now)

	if !strings.Contains(track, "AL━━●━━WR") {
		t.Fatalf("expected dot between actual trip stops AL and WR, got %q", track)
	}
}

func TestTripSegmentByTimeHandlesOvernightTrips(t *testing.T) {
	now := time.Date(2026, 5, 15, 0, 2, 0, 0, time.Local)
	left, right, ok := tripSegmentByTime([]transit.TripStop{
		{Code: "A", Order: 1, DepartureComputed: "23:58"},
		{Code: "B", Order: 2, ArrivalComputed: "00:05"},
	}, now)

	if !ok || left != "A" || right != "B" {
		t.Fatalf("expected overnight segment A-B, got %q-%q ok=%v", left, right, ok)
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
		trips: map[string][]transit.TripStop{
			"1234": {
				{Code: "UN", Order: 1, DepartureComputed: "12:10"},
				{Code: "EX", Order: 2, ArrivalComputed: "12:18"},
			},
		},
	}

	if !strings.Contains(m.headerView(), "scroll") {
		t.Fatal("expected header to expose scroll controls")
	}
	body := m.bodyView()
	if !strings.Contains(body, "Active alerts") || !strings.Contains(body, "Live trains") {
		t.Fatalf("expected body to include alerts and trains: %q", body)
	}
	if !strings.Contains(body, "arrives EX at 12:18") {
		t.Fatalf("expected body to include next station timing: %q", body)
	}
}

func TestRefreshedAgoUsesSnapshotReceiveTime(t *testing.T) {
	now := time.Date(2026, 5, 14, 19, 45, 30, 0, time.Local)

	if got := refreshedAgo(now.Add(-12*time.Second), now); got != "12s ago" {
		t.Fatalf("expected seconds, got %q", got)
	}
	if got := refreshedAgo(now.Add(-3*time.Minute), now); got != "3m ago" {
		t.Fatalf("expected minutes, got %q", got)
	}
	if got := refreshedAgo(now.Add(time.Second), now); got != "0s ago" {
		t.Fatalf("expected future clock skew to clamp, got %q", got)
	}
	if got := refreshedAgo(time.Time{}, now); got != "just now" {
		t.Fatalf("expected zero-time fallback, got %q", got)
	}
}

func TestDelayTextShowsDelayedTrainsInRed(t *testing.T) {
	got := delayText(-19 * 60)

	if !strings.Contains(got, "delayed by 19 minutes") {
		t.Fatalf("expected human delayed label, got %q", got)
	}
}

func TestDelayTextPluralizesOneMinute(t *testing.T) {
	if got := delayText(-30); !strings.Contains(got, "delayed by 1 minute") {
		t.Fatalf("expected singular delayed label, got %q", got)
	}
}

func TestTrainTimingStatusShowsNextStationArrival(t *testing.T) {
	got := trainTimingStatus(transit.TrainPosition{
		NextStop: "EX",
	}, []transit.TripStop{
		{Code: "UN", Order: 1, DepartureComputed: "12:10"},
		{Code: "EX", Order: 2, ArrivalComputed: "12:18"},
	})

	if got != "arrives EX at 12:18" {
		t.Fatalf("expected next station arrival, got %q", got)
	}
}

func TestTrainTimingStatusShowsCurrentStationDeparture(t *testing.T) {
	at := "UN"
	got := trainTimingStatus(transit.TrainPosition{
		AtStation: &at,
		NextStop:  "EX",
	}, []transit.TripStop{
		{Code: "UN", Order: 1, DepartureComputed: "12:10"},
		{Code: "EX", Order: 2, ArrivalComputed: "12:18"},
	})

	if got != "departs UN at 12:10" {
		t.Fatalf("expected current station departure, got %q", got)
	}
}

func TestTrainTimingStatusFallsBackToScheduledArrival(t *testing.T) {
	got := trainTimingStatus(transit.TrainPosition{
		NextStop: "EX",
	}, []transit.TripStop{
		{Code: "EX", Order: 2, ArrivalScheduled: "12:18"},
	})

	if got != "scheduled to arrive EX at 12:18" {
		t.Fatalf("expected scheduled arrival fallback, got %q", got)
	}
}

func TestTrainTimingStatusFallsBackToScheduledDeparture(t *testing.T) {
	at := "UN"
	got := trainTimingStatus(transit.TrainPosition{
		AtStation: &at,
	}, []transit.TripStop{
		{Code: "UN", Order: 1, DepartureScheduled: "12:10"},
	})

	if got != "scheduled to depart UN at 12:10" {
		t.Fatalf("expected scheduled departure fallback, got %q", got)
	}
}
