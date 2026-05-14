package tui

import (
	"strings"
	"testing"

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
