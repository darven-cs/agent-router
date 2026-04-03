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

// Message types for upstream changes
type UpstreamAdded struct{ Upstream *Upstream }
type UpstreamUpdated struct{ Upstream *Upstream; OldName string }
type UpstreamDeleted struct{ Name string }

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

	// Callback for upstream changes
	OnUpstreamAdded   func(*Upstream)
	OnUpstreamUpdated func(*Upstream, string) // upstream, oldName
	OnUpstreamDeleted func(string)           // name
}

// UpstreamChange represents a change to upstream configuration
type UpstreamChange struct {
	Type     string    // "added", "updated", "deleted"
	Upstream *Upstream // For added/updated
	OldName  string    // For updated when name changed
	Name     string    // For deleted
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
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "y", "enter":
			if m.confirmType == "delete" {
				deletedName := m.upstreams[m.selectedIndex].Name
				m.upstreams = append(m.upstreams[:m.selectedIndex], m.upstreams[m.selectedIndex+1:]...)
				if m.selectedIndex >= len(m.upstreams) && m.selectedIndex > 0 {
					m.selectedIndex--
				}
				m.confirmMode = false
				m.confirmType = ""
				if m.OnUpstreamDeleted != nil {
					m.OnUpstreamDeleted(deletedName)
				}
				return m, nil
			} else if m.confirmType == "shutdown" {
				return m, tea.Quit
			}
		case "n", "esc":
			// Cancel
		}
		m.confirmMode = false
		m.confirmType = ""
	}
	return m, nil
}

// handleFormInput processes form input
func (m model) handleFormInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up":
			if m.formField > 0 {
				m.formField--
			}
		case "down", "tab":
			if m.formField < 5 {
				m.formField++
			}
		case "enter":
			if m.formField < 5 {
				m.formField++
			} else {
				return m.submitForm()
			}
		case "b", "backspace":
			m = m.handleFormBackspace(msg)
		case "esc":
			m.formMode = ""
			m.formData = Upstream{}
			m.formField = 0
		default:
			m = m.handleFormTextInput(msg)
		}
	}
	return m, nil
}

// submitForm validates and submits the form
func (m model) submitForm() (tea.Model, tea.Cmd) {
	// Validate: Name and URL required
	if m.formData.Name == "" || m.formData.URL == "" {
		m.formMode = ""
		m.formField = 0
		return m, nil
	}
	if m.formMode == "add" {
		m.formMode = ""
		m.formField = 0
		if m.OnUpstreamAdded != nil {
			m.OnUpstreamAdded(&m.formData)
		}
		return m, nil
	} else if m.formMode == "edit" {
		oldName := m.upstreams[m.selectedIndex].Name
		m.formMode = ""
		m.formField = 0
		if m.OnUpstreamUpdated != nil {
			m.OnUpstreamUpdated(&m.formData, oldName)
		}
		return m, nil
	}
	return m, nil
}

// handleFormTextInput handles text input for form fields
func (m model) handleFormTextInput(msg tea.KeyMsg) model {
	runes := msg.Runes
	if len(runes) == 0 {
		return m
	}
	switch m.formField {
	case 0: // Name
		m.formData.Name += string(runes[0])
	case 1: // URL
		m.formData.URL += string(runes[0])
	case 2: // APIKey
		m.formData.APIKey += string(runes[0])
	case 3: // AuthType (toggle)
		// Already handled by enter key
	case 4: // Timeout (number input)
		if len(runes) > 0 && runes[0] >= '0' && runes[0] <= '9' {
			m.formData.Timeout = (m.formData.Timeout / time.Second) * time.Second
			m.formData.Timeout += time.Duration(runes[0]-'0') * time.Second
		}
	}
	return m
}

// handleFormBackspace handles backspace for form fields
func (m model) handleFormBackspace(msg tea.KeyMsg) model {
	switch m.formField {
	case 0: // Name
		if len(m.formData.Name) > 0 {
			m.formData.Name = m.formData.Name[:len(m.formData.Name)-1]
		}
	case 1: // URL
		if len(m.formData.URL) > 0 {
			m.formData.URL = m.formData.URL[:len(m.formData.URL)-1]
		}
	case 2: // APIKey
		if len(m.formData.APIKey) > 0 {
			m.formData.APIKey = m.formData.APIKey[:len(m.formData.APIKey)-1]
		}
	}
	return m
}

// maskString hides API key for display
func maskString(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// View renders the TUI
func (m model) View() string {
	var s string

	// Header
	s += styleHeader.Render("╭─────────────────────────────────────────────────╮\n")
	s += styleHeader.Render(fmt.Sprintf("│ %s v%s                              │\n", m.serviceName, m.version))
	s += styleHeader.Render(fmt.Sprintf("│ Port: %d  |  Uptime: %s           │\n", m.port, time.Since(m.startTime).String()))
	s += styleHeader.Render("╰─────────────────────────────────────────────────╯\n\n")

	// Form mode
	if m.formMode != "" {
		modeStr := "Add Upstream"
		if m.formMode == "edit" {
			modeStr = "Edit Upstream"
		}
		s += styleHeader.Render(fmt.Sprintf("\n%s (ESC to cancel)\n", modeStr))
		s += styleSecondary.Render("───────────────────────────────────────\n")

		fields := []string{
			fmt.Sprintf("Name:     [%s]", m.formData.Name),
			fmt.Sprintf("URL:      [%s]", m.formData.URL),
			fmt.Sprintf("API Key:  [%s]", maskString(m.formData.APIKey)),
			fmt.Sprintf("Auth:     [%s] (enter to toggle)", m.formData.AuthType),
			fmt.Sprintf("Timeout:  [%d] seconds", int(m.formData.Timeout/time.Second)),
			fmt.Sprintf("Enabled:  [%v] (enter to toggle)", m.formData.Enabled),
		}
		for i, field := range fields {
			if i == m.formField {
				s += styleFormActive.Render("> " + field + "\n")
			} else {
				s += "  " + field + "\n"
			}
		}
		s += "\n"
		s += styleSecondary.Render("↑↓ Navigate | Enter Next/Submit | a-z Type | b Backspace\n")
		return s
	}

	// Confirmation mode
	if m.confirmMode {
		s += "\n"
		if m.confirmType == "delete" {
			upstreamName := ""
			if m.selectedIndex < len(m.upstreams) {
				upstreamName = m.upstreams[m.selectedIndex].Name
			}
			s += styleWarning.Render(fmt.Sprintf("╭─────────────────────────────────────╮\n"))
			s += styleWarning.Render(fmt.Sprintf("│  Delete '%s'? [y/n]            │\n", upstreamName))
			s += styleWarning.Render(fmt.Sprintf("╰─────────────────────────────────────╯\n"))
		} else if m.confirmType == "shutdown" {
			s += styleError.Render(fmt.Sprintf("╭─────────────────────────────────────╮\n"))
			s += styleError.Render(fmt.Sprintf("│  Shutdown? [y/n]                   │\n"))
			s += styleError.Render(fmt.Sprintf("╰─────────────────────────────────────╯\n"))
		}
		s += "\n"
	}

	// Upstream list
	s += styleBold.Render("Upstreams:\n")
	if len(m.upstreams) == 0 {
		s += styleSecondary.Render("  (no upstreams configured)\n")
	} else {
		for i, us := range m.upstreams {
			status := styleSuccess.Render("● enabled")
			if !us.Enabled {
				status = styleError.Render("○ disabled")
			}
			prefix := "  "
			if i == m.selectedIndex && !m.confirmMode {
				prefix = styleFormActive.Render("> ")
			}
			s += fmt.Sprintf("%s%s %s [%s]\n", prefix, us.Name, status, us.AuthType)
		}
	}
	s += "\n"
	s += styleSecondary.Render("↑↓ Navigate | a Add | e Edit | d Delete | q Quit\n\n")

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
