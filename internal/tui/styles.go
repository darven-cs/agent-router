package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha palette
var (
	// Base colors
	bgColor       = lipgloss.Color("235") // base
	surfaceColor  = lipgloss.Color("238") // surface0
	surface1Color = lipgloss.Color("239") // surface1
	overlayColor  = lipgloss.Color("243") // overlay

	// Text colors
	textColor    = lipgloss.Color("222") // text
	subtextColor = lipgloss.Color("245") // subtext0
	accentColor  = lipgloss.Color("250") // lavender/highlight

	// Accent colors
	mauveColor  = lipgloss.Color("205") // mauve (primary accent)
	tealColor   = lipgloss.Color("115") // teal (secondary accent)
	greenColor  = lipgloss.Color("114") // green (success)
	redColor    = lipgloss.Color("203") // red (error)
	yellowColor = lipgloss.Color("228") // yellow (warning)

	// Highlight for selected items
	selectedBg = lipgloss.Color("24")  // blue highlight background
	selectedFg = lipgloss.Color("230") // white text on highlight
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
