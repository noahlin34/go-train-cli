package cli

import "testing"

func TestFormatDelayShowsDelayedPhrase(t *testing.T) {
	if got := formatDelay(-19 * 60); got != "delayed by 19 minutes" {
		t.Fatalf("expected delayed phrase, got %q", got)
	}
}

func TestFormatDelayPluralizesOneMinute(t *testing.T) {
	if got := formatDelay(-30); got != "delayed by 1 minute" {
		t.Fatalf("expected singular delayed phrase, got %q", got)
	}
}

func TestFormatDelayShowsEarlyPhrase(t *testing.T) {
	if got := formatDelay(90); got != "early by 2 minutes" {
		t.Fatalf("expected early phrase, got %q", got)
	}
}
