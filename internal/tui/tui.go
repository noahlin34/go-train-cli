package tui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

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
	err       error
	blink     bool
	loading   bool
	updatedAt time.Time
}

type snapshotMsg struct {
	trains   []transit.TrainPosition
	alerts   []transit.Alert
	topology map[string][]transit.LineStop
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
	case snapshotMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.trains = msg.trains
			m.alerts = msg.alerts
			m.topology = msg.topology
			m.updatedAt = time.Now()
		}
		return m, nil
	case tickMsg:
		m.blink = !m.blink
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
	var b strings.Builder
	lineName := m.line
	if lineName == "" {
		lineName = "ALL"
	}
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("gotrain live"))
	fmt.Fprintf(&b, "%s  refresh %s  keys: r refresh, 1 LW, 2 LE, 3 KI, 4 BR, 5 ST, 6 RH, 7 MI, l all, q quit\n\n",
		mutedStyle.Render("line "+lineName), m.refresh)
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
		fmt.Fprintf(&b, "%s\n", renderTrack(train, m.topology[topologyKey(train.Line, train.Direction)], m.blink))
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render(train.PositionLabel+" | updated "+train.LastUpdated))
	}
	return b.String()
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
		return snapshotMsg{trains: trains, alerts: alertSnap.Data, topology: topology, err: err}
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

func renderFullTrack(train transit.TrainPosition, stops []transit.LineStop, blink bool) string {
	dot := "o"
	if blink {
		dot = "●"
	}
	atCode := ""
	if train.AtStation != nil {
		atCode = strings.TrimSpace(*train.AtStation)
	}
	prevIndex := stopIndex(stops, train.PreviousStop)
	nextIndex := stopIndex(stops, train.NextStop)
	atIndex := stopIndex(stops, atCode)
	var b strings.Builder
	for i, stop := range stops {
		if i > 0 {
			fmt.Fprint(&b, lineStyle.Render("━"))
		}
		if atIndex == i {
			fmt.Fprint(&b, dotStyle.Render(dot))
		} else {
			fmt.Fprint(&b, mutedStyle.Render(stop.Code))
		}
		if atIndex == -1 && shouldInsertBetweenDot(i, prevIndex, nextIndex) {
			fmt.Fprint(&b, lineStyle.Render("━"))
			fmt.Fprint(&b, dotStyle.Render(dot))
		}
	}
	return b.String()
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
	if seconds > 60 {
		return alertStyle.Render(fmt.Sprintf("+%dm", seconds/60))
	}
	if seconds < -60 {
		return mutedStyle.Render(fmt.Sprintf("%dm", seconds/60))
	}
	return lineStyle.Render("on time")
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
