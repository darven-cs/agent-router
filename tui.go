package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Style definitions
var (
	stylePrimary   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	styleSecondary = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleSuccess   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleHeader    = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	styleFormActive = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	styleWarning   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// model holds all TUI state
type model struct {
	serviceName  string
	version      string
	port         int
	startTime    time.Time
	upstreams    []*Upstream
	logs         []RequestLog
	requestCount int64
	successCount int64

	// Navigation state
	selectedIndex int

	// Form state
	formMode  string    // "", "add", "edit"
	formData  Upstream  // Form working copy
	formField int       // Current field index (0-5)

	// Confirmation mode
	confirmMode bool
	confirmType string // "delete" or "shutdown"
}

// NewModel creates a new TUI model
func NewModel(serviceName, version string, port int, upstreams []*Upstream) model {
	return model{
		serviceName:  serviceName,
		version:      version,
		port:         port,
		startTime:    time.Now(),
		upstreams:    upstreams,
		logs:         make([]RequestLog, 0, 50),
		selectedIndex: 0,
		formMode:      "",
		confirmMode:   false,
	}
}

// Init initializes the TUI model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles TUI messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmMode {
			return m.handleConfirm(msg)
		}
		if m.formMode != "" {
			return m.handleFormInput(msg)
		}
		// Navigation mode
		switch msg.String() {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down":
			if m.selectedIndex < len(m.upstreams)-1 {
				m.selectedIndex++
			}
		case "a":
			m.formMode = "add"
			m.formData = Upstream{Enabled: true, Timeout: 30 * time.Second, AuthType: "bearer"}
			m.formField = 0
		case "e", "enter":
			if len(m.upstreams) > 0 {
				m.formMode = "edit"
				m.formData = *m.upstreams[m.selectedIndex]
				m.formField = 0
			}
		case "d":
			if len(m.upstreams) > 0 {
				m.confirmMode = true
				m.confirmType = "delete"
			}
		case "q", "ctrl+c":
			m.confirmMode = true
			m.confirmType = "shutdown"
		case "esc":
			// No form to cancel in navigation mode
		}
	case RequestLog:
		m.logs = append(m.logs, msg)
		if len(m.logs) > 50 {
			m.logs = m.logs[1:]
		}
		m.requestCount++
		if msg.StatusCode >= 200 && msg.StatusCode < 400 {
			m.successCount++
		}
	}
	return m, nil
}

// handleConfirm processes confirmation dialog input
func (m model) handleConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// handleFormInput processes form input
func (m model) handleFormInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the TUI
func (m model) View() string {
	var s string

	// Header
	s += styleHeader.Render("╭─────────────────────────────────────────────────╮\n")
	s += styleHeader.Render(fmt.Sprintf("│ %s v%s                              │\n", m.serviceName, m.version))
	s += styleHeader.Render(fmt.Sprintf("│ Port: %d  |  Uptime: %s           │\n", m.port, time.Since(m.startTime).String()))
	s += styleHeader.Render("╰─────────────────────────────────────────────────╯\n\n")

	// Upstream list
	s += styleBold.Render("Upstreams:\n")
	for _, us := range m.upstreams {
		status := styleSuccess.Render("● enabled")
		if !us.Enabled {
			status = styleError.Render("○ disabled")
		}
		s += fmt.Sprintf("  %s %s [%s]\n", us.Name, status, us.AuthType)
	}
	s += "\n"

	// Request log
	s += styleBold.Render("Request Log:\n")
	if len(m.logs) == 0 {
		s += styleSecondary.Render("  (no requests yet)\n")
	} else {
		for _, log := range m.logs {
			statusStr := fmt.Sprintf("%d", log.StatusCode)
			if log.StatusCode == 0 {
				statusStr = "ERR"
			}
			s += fmt.Sprintf("  %s | %4dms | %s | %s\n",
				log.Timestamp.Format("15:04:05"),
				log.LatencyMs,
				log.UpstreamName,
				statusStr)
		}
	}
	s += "\n"

	// Stats
	s += styleBold.Render("Statistics:\n")
	total := m.requestCount
	success := m.successCount
	rate := float64(0)
	if total > 0 {
		rate = float64(success) / float64(total) * 100
	}
	s += fmt.Sprintf("  Total: %d | Success: %d | Rate: %.1f%%\n", total, success, rate)

	return s
}
