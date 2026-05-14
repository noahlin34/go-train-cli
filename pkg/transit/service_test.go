package transit

import (
	"testing"

	"github.com/noah/go-train-cli/pkg/metrolinx"
)

func TestNormalizeTrainPositionLabelsAtStation(t *testing.T) {
	at := "UN"
	train := normalizeTrain(metrolinx.TrainTrip{
		TripNumber:    "1234",
		LineCode:      "LW",
		Display:       "LW - Aldershot GO",
		PrevStopCode:  "UN",
		NextStopCode:  "EX",
		AtStationCode: &at,
		DelaySeconds:  125,
	})

	if train.PositionLabel != "at UN" {
		t.Fatalf("expected at-station label, got %q", train.PositionLabel)
	}
	if train.DelayMinutes != 2 {
		t.Fatalf("expected rounded minute truncation of 2, got %d", train.DelayMinutes)
	}
}

func TestNormalizeTrainPositionLabelsBetweenStations(t *testing.T) {
	train := normalizeTrain(metrolinx.TrainTrip{
		TripNumber:   "1235",
		LineCode:     "LW",
		Display:      "LW - Union Station",
		PrevStopCode: "OAKY",
		NextStopCode: "CL",
		IsInMotion:   true,
	})

	if train.PositionLabel != "between OAKY and CL" {
		t.Fatalf("expected between-stations label, got %q", train.PositionLabel)
	}
	if !train.InMotion {
		t.Fatal("expected in-motion flag to be preserved")
	}
}

func TestNormalizeAlertCompactsRefs(t *testing.T) {
	alert := normalizeAlert(metrolinx.AlertMessage{
		Code:           "M1",
		Status:         "INIT",
		SubjectEnglish: "Elevator out of service",
		Category:       "Amenity",
		SubCategory:    "Elevator",
		Lines:          []metrolinx.CodeRef{{Code: "LW"}, {Code: ""}},
		Stops:          []metrolinx.StopRef{{Code: "UN"}},
	})

	if len(alert.Lines) != 1 || alert.Lines[0] != "LW" {
		t.Fatalf("unexpected lines: %#v", alert.Lines)
	}
	if alert.Category != "Amenity / Elevator" {
		t.Fatalf("unexpected category: %q", alert.Category)
	}
}
