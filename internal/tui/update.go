package tui

import (
	"fmt"
	"os"
	"time"

	"agent-router/internal/proxy"
	"agent-router/internal/upstream"

	"github.com/charmbracelet/bubbletea"
)

// Update handles TUI messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if len(m.upstreams) > 0 && m.Callbacks.OnUpstreamToggled != nil {
				us := m.upstreams[m.selectedIndex]
				us.Enabled = !us.Enabled
				m.Callbacks.OnUpstreamToggled(us)
			}
		case "m":
			// Enter model selection mode
			m.modelSelectMode = true
			m.modelSelectIndex = 0 // Start at Auto option
		case "a":
			m.formMode = "add"
			m.formData = upstream.Upstream{Enabled: true, Timeout: 30 * time.Second, AuthType: "bearer"}
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
			if m.Callbacks.OnReload != nil {
				err := m.Callbacks.OnReload()
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
	case proxy.RequestLog:
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
func (m Model) handleConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				if m.Callbacks.OnUpstreamDeleted != nil {
					m.Callbacks.OnUpstreamDeleted(deletedName)
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
// This is the ORIGINAL logic from tui.go -- Task 2 will redesign this for primary upstream
func (m Model) handleModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
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
					if m.Callbacks.OnUpstreamModelSelected != nil {
						m.Callbacks.OnUpstreamModelSelected(us)
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
					if m.Callbacks.OnUpstreamModelSelected != nil {
						m.Callbacks.OnUpstreamModelSelected(us)
					}
				}
			}
			m.modelSelectMode = false
		}
	}
	return m, nil
}

// handleFormInput processes form input
func (m Model) handleFormInput(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.formData = upstream.Upstream{}
			m.formField = 0
		default:
			m = m.handleFormTextInput(msg)
		}
	}
	return m, nil
}

// submitForm validates and submits the form
func (m Model) submitForm() (tea.Model, tea.Cmd) {
	// Validate: Name and URL required
	if m.formData.Name == "" || m.formData.URL == "" {
		m.formMode = ""
		m.formField = 0
		return m, nil
	}
	if m.formMode == "add" {
		m.formMode = ""
		m.formField = 0
		if m.Callbacks.OnUpstreamAdded != nil {
			m.Callbacks.OnUpstreamAdded(&m.formData)
		}
		return m, nil
	} else if m.formMode == "edit" {
		oldName := m.upstreams[m.selectedIndex].Name
		m.formMode = ""
		m.formField = 0
		if m.Callbacks.OnUpstreamUpdated != nil {
			m.Callbacks.OnUpstreamUpdated(&m.formData, oldName)
		}
		return m, nil
	}
	return m, nil
}

// handleFormTextInput handles text input for form fields
func (m Model) handleFormTextInput(msg tea.KeyMsg) Model {
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
func (m Model) handleFormBackspace(msg tea.KeyMsg) Model {
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
