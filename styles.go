package main

import "github.com/charmbracelet/lipgloss"

var (
	// Common styles
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("60"))

	// Main screen styles
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	contentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	// Auth form styles
	formStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("105")).
			MarginBottom(1)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	// Error styles
	errorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("9")).
			Foreground(lipgloss.Color("9")).
			Padding(1, 2)

	errorTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("9"))

	// Command menu styles
	commandMenuStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	commandMenuTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("105")).
				MarginBottom(1)

	commandItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("189"))

	commandItemSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("105"))

	commandDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("60"))

	// Settings modal styles
	settingsTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("105"))

	settingsTabActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("62")).
				Padding(0, 2)

	settingsTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("60")).
				Padding(0, 2)

	settingsLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("189"))

	settingsErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9"))

	// Common button styles
	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("238")).
			Padding(0, 2)

	buttonActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("62")).
				Bold(true).
				Padding(0, 2)

	buttonDangerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("196")).
				Bold(true).
				Padding(0, 2)
)
