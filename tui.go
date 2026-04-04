package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha palette
var (
	// Base colors
	bgColor      = lipgloss.Color("235")  // base
	surfaceColor = lipgloss.Color("238")  // surface0
	surface1Color = lipgloss.Color("239") // surface1
	overlayColor = lipgloss.Color("243")  // overlay

	// Text colors
	textColor      = lipgloss.Color("222") // text
	subtextColor   = lipgloss.Color("245") // subtext0
	accentColor    = lipgloss.Color("250") // lavender/highlight

	// Accent colors
	mauveColor  = lipgloss.Color("205") // mauve (primary accent)
	tealColor   = lipgloss.Color("115") // teal (secondary accent)
	greenColor  = lipgloss.Color("114") // green (success)
	redColor    = lipgloss.Color("203") // red (error)
	yellowColor = lipgloss.Color("228") // yellow (warning)

	// Highlight for selected items
	selectedBg   = lipgloss.Color("24")  // blue highlight background
	selectedFg   = lipgloss.Color("230") // white text on highlight
)

// Style definitions with Catppuccin
var (
	styleBase = lipgloss.NewStyle().
			Background(bgColor).
			Foreground(textColor)

	styleSurface = lipgloss.NewStyle().
			Background(surfaceColor).
			Foreground(textColor)

	styleSurfaceBright = lipgloss.NewStyle().
				Background(surface1Color).
				Foreground(textColor)

	styleMauve = lipgloss.NewStyle().
			Foreground(mauveColor)

	styleTeal = lipgloss.NewStyle().
			Foreground(tealColor)

	styleGreen = lipgloss.NewStyle().
			Foreground(greenColor)

	styleRed = lipgloss.NewStyle().
			Foreground(redColor)

	styleYellow = lipgloss.NewStyle().
			Foreground(yellowColor)

	styleBold = lipgloss.NewStyle().Bold(true)

	styleDim = lipgloss.NewStyle().Foreground(subtextColor)

	// Border styles
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mauveColor)

	navBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mauveColor).
			Padding(0, 1)

	contentBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(tealColor)

	statusBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(overlayColor)

	// Selected row style
	selectedStyle = lipgloss.NewStyle().
			Background(selectedBg).
			Foreground(selectedFg).
			Bold(true)

	// Inverted style for status bar
	invertedStyle = lipgloss.NewStyle().
			Background(textColor).
			Foreground(bgColor)
)

// Message types for upstream changes
type UpstreamAdded struct{ Upstream *Upstream }
type UpstreamUpdated struct{ Upstream *Upstream; OldName string }
type UpstreamDeleted struct{ Name string }
type UpstreamToggled struct{ Upstream *Upstream } // For enable/disable toggle

// Message types for reload
type ReloadRequest struct{}
type ReloadComplete struct{ Error error }

// Message types for model selection
type ModelSelected struct{ Model string }

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
	defaultModel string // Current default model

	// Window size for responsive layout
	width  int
	height int

	// Navigation state
	selectedIndex int

	// Form state
	formMode  string    // "", "add", "edit"
	formData  Upstream  // Form working copy
	formField int       // Current field index (0-5)

	// Model selection mode
	modelSelectMode bool

	// Confirmation mode
	confirmMode bool
	confirmType string // "delete" or "shutdown"

	// Callback for upstream changes
	OnUpstreamAdded    func(*Upstream)
	OnUpstreamUpdated  func(*Upstream, string) // upstream, oldName
	OnUpstreamDeleted  func(string)            // name
	OnUpstreamToggled  func(*Upstream)         // for enable/disable
	OnDefaultModelChanged   func(string)   // new global default model
	OnUpstreamModelSelected func(*Upstream) // upstream whose model was selected
	OnReload               func() error   // Config reload callback
}

// NewModel creates a new TUI model
func NewModel(serviceName, version string, port int, upstreams []*Upstream) model {
	return model{
		serviceName:     serviceName,
		version:         version,
		port:            port,
		startTime:       time.Now(),
		upstreams:       upstreams,
		logs:            make([]RequestLog, 0, 50),
		selectedIndex:   0,
		formMode:        "",
		confirmMode:     false,
		modelSelectMode: false,
		width:           80,  // default
		height:          24,  // default
	}
}

// Init initializes the TUI model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles TUI messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.confirmMode {
			return m.handleConfirm(msg)
		}
		if m.formMode != "" {
			return m.handleFormInput(msg)
		}
		if m.modelSelectMode {
			return m.handleModelSelect(msg)
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
		case " ":
			// Toggle upstream enabled/disabled
			if len(m.upstreams) > 0 && m.OnUpstreamToggled != nil {
				us := m.upstreams[m.selectedIndex]
				us.Enabled = !us.Enabled
				m.OnUpstreamToggled(us)
			}
		case "m":
			// Enter model selection mode
			m.modelSelectMode = true
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
		case "r":
			if m.OnReload != nil {
				err := m.OnReload()
				return m, func() tea.Msg {
					return ReloadComplete{Error: err}
				}
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
	case ReloadComplete:
		if msg.Error != nil {
			fmt.Fprintf(os.Stderr, "Reload error: %v\n", msg.Error)
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

// handleModelSelect processes model selection input
func (m model) handleModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down":
			if m.selectedIndex < len(m.upstreams)-1 {
				m.selectedIndex++
			}
		case "enter":
			// Select the current upstream's model as its default (not global)
			if len(m.upstreams) > 0 {
				us := m.upstreams[m.selectedIndex]
				if us.Model != "" {
					// Only update the upstream's model, NOT the global default
					// The global defaultModel stays unchanged in model-select mode
					if m.OnUpstreamModelSelected != nil {
						m.OnUpstreamModelSelected(us)
					}
				}
			}
			m.modelSelectMode = false
		case "esc":
			m.modelSelectMode = false
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Direct number selection - select that upstream's model as its default
			idx := int(msg.Runes[0] - '0')
			if idx < len(m.upstreams) {
				us := m.upstreams[idx]
				if us.Model != "" {
					// Only update the upstream's model, NOT the global default
					if m.OnUpstreamModelSelected != nil {
						m.OnUpstreamModelSelected(us)
					}
				}
			}
			m.modelSelectMode = false
		}
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
			if m.formField < 6 {
				m.formField++
			}
		case "enter":
			if m.formField < 6 {
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
	case 6: // Model
		m.formData.Model += string(runes[0])
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
	case 6: // Model
		if len(m.formData.Model) > 0 {
			m.formData.Model = m.formData.Model[:len(m.formData.Model)-1]
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

// View renders the TUI with three-section layout
func (m model) View() string {
	nav := m.renderNavigation()
	content := m.renderContent()
	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Top,
		nav,
		content,
		status,
	)
}

// renderNavigation renders the top navigation bar
func (m model) renderNavigation() string {
	infoStyle := lipgloss.NewStyle().
		Foreground(mauveColor).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(subtextColor)

	modelStyle := lipgloss.NewStyle().
		Foreground(tealColor).
		Bold(true)

	hintsStyle := lipgloss.NewStyle().
			Foreground(tealColor)

	nav := lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mauveColor).
		Padding(0, 1, 0, 1)

	// Show current model in nav bar
	modelStr := ""
	if m.defaultModel != "" {
		modelStr = modelStyle.Render(" " + m.defaultModel + " ")
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		infoStyle.Render(fmt.Sprintf(" %s v%s ", m.serviceName, m.version)),
		dimStyle.Render(fmt.Sprintf("  Port: %d  Uptime: %s", m.port, time.Since(m.startTime).Round(time.Second))),
		modelStr,
		dimStyle.Render("  "),
		hintsStyle.Render("[a]Add [e]Edit [d]Del [m]Model [r]Reload [q]Quit "),
	)

	return nav.Render(content)
}

// renderContent renders the middle main content area
func (m model) renderContent() string {
	if m.formMode != "" {
		return m.renderForm()
	}
	if m.confirmMode {
		return m.renderConfirmation()
	}
	if m.modelSelectMode {
		return m.renderModelSelect()
	}
	return m.renderUpstreamList()
}

// renderForm renders the add/edit form
func (m model) renderForm() string {
	contentWidth := m.width - 4

	modeStr := "Add Upstream"
	if m.formMode == "edit" {
		modeStr = "Edit Upstream"
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(mauveColor).
		Bold(true).
		Width(contentWidth)

	formStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tealColor).
		Padding(1, 1, 0, 1)

	activeFieldStyle := lipgloss.NewStyle().
				Foreground(greenColor).
				Bold(true)

	inactiveFieldStyle := lipgloss.NewStyle().
				Foreground(textColor)

	dimStyle := lipgloss.NewStyle().
		Foreground(subtextColor)

	var lines []string

	lines = append(lines, headerStyle.Render(modeStr+" (ESC to cancel)"))

	fields := []struct {
		label  string
		value  string
		isActive bool
	}{
		{"Name", m.formData.Name, m.formField == 0},
		{"URL", m.formData.URL, m.formField == 1},
		{"API Key", maskString(m.formData.APIKey), m.formField == 2},
		{"Auth", m.formData.AuthType + " (enter to toggle)", m.formField == 3},
		{"Timeout", fmt.Sprintf("%d seconds", int(m.formData.Timeout/time.Second)), m.formField == 4},
		{"Enabled", fmt.Sprintf("%v (enter to toggle)", m.formData.Enabled), m.formField == 5},
		{"Model", m.formData.Model, m.formField == 6},
	}

	for i, field := range fields {
		prefix := "  "
		style := inactiveFieldStyle
		if field.isActive {
			prefix = " ●"
			style = activeFieldStyle
		}
		line := fmt.Sprintf("%s %-10s %s", prefix, field.label+":", field.value)
		if i == m.formField {
			lines = append(lines, style.Render(line))
		} else {
			lines = append(lines, inactiveFieldStyle.Render(line))
		}
	}

	lines = append(lines, dimStyle.Render("↑↓ Navigate | Enter Next/Submit | b Backspace"))

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		lines...,
	)

	return formStyle.Render(content)
}

// renderConfirmation renders the confirmation dialog
func (m model) renderConfirmation() string {
	warningStyle := lipgloss.NewStyle().
			Foreground(yellowColor).
			Bold(true)

	errorStyle := lipgloss.NewStyle().
			Foreground(redColor).
			Bold(true)

	confirmStyle := lipgloss.NewStyle().
			Width(m.width - 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(yellowColor).
			Padding(1, 2)

	var content string
	if m.confirmType == "delete" {
		upstreamName := ""
		if m.selectedIndex < len(m.upstreams) {
			upstreamName = m.upstreams[m.selectedIndex].Name
		}
		content = lipgloss.JoinVertical(
			lipgloss.Top,
			warningStyle.Render("⚠ Confirm"),
			warningStyle.Render(fmt.Sprintf("Delete '%s'? [y/n]", upstreamName)),
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Top,
			errorStyle.Render("⚠ Confirm"),
			errorStyle.Render("Shutdown? [y/n]"),
		)
	}

	return confirmStyle.Render(content)
}

// renderUpstreamList renders the upstream list view
func (m model) renderUpstreamList() string {
	contentWidth := m.width - 4

	headerStyle := lipgloss.NewStyle().
		Foreground(mauveColor).
		Bold(true).
		Width(contentWidth)

	listStyle := lipgloss.NewStyle().
			Width(contentWidth).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tealColor).
			Padding(1, 1, 0, 1)

	itemStyle := lipgloss.NewStyle().
			Foreground(textColor)

	selectedItemStyle := lipgloss.NewStyle().
			Background(selectedBg).
			Foreground(selectedFg).
			Bold(true)

	enabledStyle := lipgloss.NewStyle().Foreground(greenColor)
	disabledStyle := lipgloss.NewStyle().Foreground(redColor)

	dimStyle := lipgloss.NewStyle().
		Foreground(subtextColor)

	var lines []string

	lines = append(lines, headerStyle.Render("Upstreams"))

	if len(m.upstreams) == 0 {
		lines = append(lines, dimStyle.Render("  (no upstreams configured)"))
	} else {
		for i, us := range m.upstreams {
			status := enabledStyle.Render("●")
			if !us.Enabled {
				status = disabledStyle.Render("○")
			}
			modelStr := ""
			if us.Model != "" {
				modelStr = dimStyle.Render(" → " + us.Model)
			}
			line := fmt.Sprintf("  %s %s [%s]%s", us.Name, status, us.AuthType, modelStr)

			if i == m.selectedIndex && !m.confirmMode {
				lines = append(lines, selectedItemStyle.Render("▶ "+us.Name+" "+enabledStyle.Render("[")+us.AuthType+enabledStyle.Render("]")+modelStr))
			} else {
				lines = append(lines, itemStyle.Render(line))
			}
		}
	}

	lines = append(lines, dimStyle.Render("↑↓ Navigate | Space Toggle | a Add | e Edit | d Delete | m Model | r Reload"))

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		lines...,
	)

	return listStyle.Render(content)
}

// renderModelSelect renders the model selection view
func (m model) renderModelSelect() string {
	contentWidth := m.width - 4

	headerStyle := lipgloss.NewStyle().
		Foreground(mauveColor).
		Bold(true).
		Width(contentWidth)

	listStyle := lipgloss.NewStyle().
			Width(contentWidth).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenColor).
			Padding(1, 1, 0, 1)

	itemStyle := lipgloss.NewStyle().
			Foreground(textColor)

	selectedItemStyle := lipgloss.NewStyle().
			Background(selectedBg).
			Foreground(selectedFg).
			Bold(true)

	enabledStyle := lipgloss.NewStyle().Foreground(greenColor)
	disabledStyle := lipgloss.NewStyle().Foreground(redColor)
	dimStyle := lipgloss.NewStyle().
		Foreground(subtextColor)

	var lines []string

	lines = append(lines, headerStyle.Render("Select Default Model (ESC to cancel)"))

	// Show current default model
	if m.defaultModel != "" {
		lines = append(lines, dimStyle.Render("  Current: "+m.defaultModel))
		lines = append(lines, dimStyle.Render("  "))
	}

	if len(m.upstreams) == 0 {
		lines = append(lines, dimStyle.Render("  (no upstreams configured)"))
	} else {
		for i, us := range m.upstreams {
			status := enabledStyle.Render("●")
			if !us.Enabled {
				status = disabledStyle.Render("○")
			}
			modelStr := us.Model
			if modelStr == "" {
				modelStr = "(no model)"
			}
			line := fmt.Sprintf("  [%d] %s %s → %s", i, us.Name, status, modelStr)

			if i == m.selectedIndex {
				lines = append(lines, selectedItemStyle.Render("▶ "+line))
			} else {
				lines = append(lines, itemStyle.Render(line))
			}
		}
	}

	lines = append(lines, dimStyle.Render(""))
	lines = append(lines, dimStyle.Render("↑↓ Select | Enter to choose | 0-9 Quick select"))

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		lines...,
	)

	return listStyle.Render(content)
}

// renderStatus renders the bottom status bar
func (m model) renderStatus() string {
	total := m.requestCount
	success := m.successCount
	rate := float64(0)
	if total > 0 {
		rate = float64(success) / float64(total) * 100
	}

	greenStyle := lipgloss.NewStyle().Foreground(greenColor)
	redStyle := lipgloss.NewStyle().Foreground(redColor)
	statsStyle := lipgloss.NewStyle().Foreground(textColor)
	labelStyle := lipgloss.NewStyle().Foreground(subtextColor)

	// Build stats
	stats := labelStyle.Render("Total: ") +
		statsStyle.Render(fmt.Sprintf("%d", total)) +
		labelStyle.Render(" | Success: ") +
		greenStyle.Render(fmt.Sprintf("%d", success)) +
		labelStyle.Render(" | Rate: ") +
		greenStyle.Render(fmt.Sprintf("%.1f%%", rate))

	// Last log entry
	var lastLogStr string
	if len(m.logs) > 0 {
		log := m.logs[len(m.logs)-1]
		statusStr := greenStyle.Render(fmt.Sprintf("%d", log.StatusCode))
		if log.StatusCode == 0 || log.StatusCode >= 400 {
			statusStr = redStyle.Render("ERR")
		} else if log.StatusCode >= 400 {
			statusStr = redStyle.Render(fmt.Sprintf("%d", log.StatusCode))
		}
		lastLogStr = labelStyle.Render("Last: ") +
			statsStyle.Render(log.Timestamp.Format("15:04:05")) +
			labelStyle.Render(" | ") +
			statsStyle.Render(fmt.Sprintf("%4dms", log.LatencyMs)) +
			labelStyle.Render(" | ") +
			statsStyle.Render(log.UpstreamName) +
			labelStyle.Render(" | ") +
			statusStr
	}

	// Full-width inverted status bar
	statusBarStyle := lipgloss.NewStyle().
			Width(m.width).
			Background(textColor).
			Foreground(bgColor).
			Padding(0, 1, 0, 1)

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		stats,
		lipgloss.NewStyle().Width(m.width - len(stripAnsi(stats)) - len(stripAnsi(lastLogStr)) - 2).Render(""),
		lastLogStr,
	)

	return statusBarStyle.Render(content)
}

// stripAnsi calculates visible string length (helper for status bar)
func stripAnsi(s string) string {
	// Simple helper - returns approximate length
	result := ""
	inEscape := false
	for _, c := range s {
		if c == '\x1b' {
			inEscape = true
		} else if inEscape && c == 'm' {
			inEscape = false
		} else if !inEscape {
			result += string(c)
		}
	}
	return result
}
