package transit

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/noah/go-train-cli/pkg/metrolinx"
)

type Service struct {
	client *metrolinx.Client
}

func NewService(client *metrolinx.Client) *Service {
	return &Service{client: client}
}

type Snapshot[T any] struct {
	GeneratedAt string `json:"generated_at"`
	SourceTime  string `json:"source_time"`
	Data        T      `json:"data"`
}

type Station struct {
	Code string `json:"code"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type TrainPosition struct {
	TripNumber    string   `json:"trip_number"`
	Line          string   `json:"line"`
	Direction     string   `json:"direction"`
	Display       string   `json:"display"`
	Cars          string   `json:"cars,omitempty"`
	FirstStop     string   `json:"first_stop"`
	LastStop      string   `json:"last_stop"`
	PreviousStop  string   `json:"previous_stop"`
	NextStop      string   `json:"next_stop"`
	AtStation     *string  `json:"at_station,omitempty"`
	InMotion      bool     `json:"in_motion"`
	DelaySeconds  int      `json:"delay_seconds"`
	DelayMinutes  int      `json:"delay_minutes"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Course        *float64 `json:"course,omitempty"`
	LastUpdated   string   `json:"last_updated"`
	TrackingKind  string   `json:"tracking_kind"`
	PositionLabel string   `json:"position_label"`
}

type Alert struct {
	Code     string   `json:"code"`
	Status   string   `json:"status"`
	PostedAt string   `json:"posted_at"`
	Subject  string   `json:"subject"`
	Body     string   `json:"body"`
	Category string   `json:"category"`
	Lines    []string `json:"lines"`
	Stops    []string `json:"stops"`
	TripIDs  []string `json:"trip_ids"`
}

type LineStop struct {
	Code    string `json:"code"`
	Order   int    `json:"order"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	IsMajor bool   `json:"is_major"`
}

type TripStop struct {
	Code               string `json:"code"`
	Order              int    `json:"order"`
	Status             string `json:"status"`
	ArrivalScheduled   string `json:"arrival_scheduled,omitempty"`
	ArrivalComputed    string `json:"arrival_computed,omitempty"`
	DepartureScheduled string `json:"departure_scheduled,omitempty"`
	DepartureComputed  string `json:"departure_computed,omitempty"`
}

func (s *Service) Stations(ctx context.Context, query string, trainOnly bool) (Snapshot[[]Station], error) {
	stops, meta, err := s.client.Stops(ctx)
	if err != nil {
		return Snapshot[[]Station]{}, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var stations []Station
	for _, stop := range stops {
		if trainOnly && !strings.Contains(strings.ToLower(stop.LocationType), "train") {
			continue
		}
		item := Station{
			Code: stop.LocationCode,
			ID:   stop.PublicStopID,
			Name: stop.LocationName,
			Type: stop.LocationType,
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Code+" "+item.Name+" "+item.Type), query) {
			continue
		}
		stations = append(stations, item)
	}
	sort.Slice(stations, func(i, j int) bool {
		return stations[i].Name < stations[j].Name
	})
	return Snapshot[[]Station]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        stations,
	}, nil
}

func (s *Service) Trains(ctx context.Context, line string) (Snapshot[[]TrainPosition], error) {
	trips, meta, err := s.client.TrainTrips(ctx)
	if err != nil {
		return Snapshot[[]TrainPosition]{}, err
	}
	line = strings.ToUpper(strings.TrimSpace(line))
	var positions []TrainPosition
	for _, trip := range trips {
		if line != "" && strings.ToUpper(strings.TrimSpace(trip.LineCode)) != line {
			continue
		}
		positions = append(positions, normalizeTrain(trip))
	}
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].Line == positions[j].Line {
			return positions[i].TripNumber < positions[j].TripNumber
		}
		return positions[i].Line < positions[j].Line
	})
	return Snapshot[[]TrainPosition]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        positions,
	}, nil
}

func (s *Service) Train(ctx context.Context, tripNumber string) (Snapshot[*TrainPosition], error) {
	snap, err := s.Trains(ctx, "")
	if err != nil {
		return Snapshot[*TrainPosition]{}, err
	}
	for _, train := range snap.Data {
		if train.TripNumber == tripNumber {
			t := train
			return Snapshot[*TrainPosition]{
				GeneratedAt: snap.GeneratedAt,
				SourceTime:  snap.SourceTime,
				Data:        &t,
			}, nil
		}
	}
	return Snapshot[*TrainPosition]{
		GeneratedAt: snap.GeneratedAt,
		SourceTime:  snap.SourceTime,
		Data:        nil,
	}, nil
}

func (s *Service) Alerts(ctx context.Context, line string) (Snapshot[[]Alert], error) {
	raw, meta, err := s.client.ServiceAlerts(ctx)
	if err != nil {
		return Snapshot[[]Alert]{}, err
	}
	line = strings.ToUpper(strings.TrimSpace(line))
	var alerts []Alert
	for _, msg := range raw {
		alert := normalizeAlert(msg)
		if line != "" && !contains(alert.Lines, line) {
			continue
		}
		alerts = append(alerts, alert)
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].PostedAt > alerts[j].PostedAt
	})
	return Snapshot[[]Alert]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        alerts,
	}, nil
}

func (s *Service) LineStops(ctx context.Context, line, direction string, day time.Time) (Snapshot[[]LineStop], error) {
	line = strings.ToUpper(strings.TrimSpace(line))
	direction = strings.ToUpper(strings.TrimSpace(direction))
	if day.IsZero() {
		day = time.Now()
	}
	raw, meta, err := s.client.LineStops(ctx, day.Format("20060102"), line, direction)
	if err != nil {
		return Snapshot[[]LineStop]{}, err
	}
	stops := make([]LineStop, 0, len(raw))
	for _, stop := range raw {
		stops = append(stops, LineStop{
			Code:    strings.TrimSpace(stop.Code),
			Order:   stop.Order,
			Name:    stop.Name,
			Type:    stop.Type,
			IsMajor: stop.IsMajor,
		})
	}
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].Order < stops[j].Order
	})
	return Snapshot[[]LineStop]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        stops,
	}, nil
}

func (s *Service) TripStops(ctx context.Context, tripNumber string, day time.Time) (Snapshot[[]TripStop], error) {
	if day.IsZero() {
		day = time.Now()
	}
	trip, meta, err := s.client.ScheduledTrip(ctx, day.Format("20060102"), tripNumber)
	if err != nil {
		return Snapshot[[]TripStop]{}, err
	}
	if trip == nil {
		return Snapshot[[]TripStop]{
			GeneratedAt: nowISO(),
			SourceTime:  meta.TimeStamp,
			Data:        []TripStop{},
		}, nil
	}
	stops := make([]TripStop, 0, len(trip.Stops))
	for i, stop := range trip.Stops {
		stops = append(stops, TripStop{
			Code:               strings.TrimSpace(stop.Code),
			Order:              i + 1,
			Status:             strings.TrimSpace(stop.Status),
			ArrivalScheduled:   strings.TrimSpace(stop.ArrivalTime.Scheduled),
			ArrivalComputed:    strings.TrimSpace(stop.ArrivalTime.Computed),
			DepartureScheduled: strings.TrimSpace(stop.DepartureTime.Scheduled),
			DepartureComputed:  strings.TrimSpace(stop.DepartureTime.Computed),
		})
	}
	return Snapshot[[]TripStop]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        stops,
	}, nil
}

func (s *Service) Departures(ctx context.Context, stopCode string) (Snapshot[json.RawMessage], error) {
	raw, meta, err := s.client.NextService(ctx, stopCode)
	if err != nil {
		return Snapshot[json.RawMessage]{}, err
	}
	return Snapshot[json.RawMessage]{
		GeneratedAt: nowISO(),
		SourceTime:  meta.TimeStamp,
		Data:        raw,
	}, nil
}

func normalizeTrain(trip metrolinx.TrainTrip) TrainPosition {
	line := strings.TrimSpace(trip.LineCode)
	at := trip.AtStationCode
	trackingKind := "estimated_between_stations"
	position := "between " + cleanStop(trip.PrevStopCode) + " and " + cleanStop(trip.NextStopCode)
	if at != nil && strings.TrimSpace(*at) != "" {
		trackingKind = "observed_at_station"
		position = "at " + cleanStop(*at)
	}
	delayMinutes := int(float64(trip.DelaySeconds) / 60.0)
	return TrainPosition{
		TripNumber:    trip.TripNumber,
		Line:          line,
		Direction:     strings.TrimSpace(trip.VariantDir),
		Display:       strings.TrimSpace(trip.Display),
		Cars:          strings.TrimSpace(trip.Cars),
		FirstStop:     cleanStop(trip.FirstStopCode),
		LastStop:      cleanStop(trip.LastStopCode),
		PreviousStop:  cleanStop(trip.PrevStopCode),
		NextStop:      cleanStop(trip.NextStopCode),
		AtStation:     at,
		InMotion:      trip.IsInMotion,
		DelaySeconds:  trip.DelaySeconds,
		DelayMinutes:  delayMinutes,
		Latitude:      trip.Latitude,
		Longitude:     trip.Longitude,
		Course:        trip.Course,
		LastUpdated:   trip.ModifiedDate,
		TrackingKind:  trackingKind,
		PositionLabel: position,
	}
}

func normalizeAlert(msg metrolinx.AlertMessage) Alert {
	var lines []string
	for _, line := range msg.Lines {
		lines = append(lines, strings.TrimSpace(line.Code))
	}
	var stops []string
	for _, stop := range msg.Stops {
		stops = append(stops, strings.TrimSpace(stop.Code))
	}
	var trips []string
	for _, trip := range msg.Trips {
		trips = append(trips, strings.TrimSpace(trip.TripNumber))
	}
	return Alert{
		Code:     msg.Code,
		Status:   msg.Status,
		PostedAt: msg.PostedDateTime,
		Subject:  msg.SubjectEnglish,
		Body:     msg.BodyEnglish,
		Category: strings.TrimSpace(strings.Join([]string{msg.Category, msg.SubCategory}, " / ")),
		Lines:    nonNil(compact(lines)),
		Stops:    nonNil(compact(stops)),
		TripIDs:  nonNil(compact(trips)),
	}
}

func cleanStop(code string) string {
	return strings.TrimSpace(code)
}

func compact(items []string) []string {
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
