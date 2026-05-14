package metrolinx

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Metadata struct {
	TimeStamp    string `json:"TimeStamp"`
	ErrorCode    string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

type Stop struct {
	LocationCode string `json:"LocationCode"`
	PublicStopID string `json:"PublicStopId"`
	LocationName string `json:"LocationName"`
	LocationType string `json:"LocationType"`
}

type TrainTrip struct {
	Cars          string   `json:"Cars"`
	TripNumber    string   `json:"TripNumber"`
	StartTime     string   `json:"StartTime"`
	EndTime       string   `json:"EndTime"`
	LineCode      string   `json:"LineCode"`
	RouteNumber   string   `json:"RouteNumber"`
	VariantDir    string   `json:"VariantDir"`
	Display       string   `json:"Display"`
	Latitude      *float64 `json:"Latitude"`
	Longitude     *float64 `json:"Longitude"`
	IsInMotion    bool     `json:"IsInMotion"`
	DelaySeconds  int      `json:"DelaySeconds"`
	Course        *float64 `json:"Course"`
	FirstStopCode string   `json:"FirstStopCode"`
	LastStopCode  string   `json:"LastStopCode"`
	PrevStopCode  string   `json:"PrevStopCode"`
	NextStopCode  string   `json:"NextStopCode"`
	AtStationCode *string  `json:"AtStationCode"`
	ModifiedDate  string   `json:"ModifiedDate"`
}

type LineStop struct {
	Code    string `json:"Code"`
	Order   int    `json:"Order"`
	Name    string `json:"Name"`
	Type    string `json:"Type"`
	IsMajor bool   `json:"IsMajor"`
}

type AlertMessage struct {
	Code           string    `json:"Code"`
	ParentCode     *string   `json:"ParentCode"`
	Status         string    `json:"Status"`
	PostedDateTime string    `json:"PostedDateTime"`
	SubjectEnglish string    `json:"SubjectEnglish"`
	BodyEnglish    string    `json:"BodyEnglish"`
	Category       string    `json:"Category"`
	SubCategory    string    `json:"SubCategory"`
	Lines          []CodeRef `json:"Lines"`
	Stops          []StopRef `json:"Stops"`
	Trips          []TripRef `json:"Trips"`
}

type CodeRef struct {
	Code string `json:"Code"`
}

type StopRef struct {
	Name *string `json:"Name"`
	Code string  `json:"Code"`
}

type TripRef struct {
	TripNumber string `json:"TripNumber"`
}

type NextServiceLine struct {
	LineCode string `json:"LineCode"`
	LineName string `json:"LineName"`
	Service  string `json:"Service"`
}

type RawMessage = json.RawMessage

func oneOrMany[T any](b []byte) ([]T, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil, nil
	}
	if b[0] == '[' {
		var out []T
		return out, json.Unmarshal(b, &out)
	}
	var one T
	if err := json.Unmarshal(b, &one); err != nil {
		return nil, err
	}
	return []T{one}, nil
}

func requireOK(meta Metadata) error {
	if meta.ErrorCode != "" && meta.ErrorCode != "200" {
		return fmt.Errorf("metrolinx API error %s: %s", meta.ErrorCode, meta.ErrorMessage)
	}
	return nil
}
