package main

import (
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/truncate"
)

// placeOverlay places fg on top of bg at position x, y
func placeOverlay(x, y int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

	fgWidth := maxLineWidth(fgLines)
	bgWidth := maxLineWidth(bgLines)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		return fg
	}

	// Clamp position
	if x < 0 {
		x = 0
	}
	if x > bgWidth-fgWidth {
		x = bgWidth - fgWidth
	}
	if y < 0 {
		y = 0
	}
	if y > bgHeight-fgHeight {
		y = bgHeight - fgHeight
	}

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		// Get left part of background
		if x > 0 {
			left := truncate.String(bgLine, uint(x))
			b.WriteString(left)
			leftWidth := ansi.StringWidth(left)
			if leftWidth < x {
				b.WriteString(strings.Repeat(" ", x-leftWidth))
			}
		}

		// Write foreground line
		fgLine := fgLines[i-y]
		b.WriteString(fgLine)

		// Get right part of background
		fgLineWidth := ansi.StringWidth(fgLine)
		rightStart := x + fgLineWidth
		bgLineWidth := ansi.StringWidth(bgLine)

		if rightStart < bgLineWidth {
			right := ansi.TruncateLeft(bgLine, rightStart, "")
			b.WriteString(right)
		}
	}

	return b.String()
}

// placeOverlayCenter places fg centered on top of bg
func placeOverlayCenter(fg, bg string, width, height int) string {
	fgWidth := maxLineWidth(strings.Split(fg, "\n"))
	fgHeight := strings.Count(fg, "\n") + 1

	x := (width - fgWidth) / 2
	y := (height - fgHeight) / 2

	return placeOverlay(x, y, fg, bg)
}

// maxLineWidth returns the maximum width among all lines
func maxLineWidth(lines []string) int {
	max := 0
	for _, line := range lines {
		w := ansi.StringWidth(line)
		if w > max {
			max = w
		}
	}
	return max
}

func stringPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

// sidebarWidth returns sidebar width: max(32, terminalWidth/3)
func sidebarWidth(terminalWidth int) int {
	third := terminalWidth / 3
	if third < 32 {
		return third
	}
	return 32
}

// overlayLoadingModal renders a centered loading modal overlay
func overlayLoadingModal(spinnerView, background string, width, height int) string {
	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("231"))
	text := loadingStyle.Render(spinnerView + " Loading...")

	config := ModalConfig{
		Width:    ModalWidth{Value: 30, Percent: false},
		MinWidth: 20,
		MaxWidth: 40,
		Style:    commandMenuStyle,
	}

	// Center the text within the modal content area
	contentWidth := config.Width.Value - config.Style.GetHorizontalFrameSize()
	content := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(text)

	modalContent := renderModal(content, config, width)
	return placeOverlayCenter(modalContent, background, width, height)
}

// openInBrowser opens a URL in the default browser
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			return nil
		}
		cmd.Start()
		return nil
	}
}

// openInSafariWithFallback opens a URL in Safari, falling back to default browser
func openInSafariWithFallback(url string) tea.Cmd {
	return func() tea.Msg {
		if runtime.GOOS == "darwin" {
			// Try Safari first
			cmd := exec.Command("open", "-a", "Safari", url)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
		// Fallback to default browser
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			return nil
		}
		cmd.Start()
		return nil
	}
}
