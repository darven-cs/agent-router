package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI with three-section layout
func (m Model) View() string {
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
func (m Model) renderNavigation() string {
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

	// Show current model in nav bar and active upstream
	modelStr := ""
	if m.DefaultModel != "" {
		modelStr = modelStyle.Render(" " + m.DefaultModel + " ")
	}

	// Show active primary upstream in nav bar (per D-10)
	activeUpstreamStr := ""
	if m.primaryUpstream != nil {
		activeUpstreamStr = styleRed.Render(" [Primary: " + m.primaryUpstream.Name + "]")
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		infoStyle.Render(fmt.Sprintf(" %s v%s ", m.serviceName, m.version)),
		dimStyle.Render(fmt.Sprintf("  Port: %d  Uptime: %s", m.port, time.Since(m.startTime).Round(time.Second))),
		modelStr,
		activeUpstreamStr,
		dimStyle.Render("  "),
		hintsStyle.Render("[a]Add [e]Edit [d]Del [m]Model [r]Reload [q]Quit "),
	)

	return nav.Render(content)
}

// renderContent renders the middle main content area
func (m Model) renderContent() string {
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
func (m Model) renderForm() string {
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
		label    string
		value    string
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
func (m Model) renderConfirmation() string {
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
func (m Model) renderUpstreamList() string {
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

// renderModelSelect renders the primary upstream selection view
// Redesigned in Task 2: index 0 = Auto (hash), 1..N = upstreams
func (m Model) renderModelSelect() string {
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

	lines = append(lines, headerStyle.Render("Select Primary Upstream (ESC to cancel)"))

	// Auto option at index 0
	autoStr := "  Auto (hash) -- FNV hash distribution"
	if m.modelSelectIndex == 0 {
		lines = append(lines, selectedItemStyle.Render("▶ "+autoStr))
	} else {
		lines = append(lines, dimStyle.Render(autoStr))
	}

	// Separator
	lines = append(lines, dimStyle.Render("  ──"))

	if len(m.upstreams) == 0 {
		lines = append(lines, dimStyle.Render("  (no upstreams configured)"))
	} else {
		for i, us := range m.upstreams {
			idx := i + 1 // modelSelectIndex offset: 1..N
			status := enabledStyle.Render("●")
			if !us.Enabled {
				status = disabledStyle.Render("○")
			}
			modelStr := us.Model
			if modelStr == "" {
				modelStr = "(no model)"
			}

			// Show star if this is the current primary
			star := ""
			if m.primaryUpstream != nil && m.primaryUpstream.Name == us.Name {
				star = styleRed.Render(" *")
			}
			line := fmt.Sprintf("  %s %s [%s] → %s%s", us.Name, status, us.AuthType, modelStr, star)

			if idx == m.modelSelectIndex {
				lines = append(lines, selectedItemStyle.Render("▶ "+line))
			} else {
				lines = append(lines, itemStyle.Render(line))
			}
		}
	}

	lines = append(lines, dimStyle.Render(""))
	lines = append(lines, dimStyle.Render("↑↓ Select | Enter to choose | ESC cancel"))

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		lines...,
	)

	return listStyle.Render(content)
}

// renderStatus renders the bottom status bar
func (m Model) renderStatus() string {
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

	// Active upstream display (per D-10)
	var activeStr string
	if m.primaryUpstream != nil {
		activeStr = lipgloss.NewStyle().Foreground(mauveColor).Bold(true).Render("Active: " + m.primaryUpstream.Name)
	} else {
		activeStr = lipgloss.NewStyle().Foreground(subtextColor).Render("Active: Auto (hash)")
	}

	// Build stats
	stats := labelStyle.Render("Total: ") +
		statsStyle.Render(fmt.Sprintf("%d", total)) +
		labelStyle.Render(" | Success: ") +
		greenStyle.Render(fmt.Sprintf("%d", success)) +
		labelStyle.Render(" | Rate: ") +
		greenStyle.Render(fmt.Sprintf("%.1f%%", rate)) +
		labelStyle.Render(" | ") +
		activeStr

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
		// Fallback prefix per D-11
		var prefix string
		if log.RetryAttempt > 0 {
			prefix = styleRed.Render("[Fallback]") + " "
		}
		lastLogStr = labelStyle.Render("Last: ") +
			statsStyle.Render(log.Timestamp.Format("15:04:05")) +
			labelStyle.Render(" | ") +
			statsStyle.Render(fmt.Sprintf("%4dms", log.LatencyMs)) +
			labelStyle.Render(" | ") +
			prefix +
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
		lipgloss.NewStyle().Width(m.width-len(stripAnsi(stats))-len(stripAnsi(lastLogStr))-2).Render(""),
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
