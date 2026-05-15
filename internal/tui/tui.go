package tui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noah/go-train-cli/pkg/transit"
)

type model struct {
	service   *transit.Service
	line      string
	refresh   time.Duration
	trains    []transit.TrainPosition
	alerts    []transit.Alert
	topology  map[string][]transit.LineStop
	trips     map[string][]transit.TripStop
	viewport  viewport.Model
	ready     bool
	err       error
	blink     bool
	loading   bool
	updatedAt time.Time
}

type snapshotMsg struct {
	trains   []transit.TrainPosition
	alerts   []transit.Alert
	topology map[string][]transit.LineStop
	trips    map[string][]transit.TripStop
	err      error
}

type tickMsg time.Time

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	dotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	lineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func Run(ctx context.Context, service *transit.Service, line string, refreshSeconds int, out io.Writer) error {
	if refreshSeconds < 5 {
		refreshSeconds = 5
	}
	m := model{
		service: service,
		line:    strings.ToUpper(strings.TrimSpace(line)),
		refresh: time.Duration(refreshSeconds) * time.Second,
		loading: true,
	}
	_, err := tea.NewProgram(m, tea.WithOutput(out), tea.WithContext(ctx), tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetch(), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		height := msg.Height - headerHeight
		if height < 1 {
			height = 1
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, height)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = height
		}
		m.viewport.SetContent(m.bodyView())
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.fetch()
		case "l":
			m.line = ""
			m.loading = true
			return m, m.fetch()
		case "1":
			m.line = "LW"
			m.loading = true
			return m, m.fetch()
		case "2":
			m.line = "LE"
			m.loading = true
			return m, m.fetch()
		case "3":
			m.line = "KI"
			m.loading = true
			return m, m.fetch()
		case "4":
			m.line = "BR"
			m.loading = true
			return m, m.fetch()
		case "5":
			m.line = "ST"
			m.loading = true
			return m, m.fetch()
		case "6":
			m.line = "RH"
			m.loading = true
			return m, m.fetch()
		case "7":
			m.line = "MI"
			m.loading = true
			return m, m.fetch()
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case snapshotMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.trains = msg.trains
			m.alerts = msg.alerts
			m.topology = msg.topology
			m.trips = msg.trips
			m.updatedAt = time.Now()
		}
		m.viewport.SetContent(m.bodyView())
		return m, nil
	case tickMsg:
		m.blink = !m.blink
		m.viewport.SetContent(m.bodyView())
		cmds := []tea.Cmd{tick()}
		if time.Since(m.updatedAt) >= m.refresh {
			m.loading = true
			cmds = append(cmds, m.fetch())
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return m.headerView() + "\n\n" + m.bodyView()
	}
	return m.headerView() + "\n" + m.viewport.View()
}

func (m model) headerView() string {
	var b strings.Builder
	lineName := m.line
	if lineName == "" {
		lineName = "ALL"
	}
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("gotrain live"))
	fmt.Fprintf(&b, "%s  refresh %s  scroll ↑/↓ pgup/pgdn home/end  keys: r refresh, 1 LW, 2 LE, 3 KI, 4 BR, 5 ST, 6 RH, 7 MI, l all, q quit",
		mutedStyle.Render("line "+lineName), m.refresh)
	return b.String()
}

func (m model) bodyView() string {
	var b strings.Builder
	if m.err != nil {
		fmt.Fprintf(&b, "%s\n\n", alertStyle.Render(m.err.Error()))
	}
	if m.loading {
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render("refreshing live train positions..."))
	}
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("Active alerts"))
	if len(m.alerts) == 0 {
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render("No active alerts for this view."))
	} else {
		for i, alert := range m.alerts {
			if i >= 4 {
				fmt.Fprintf(&b, "%s\n", mutedStyle.Render(fmt.Sprintf("+%d more alerts", len(m.alerts)-i)))
				break
			}
			fmt.Fprintf(&b, "%s %s\n", alertStyle.Render(alert.Category), alert.Subject)
			fmt.Fprintf(&b, "%s\n", mutedStyle.Render(alertMeta(alert)))
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("Live trains"))
	if len(m.trains) == 0 {
		fmt.Fprintf(&b, "%s\n", mutedStyle.Render("No live trains found for this view."))
		return b.String()
	}
	for _, train := range m.trains {
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", titleStyle.Render(train.Line), train.TripNumber, train.Display, delayText(train.DelaySeconds))
		fmt.Fprintf(&b, "%s\n", m.renderTrainTrack(train))
		if status := m.trainTimingStatus(train); status != "" {
			fmt.Fprintf(&b, "%s\n", lineStyle.Render(status))
		}
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render(train.PositionLabel+" | refreshed "+refreshedAgo(m.updatedAt, time.Now())))
	}
	return b.String()
}

func (m model) renderTrainTrack(train transit.TrainPosition) string {
	if tripStops := m.trips[train.TripNumber]; len(tripStops) > 0 {
		return renderTripTrack(train, tripStops, m.blink, m.updatedAt)
	}
	return renderTrack(train, m.topology[topologyKey(train.Line, train.Direction)], m.blink)
}

func (m model) trainTimingStatus(train transit.TrainPosition) string {
	return trainTimingStatus(train, m.trips[train.TripNumber])
}

func (m model) fetch() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		snap, err := m.service.Trains(ctx, m.line)
		trains := snap.Data
		alertSnap, alertErr := m.service.Alerts(ctx, m.line)
		if err == nil && alertErr != nil {
			err = alertErr
		}
		topology := make(map[string][]transit.LineStop)
		tripStops := make(map[string][]transit.TripStop)
		sort.Slice(trains, func(i, j int) bool {
			if trains[i].Direction == trains[j].Direction {
				return trains[i].TripNumber < trains[j].TripNumber
			}
			return trains[i].Direction < trains[j].Direction
		})
		for _, train := range trains {
			if train.Line == "" || train.Direction == "" {
				continue
			}
			key := topologyKey(train.Line, train.Direction)
			if _, ok := topology[key]; ok {
				continue
			}
			lineSnap, lineErr := m.service.LineStops(ctx, train.Line, train.Direction, time.Now())
			if lineErr != nil {
				continue
			}
			topology[key] = lineSnap.Data
		}
		for _, train := range trains {
			if train.TripNumber == "" {
				continue
			}
			tripSnap, tripErr := m.service.TripStops(ctx, train.TripNumber, time.Now())
			if tripErr != nil {
				continue
			}
			tripStops[train.TripNumber] = tripSnap.Data
		}
		return snapshotMsg{trains: trains, alerts: alertSnap.Data, topology: topology, trips: tripStops, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func renderTrack(train transit.TrainPosition, stops []transit.LineStop, blink bool) string {
	if len(stops) > 0 {
		return renderFullTrack(train, stops, blink)
	}
	left := train.PreviousStop
	right := train.NextStop
	if train.AtStation != nil && strings.TrimSpace(*train.AtStation) != "" {
		left = *train.AtStation
		right = train.NextStop
	}
	dot := "o"
	if blink {
		dot = "●"
	}
	if train.InMotion {
		return fmt.Sprintf("%s %s %s %s %s",
			mutedStyle.Render(left),
			lineStyle.Render("━━━━━━"),
			dotStyle.Render(dot),
			lineStyle.Render("━━━━━━"),
			mutedStyle.Render(right),
		)
	}
	return fmt.Sprintf("%s %s %s %s %s",
		mutedStyle.Render(left),
		lineStyle.Render("━━"),
		dotStyle.Render(dot),
		lineStyle.Render("━━━━━━━━━━"),
		mutedStyle.Render(right),
	)
}

func renderTripTrack(train transit.TrainPosition, stops []transit.TripStop, blink bool, now time.Time) string {
	lineStops := tripStopsAsLineStops(stops)
	if len(lineStops) == 0 {
		return renderTrack(train, nil, blink)
	}
	if trainHasPublicPosition(train, lineStops) {
		return renderFullTrack(train, lineStops, blink)
	}
	left, right, ok := tripSegmentByTime(stops, now)
	if !ok {
		return renderFullTrack(train, lineStops, blink)
	}
	mapped := train
	mapped.PreviousStop = left
	mapped.NextStop = right
	mapped.AtStation = nil
	return renderFullTrack(mapped, lineStops, blink)
}

func trainHasPublicPosition(train transit.TrainPosition, stops []transit.LineStop) bool {
	if train.AtStation != nil && stopIndex(stops, *train.AtStation) >= 0 {
		return true
	}
	return stopIndex(stops, train.PreviousStop) >= 0 || stopIndex(stops, train.NextStop) >= 0
}

func tripStopsAsLineStops(stops []transit.TripStop) []transit.LineStop {
	if len(stops) == 0 {
		return nil
	}
	out := make([]transit.LineStop, 0, len(stops))
	for _, stop := range stops {
		if strings.TrimSpace(stop.Code) == "" {
			continue
		}
		out = append(out, transit.LineStop{
			Code:    stop.Code,
			Order:   stop.Order,
			IsMajor: true,
		})
	}
	return out
}

func tripSegmentByTime(stops []transit.TripStop, now time.Time) (string, string, bool) {
	if len(stops) == 0 || now.IsZero() {
		return "", "", false
	}
	bestPrevious := -1
	for i, stop := range stops {
		stopTime, ok := publicStopTime(stop, now)
		if !ok {
			continue
		}
		if !stopTime.After(now) {
			bestPrevious = i
			continue
		}
		if bestPrevious >= 0 {
			return stops[bestPrevious].Code, stop.Code, true
		}
		if i > 0 {
			return stops[i-1].Code, stop.Code, true
		}
		if len(stops) > 1 {
			return stops[0].Code, stops[1].Code, true
		}
		return stop.Code, stop.Code, true
	}
	if bestPrevious >= 0 && bestPrevious < len(stops)-1 {
		return stops[bestPrevious].Code, stops[bestPrevious+1].Code, true
	}
	if len(stops) >= 2 {
		return stops[len(stops)-2].Code, stops[len(stops)-1].Code, true
	}
	return stops[0].Code, stops[0].Code, true
}

func publicStopTime(stop transit.TripStop, now time.Time) (time.Time, bool) {
	for _, value := range []string{
		stop.DepartureComputed,
		stop.ArrivalComputed,
		stop.DepartureScheduled,
		stop.ArrivalScheduled,
	} {
		if parsed, ok := parseClockTime(value, now); ok {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func trainTimingStatus(train transit.TrainPosition, stops []transit.TripStop) string {
	if len(stops) == 0 {
		return ""
	}
	if train.AtStation != nil && strings.TrimSpace(*train.AtStation) != "" {
		stop, ok := tripStopByCode(stops, *train.AtStation)
		if !ok {
			return ""
		}
		if computed := strings.TrimSpace(stop.DepartureComputed); computed != "" {
			return fmt.Sprintf("departs %s at %s", stop.Code, computed)
		}
		if scheduled := strings.TrimSpace(stop.DepartureScheduled); scheduled != "" {
			return fmt.Sprintf("scheduled to depart %s at %s", stop.Code, scheduled)
		}
		return ""
	}
	stop, ok := tripStopByCode(stops, train.NextStop)
	if !ok {
		return ""
	}
	if computed := strings.TrimSpace(stop.ArrivalComputed); computed != "" {
		return fmt.Sprintf("arrives %s at %s", stop.Code, computed)
	}
	if scheduled := strings.TrimSpace(stop.ArrivalScheduled); scheduled != "" {
		return fmt.Sprintf("scheduled to arrive %s at %s", stop.Code, scheduled)
	}
	return ""
}

func tripStopByCode(stops []transit.TripStop, code string) (transit.TripStop, bool) {
	code = strings.TrimSpace(code)
	for _, stop := range stops {
		if strings.EqualFold(stop.Code, code) {
			return stop, true
		}
	}
	return transit.TripStop{}, false
}

func parseClockTime(value string, now time.Time) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, false
	}
	candidate := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if candidate.Sub(now) > 12*time.Hour {
		candidate = candidate.Add(-24 * time.Hour)
	}
	if now.Sub(candidate) > 12*time.Hour {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate, true
}

func renderFullTrack(train transit.TrainPosition, stops []transit.LineStop, blink bool) string {
	dot := "o"
	if blink {
		dot = "●"
	}
	stops = extendStopsForTrip(stops, train)
	atCode := ""
	if train.AtStation != nil {
		atCode = strings.TrimSpace(*train.AtStation)
	}
	prevIndex := stopIndex(stops, train.PreviousStop)
	nextIndex := stopIndex(stops, train.NextStop)
	atIndex := stopIndex(stops, atCode)
	if prevIndex == -1 && nextIndex == -1 && atIndex == -1 {
		prevIndex, nextIndex = inferredPublicSegment(stops, train)
	}
	var b strings.Builder
	for i, stop := range stops {
		if i > 0 {
			fmt.Fprint(&b, lineStyle.Render("━━"))
		}
		if atIndex == i {
			fmt.Fprint(&b, dotStyle.Render(dot))
		} else {
			fmt.Fprint(&b, mutedStyle.Render(stop.Code))
		}
		if atIndex == -1 && shouldInsertBetweenDot(i, prevIndex, nextIndex) {
			fmt.Fprint(&b, lineStyle.Render("━━"))
			fmt.Fprint(&b, dotStyle.Render(dot))
		}
	}
	return b.String()
}

func extendStopsForTrip(stops []transit.LineStop, train transit.TrainPosition) []transit.LineStop {
	extended := append([]transit.LineStop(nil), stops...)
	if train.FirstStop != "" && stopIndex(extended, train.FirstStop) == -1 {
		extended = append([]transit.LineStop{{Code: train.FirstStop, Order: 0, IsMajor: true}}, extended...)
	}
	if train.LastStop != "" && stopIndex(extended, train.LastStop) == -1 {
		extended = append(extended, transit.LineStop{Code: train.LastStop, Order: len(extended) + 1, IsMajor: true})
	}
	return extended
}

func inferredPublicSegment(stops []transit.LineStop, train transit.TrainPosition) (int, int) {
	left, right, ok := telemetrySegment(train)
	if !ok {
		return -1, -1
	}
	return stopIndex(stops, left), stopIndex(stops, right)
}

func telemetrySegment(train transit.TrainPosition) (string, string, bool) {
	line := strings.ToUpper(strings.TrimSpace(train.Line))
	direction := strings.ToUpper(strings.TrimSpace(train.Direction))
	prev := strings.ToUpper(strings.TrimSpace(train.PreviousStop))
	next := strings.ToUpper(strings.TrimSpace(train.NextStop))
	at := ""
	if train.AtStation != nil {
		at = strings.ToUpper(strings.TrimSpace(*train.AtStation))
	}

	switch {
	case line == "ST" && direction == "N" && (hasTelemetry(prev, next, at, "DA") || hasTelemetry(prev, next, at, "SCAJ")):
		return "UN", "KE", true
	case line == "ST" && direction == "S" && (hasTelemetry(prev, next, at, "DA") || hasTelemetry(prev, next, at, "SCAJ")):
		return "KE", "UN", true
	case line == "LW" && direction == "W" && (hasTelemetry(prev, next, at, "BAYV") || hasTelemetry(prev, next, at, "HAMJ")):
		return "AL", "WR", true
	case line == "LW" && direction == "E" && (hasTelemetry(prev, next, at, "BAYV") || hasTelemetry(prev, next, at, "HAMJ")):
		return "WR", "AL", true
	default:
		return "", "", false
	}
}

func hasTelemetry(prev, next, at, code string) bool {
	return prev == code || next == code || at == code
}

func shouldInsertBetweenDot(i, prevIndex, nextIndex int) bool {
	if prevIndex >= 0 && nextIndex >= 0 {
		if prevIndex < nextIndex {
			return i == prevIndex
		}
		return i == nextIndex
	}
	if prevIndex >= 0 {
		return i == prevIndex
	}
	if nextIndex >= 0 {
		return i == nextIndex-1
	}
	return false
}

func stopIndex(stops []transit.LineStop, code string) int {
	code = strings.TrimSpace(code)
	for i, stop := range stops {
		if strings.EqualFold(stop.Code, code) {
			return i
		}
	}
	return -1
}

func delayText(seconds int) string {
	switch {
	case seconds < 0:
		return alertStyle.Render(delayPhrase("delayed by", delayMinutes(seconds)))
	case seconds > 0:
		return mutedStyle.Render(delayPhrase("early by", delayMinutes(seconds)))
	default:
		return lineStyle.Render("on time")
	}
}

func delayPhrase(prefix string, minutes int) string {
	unit := "minutes"
	if minutes == 1 {
		unit = "minute"
	}
	return fmt.Sprintf("%s %d %s", prefix, minutes, unit)
}

func delayMinutes(seconds int) int {
	if seconds < 0 {
		seconds = -seconds
	}
	minutes := seconds / 60
	if seconds%60 != 0 {
		minutes++
	}
	return minutes
}

func alertMeta(alert transit.Alert) string {
	var parts []string
	if len(alert.Lines) > 0 {
		parts = append(parts, "lines "+strings.Join(alert.Lines, ","))
	}
	if len(alert.Stops) > 0 {
		parts = append(parts, "stops "+strings.Join(alert.Stops, ","))
	}
	if alert.PostedAt != "" {
		parts = append(parts, "posted "+alert.PostedAt)
	}
	return strings.Join(parts, " | ")
}

func topologyKey(line, direction string) string {
	return strings.ToUpper(strings.TrimSpace(line)) + ":" + strings.ToUpper(strings.TrimSpace(direction))
}

func refreshedAgo(refreshedAt time.Time, now time.Time) string {
	if refreshedAt.IsZero() {
		return "just now"
	}
	age := now.Sub(refreshedAt)
	if age < 0 {
		age = 0
	}
	if age < time.Minute {
		return fmt.Sprintf("%ds ago", int(age.Seconds()))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(age.Hours()))
}
