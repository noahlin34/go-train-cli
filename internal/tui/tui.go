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
		fmt.Fprintf(&b, "%s\n", renderTrack(train, m.topology[topologyKey(train.Line, train.Direction)], m.blink))
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render(train.PositionLabel+" | updated "+updatedAgo(train.LastUpdated, time.Now())))
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

func updatedAgo(value string, now time.Time) string {
	updatedAt, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(value), now.Location())
	if err != nil {
		return "just now"
	}
	age := now.Sub(updatedAt)
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
