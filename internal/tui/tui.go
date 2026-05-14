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
	err       error
	blink     bool
	loading   bool
	updatedAt time.Time
}

type trainsMsg struct {
	trains []transit.TrainPosition
	err    error
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
	case trainsMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.trains = msg.trains
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
	if len(m.trains) == 0 {
		fmt.Fprintf(&b, "%s\n", mutedStyle.Render("No live trains found for this view."))
		return b.String()
	}
	for _, train := range m.trains {
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", titleStyle.Render(train.Line), train.TripNumber, train.Display, delayText(train.DelaySeconds))
		fmt.Fprintf(&b, "%s\n", renderTrack(train, m.blink))
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
		sort.Slice(trains, func(i, j int) bool {
			if trains[i].Direction == trains[j].Direction {
				return trains[i].TripNumber < trains[j].TripNumber
			}
			return trains[i].Direction < trains[j].Direction
		})
		return trainsMsg{trains: trains, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func renderTrack(train transit.TrainPosition, blink bool) string {
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

func delayText(seconds int) string {
	if seconds > 60 {
		return alertStyle.Render(fmt.Sprintf("+%dm", seconds/60))
	}
	if seconds < -60 {
		return mutedStyle.Render(fmt.Sprintf("%dm", seconds/60))
	}
	return lineStyle.Render("on time")
}
