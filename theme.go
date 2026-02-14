package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ThemeConfig represents a theme configuration from the config file
type ThemeConfig struct {
	Name             string `json:"name"`
	Accent           string `json:"accent"`
	AccentForeground string `json:"accent_foreground,omitempty"`
	Foreground       string `json:"foreground"`
	Notion           string `json:"notion"`
	Success          string `json:"success"`
	Warning          string `json:"warning"`
	Error            string `json:"error"`
	// Optional environment color overrides
	EnvDevelop string `json:"env_develop,omitempty"`
	EnvTest    string `json:"env_test,omitempty"`
	EnvStage   string `json:"env_stage,omitempty"`
	EnvProd    string `json:"env_prod,omitempty"`
}

// ThemeColors holds resolved lipgloss colors for the current theme
type ThemeColors struct {
	Accent           lipgloss.Color
	AccentForeground lipgloss.Color // Text color on accent background
	Foreground       lipgloss.Color
	Notion           lipgloss.Color
	Success          lipgloss.Color
	Warning          lipgloss.Color
	Error            lipgloss.Color
	// Environment colors
	EnvDevelop lipgloss.Color
	EnvTest    lipgloss.Color
	EnvStage   lipgloss.Color
	EnvProd    lipgloss.Color
}

// Default indigo theme colors (hardcoded fallback)
var defaultThemeColors = ThemeColors{
	Accent:           lipgloss.Color("#5F5FDF"),
	AccentForeground: lipgloss.Color("231"),
	Foreground:       lipgloss.Color("#D7D7FF"),
	Notion:           lipgloss.Color("#5F5F8A"),
	Success:          lipgloss.Color("#00D588"),
	Warning:          lipgloss.Color("#FFD600"),
	Error:            lipgloss.Color("#FF84A8"),
	EnvDevelop:       lipgloss.Color("#5F5FDF"),
	EnvTest:          lipgloss.Color("#FFD600"),
	EnvStage:         lipgloss.Color("#00D588"),
	EnvProd:          lipgloss.Color("#FF84A8"),
}

// currentTheme holds the active theme colors
var currentTheme = defaultThemeColors

// isValidHexColor checks if a string is a valid hex color (#RRGGBB)
func isValidHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// resolveColor returns the lipgloss.Color for a config color string, or fallback if invalid
func resolveColor(value string, fallback lipgloss.Color) lipgloss.Color {
	if value == "" || !isValidHexColor(value) {
		return fallback
	}
	return lipgloss.Color(value)
}

// themeFromConfig converts a ThemeConfig to ThemeColors with fallbacks
func themeFromConfig(tc ThemeConfig) ThemeColors {
	colors := ThemeColors{
		Accent:           resolveColor(tc.Accent, defaultThemeColors.Accent),
		AccentForeground: resolveColor(tc.AccentForeground, defaultThemeColors.AccentForeground),
		Foreground:       resolveColor(tc.Foreground, defaultThemeColors.Foreground),
		Notion:           resolveColor(tc.Notion, defaultThemeColors.Notion),
		Success:          resolveColor(tc.Success, defaultThemeColors.Success),
		Warning:          resolveColor(tc.Warning, defaultThemeColors.Warning),
		Error:            resolveColor(tc.Error, defaultThemeColors.Error),
	}
	// Environment colors default to base theme colors if not specified
	colors.EnvDevelop = resolveColor(tc.EnvDevelop, colors.Accent)
	colors.EnvTest = resolveColor(tc.EnvTest, colors.Warning)
	colors.EnvStage = resolveColor(tc.EnvStage, colors.Success)
	colors.EnvProd = resolveColor(tc.EnvProd, colors.Error)
	return colors
}

// loadThemeFromConfig loads and applies the selected theme from config
func loadThemeFromConfig() {
	config, err := LoadConfig()
	if err != nil || len(config.Themes) == 0 {
		currentTheme = defaultThemeColors
		rebuildStyles()
		return
	}

	// Find selected theme
	selectedName := config.SelectedTheme
	for _, tc := range config.Themes {
		if tc.Name == selectedName {
			currentTheme = themeFromConfig(tc)
			rebuildStyles()
			return
		}
	}

	// Selected theme not found, use first theme
	currentTheme = themeFromConfig(config.Themes[0])
	rebuildStyles()
}

// applyTheme applies a specific theme config and rebuilds all styles
func applyTheme(tc ThemeConfig) {
	currentTheme = themeFromConfig(tc)
	rebuildStyles()
}

// captureANSIForeground returns the ANSI escape prefix that lipgloss emits
// when rendering text with the given foreground color (everything before the
// actual character). Returns "" if lipgloss produces no escape.
func captureANSIForeground(color lipgloss.Color) string {
	styled := lipgloss.NewStyle().Foreground(color).Render("X")
	idx := strings.Index(styled, "X")
	if idx > 0 {
		return styled[:idx]
	}
	return ""
}

// buildThemeANSIMap captures the ANSI escape sequences for the semantic colors
// of the given theme. The map is saved alongside release history so that
// terminal output can be remapped when displayed under a different theme.
func buildThemeANSIMap(theme ThemeColors) *ThemeANSIMap {
	return &ThemeANSIMap{
		Warning:    captureANSIForeground(theme.Warning),
		Success:    captureANSIForeground(theme.Success),
		Error:      captureANSIForeground(theme.Error),
		Accent:     captureANSIForeground(theme.Accent),
		Foreground: captureANSIForeground(theme.Foreground),
	}
}

// defaultThemeANSIMap returns the ANSI map for the hardcoded indigo theme.
// Used as a fallback for history entries saved before theme maps were introduced.
func defaultThemeANSIMap() *ThemeANSIMap {
	return buildThemeANSIMap(defaultThemeColors)
}

// rebuildStyles reassigns all package-level style variables based on currentTheme
func rebuildStyles() {
	t := currentTheme

	// --- styles.go ---

	helpStyle = lipgloss.NewStyle().
		Foreground(t.Notion)

	sidebarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(0, 1)

	contentStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(0, 1)

	formStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		MarginBottom(1)

	inputLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	errorBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Error).
		Foreground(t.Error).
		Padding(1, 2)

	errorTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Error)

	commandMenuStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 2)

	commandMenuTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		MarginBottom(1)

	commandItemStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	commandItemSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	commandDescStyle = lipgloss.NewStyle().
		Foreground(t.Notion)

	settingsTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	settingsTabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentForeground).
		Background(t.Accent).
		Padding(0, 2)

	settingsTabStyle = lipgloss.NewStyle().
		Foreground(t.Notion).
		Padding(0, 2)

	settingsLabelStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Foreground)

	settingsErrorStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	buttonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("238")).
		Padding(0, 2)

	buttonActiveStyle = lipgloss.NewStyle().
		Foreground(t.AccentForeground).
		Background(t.Accent).
		Bold(true).
		Padding(0, 2)

	buttonDangerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(t.Error).
		Bold(true).
		Padding(0, 2)

	homeTitleStyle = lipgloss.NewStyle().
		Foreground(t.Accent)

	homeMenuItemStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	homeMenuKeyStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	homeVersionStyle = lipgloss.NewStyle().
		Foreground(t.Notion)

	historyTabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentForeground).
		Background(t.Accent).
		Padding(0, 2)

	historyTabStyle = lipgloss.NewStyle().
		Foreground(t.Notion).
		Padding(0, 2)

	historyHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Notion)

	historyStatusCompletedStyle = lipgloss.NewStyle().
		Foreground(t.Success)

	historyStatusAbortedStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	historyMetaLabelStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	historyMetaValueStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	// --- environment_screen.go ---

	envTitleStepStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("236")).
		Background(t.Warning)

	envTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentForeground).
		Background(t.Accent)

	envItemStyle = lipgloss.NewStyle().
		Foreground(t.Foreground).
		PaddingLeft(2)

	envPromptStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	envHintBaseStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	mrBranchStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	// --- version_screen.go ---

	versionInputStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

	// --- release_screen.go ---

	releasePercentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	releaseSuspendedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("236")).
		Background(t.Warning).
		PaddingLeft(1).
		PaddingRight(1)

	releaseSuccessGreenStyle = lipgloss.NewStyle().
		Background(t.Success).
		Foreground(lipgloss.Color("231"))

	releaseConflictStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(t.Error).
		PaddingLeft(1).
		PaddingRight(1).
		Bold(true)

	releaseErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(t.Error).
		PaddingLeft(1).
		PaddingRight(1).
		Bold(true)

	releaseOrangeStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	releaseActiveTextStyle = lipgloss.NewStyle().
		Foreground(t.Accent)

	releaseTerminalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	releaseTextActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	releaseHorizontalLineStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// --- git_executor.go ---

	commandLogStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	// --- project_selector.go ---

	projectItemStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	projectItemSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	projectItemActiveStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	projectItemActiveSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Warning)

	projectFilterPromptStyle = lipgloss.NewStyle().
		Foreground(t.Accent)

	projectFilterPlaceholderStyle = lipgloss.NewStyle().
		Foreground(t.Notion)

	projectFilterTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231"))

	projectSelectorStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 2)

	// --- mrs_screen.go ---

	draftSelectedColors = mrItemColors{
		titleFg:  t.Notion,
		descFg:   t.Notion,
		borderFg: t.Notion,
	}
	draftNormalColors = mrItemColors{
		titleFg: t.Notion,
		descFg:  t.Notion,
		borderFg: t.Notion,
	}
	checkedSelectedColors = mrItemColors{
		titleFg:  t.Warning,
		descFg:   t.Warning,
		borderFg: t.Warning,
	}
	checkedNormalColors = mrItemColors{
		titleFg:  t.Warning,
		descFg:   t.Warning,
		borderFg: t.Warning,
	}
	normalSelectedColors = mrItemColors{
		titleFg:  t.Accent,
		descFg:   t.Accent,
		borderFg: t.Accent,
	}
	normalNormalColors = mrItemColors{
		titleFg: t.Foreground,
		descFg:  t.Notion,
	}
}
