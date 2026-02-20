package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sgrResetBgRe matches any SGR escape sequence that resets the background color:
// \033[0m, \033[m, or any sequence containing parameter 0 or 49.
var sgrResetBgRe = regexp.MustCompile(`\033\[([0-9;]*)m`)

// ThemeConfig represents a theme configuration from the config file
type ThemeConfig struct {
	Name             string `json:"name"`
	Background       string `json:"background,omitempty"`        // App background color (empty or "transparent" = terminal default)
	Accent           string `json:"accent"`
	AccentForeground string `json:"accent_foreground,omitempty"`
	Foreground       string `json:"foreground"`
	Notion           string `json:"notion"`
	NotionForeground string `json:"notion_foreground,omitempty"`
	Success          string `json:"success"`
	SuccessForeground string `json:"success_foreground,omitempty"`
	Warning          string `json:"warning"`
	WarningForeground string `json:"warning_foreground,omitempty"`
	Error            string `json:"error"`
	ErrorForeground  string `json:"error_foreground,omitempty"`
	Muted            string `json:"muted,omitempty"`            // Subtle background for inactive elements
	MutedForeground  string `json:"muted_foreground,omitempty"` // Text color on muted background
	// Optional environment color overrides
	EnvDevelop string `json:"env_develop,omitempty"`
	EnvTest    string `json:"env_test,omitempty"`
	EnvStage   string `json:"env_stage,omitempty"`
	EnvProd    string `json:"env_prod,omitempty"`
}

// ThemeColors holds resolved lipgloss colors for the current theme
type ThemeColors struct {
	Background       lipgloss.Color // App background color ("" = transparent / terminal default)
	HasBackground    bool           // Whether a non-transparent background is set
	Accent           lipgloss.Color
	AccentForeground lipgloss.Color // Text color on accent background
	Foreground       lipgloss.Color
	Notion           lipgloss.Color
	NotionForeground lipgloss.Color // Text color on notion background
	Success          lipgloss.Color
	SuccessForeground lipgloss.Color // Text color on success background
	Warning          lipgloss.Color
	WarningForeground lipgloss.Color // Text color on warning background
	Error            lipgloss.Color
	ErrorForeground  lipgloss.Color // Text color on error background
	Muted            lipgloss.Color // Subtle background for inactive elements (buttons, code blocks, borders)
	MutedForeground  lipgloss.Color // Text color on muted background
	// Environment colors
	EnvDevelop lipgloss.Color
	EnvTest    lipgloss.Color
	EnvStage   lipgloss.Color
	EnvProd    lipgloss.Color
}

// Default indigo theme colors (hardcoded fallback)
var defaultThemeColors = ThemeColors{
	Accent:            lipgloss.Color("#5F5FDF"),
	AccentForeground:  lipgloss.Color("231"),
	Foreground:        lipgloss.Color("#D7D7FF"),
	Notion:            lipgloss.Color("#5F5F8A"),
	NotionForeground:  lipgloss.Color("#D7D7FF"),
	Success:           lipgloss.Color("#00D588"),
	SuccessForeground: lipgloss.Color("#D7D7FF"),
	Warning:           lipgloss.Color("#FFD600"),
	WarningForeground: lipgloss.Color("#D7D7FF"),
	Error:             lipgloss.Color("#FF84A8"),
	ErrorForeground:   lipgloss.Color("#D7D7FF"),
	Muted:             lipgloss.Color("#2A2A3C"),
	MutedForeground:   lipgloss.Color("#686889"),
	EnvDevelop:        lipgloss.Color("#5F5FDF"),
	EnvTest:           lipgloss.Color("#FFD600"),
	EnvStage:          lipgloss.Color("#00D588"),
	EnvProd:           lipgloss.Color("#FF84A8"),
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

// isTransparent returns true if the value explicitly requests no color
func isTransparent(value string) bool {
	return strings.TrimSpace(strings.ToLower(value)) == "transparent"
}

// resolveColor returns the lipgloss.Color for a config color string, or fallback if invalid.
// "transparent" is treated as an explicit no-color (empty lipgloss.Color).
func resolveColor(value string, fallback lipgloss.Color) lipgloss.Color {
	if isTransparent(value) {
		return lipgloss.Color("")
	}
	if value == "" || !isValidHexColor(value) {
		return fallback
	}
	return lipgloss.Color(value)
}

// resolveForegroundColor resolves a <color>_foreground with a three-level fallback:
// 1. The specific foreground value from the theme (e.g. accent_foreground)
// 2. The general foreground from the theme (if explicitly set)
// 3. The specific foreground from the default theme (e.g. defaultThemeColors.AccentForeground)
// "transparent" at any level is treated as an explicit no-color.
func resolveForegroundColor(value string, themeFg string, defaultColor lipgloss.Color) lipgloss.Color {
	if isTransparent(value) {
		return lipgloss.Color("")
	}
	if value != "" && isValidHexColor(value) {
		return lipgloss.Color(value)
	}
	if isTransparent(themeFg) {
		return lipgloss.Color("")
	}
	if themeFg != "" && isValidHexColor(themeFg) {
		return lipgloss.Color(themeFg)
	}
	return defaultColor
}

// themeFromConfig converts a ThemeConfig to ThemeColors with fallbacks
func themeFromConfig(tc ThemeConfig) ThemeColors {
	hasBackground := tc.Background != "" && !isTransparent(tc.Background) && isValidHexColor(tc.Background)
	var bg lipgloss.Color
	if hasBackground {
		bg = lipgloss.Color(tc.Background)
	}
	colors := ThemeColors{
		Background:        bg,
		HasBackground:     hasBackground,
		Accent:            resolveColor(tc.Accent, defaultThemeColors.Accent),
		AccentForeground:  resolveForegroundColor(tc.AccentForeground, tc.Foreground, defaultThemeColors.AccentForeground),
		Foreground:        resolveColor(tc.Foreground, defaultThemeColors.Foreground),
		Notion:            resolveColor(tc.Notion, defaultThemeColors.Notion),
		NotionForeground:  resolveForegroundColor(tc.NotionForeground, tc.Foreground, defaultThemeColors.NotionForeground),
		Success:           resolveColor(tc.Success, defaultThemeColors.Success),
		SuccessForeground: resolveForegroundColor(tc.SuccessForeground, tc.Foreground, defaultThemeColors.SuccessForeground),
		Warning:           resolveColor(tc.Warning, defaultThemeColors.Warning),
		WarningForeground: resolveForegroundColor(tc.WarningForeground, tc.Foreground, defaultThemeColors.WarningForeground),
		Error:             resolveColor(tc.Error, defaultThemeColors.Error),
		ErrorForeground:   resolveForegroundColor(tc.ErrorForeground, tc.Foreground, defaultThemeColors.ErrorForeground),
		Muted:             resolveColor(tc.Muted, defaultThemeColors.Muted),
		MutedForeground:   resolveForegroundColor(tc.MutedForeground, tc.Foreground, defaultThemeColors.MutedForeground),
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
		Foreground(t.Notion)

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
		Foreground(t.MutedForeground).
		Background(t.Muted).
		Padding(0, 2)

	buttonActiveStyle = lipgloss.NewStyle().
		Foreground(t.AccentForeground).
		Background(t.Accent).
		Bold(true).
		Padding(0, 2)

	buttonDangerStyle = lipgloss.NewStyle().
		Foreground(t.ErrorForeground).
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
		Foreground(t.WarningForeground).
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
		Foreground(t.Foreground)

	releaseSuspendedStyle = lipgloss.NewStyle().
		Foreground(t.WarningForeground).
		Background(t.Warning).
		PaddingLeft(1).
		PaddingRight(1)

	releaseSuccessGreenStyle = lipgloss.NewStyle().
		Background(t.Success).
		Foreground(t.SuccessForeground)

	releaseConflictStyle = lipgloss.NewStyle().
		Foreground(t.ErrorForeground).
		Background(t.Error).
		PaddingLeft(1).
		PaddingRight(1).
		Bold(true)

	releaseErrorStyle = lipgloss.NewStyle().
		Foreground(t.ErrorForeground).
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
		BorderForeground(t.Notion)

	releaseTextActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)

	releaseHorizontalLineStyle = lipgloss.NewStyle().
		Foreground(t.Notion)

	// --- git_executor.go ---

	commandLogStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	// --- project_selector.go ---

	projectItemStyle = lipgloss.NewStyle().
		Foreground(t.Foreground)

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
		Foreground(t.Foreground)

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

// parseHexColor parses a "#RRGGBB" hex string into r, g, b components.
func parseHexColor(hex string) (r, g, b int) {
	if len(hex) == 7 && hex[0] == '#' {
		r64, _ := strconv.ParseInt(hex[1:3], 16, 32)
		g64, _ := strconv.ParseInt(hex[3:5], 16, 32)
		b64, _ := strconv.ParseInt(hex[5:7], 16, 32)
		return int(r64), int(g64), int(b64)
	}
	return 0, 0, 0
}

// sgrResetsBackground checks whether an SGR parameter string resets the
// background color (contains parameter 0 or 49, or is empty which equals reset).
func sgrResetsBackground(params string) bool {
	if params == "" || params == "0" {
		return true
	}
	for _, p := range strings.Split(params, ";") {
		if p == "0" || p == "49" {
			return true
		}
	}
	return false
}

// applyFullBackground injects an ANSI 24-bit background escape code into every
// line of the rendered view so the background color persists across all content,
// including after any SGR sequence that resets the background (full reset,
// \033[m, \033[0m, or any sequence containing param 0 or 49).
// It also pads lines to width and fills remaining height with background-colored
// empty lines.
func applyFullBackground(view string, bg lipgloss.Color, width, height int) string {
	r, g, b := parseHexColor(string(bg))
	bgEsc := fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)

	lines := strings.Split(view, "\n")
	var result strings.Builder
	for i, line := range lines {
		// Inject bg escape at start and after every SGR that resets background
		line = bgEsc + sgrResetBgRe.ReplaceAllStringFunc(line, func(match string) string {
			params := sgrResetBgRe.FindStringSubmatch(match)
			if len(params) > 1 && sgrResetsBackground(params[1]) {
				return match + bgEsc
			}
			return match
		})
		// Pad to full width
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			line += strings.Repeat(" ", width-lineWidth)
		}
		line += "\033[0m"
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(line)
	}
	// Fill remaining height
	emptyLine := bgEsc + strings.Repeat(" ", width) + "\033[0m"
	for i := len(lines); i < height; i++ {
		result.WriteString("\n" + emptyLine)
	}
	return result.String()
}
