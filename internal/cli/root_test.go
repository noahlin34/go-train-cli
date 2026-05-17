package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootAboutFlagPrintsRightsNotice(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--about"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected about flag to execute, got %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "Made by noah lin. All rights reserved" {
		t.Fatalf("expected about text, got %q", got)
	}
}

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
