package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ModalWidth represents width configuration for a modal
type ModalWidth struct {
	Value   int  // Absolute value in characters or percentage (1-100)
	Percent bool // If true, Value is treated as percentage
}

// ModalConfig holds configuration for a modal
type ModalConfig struct {
	Width    ModalWidth
	MinWidth int
	MaxWidth int
	Style    lipgloss.Style
}

// DefaultModalConfig returns a default modal configuration
func DefaultModalConfig() ModalConfig {
	return ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: true},
		MinWidth: 30,
		MaxWidth: 80,
	}
}

// renderModal renders content inside a modal with proper width constraints and text wrapping
func renderModal(content string, config ModalConfig, termWidth int) string {
	// Calculate target width
	targetWidth := config.Width.Value
	if config.Width.Percent {
		targetWidth = (termWidth * config.Width.Value) / 100
	}

	// Apply constraints
	if config.MinWidth > 0 && targetWidth < config.MinWidth {
		targetWidth = config.MinWidth
	}
	if config.MaxWidth > 0 && targetWidth > config.MaxWidth {
		targetWidth = config.MaxWidth
	}

	// Don't exceed terminal width (with some margin for border)
	maxAllowed := termWidth - 4
	if targetWidth > maxAllowed {
		targetWidth = maxAllowed
	}

	// Account for padding and border in the style
	horizontalExtra := config.Style.GetHorizontalFrameSize()
	contentWidth := targetWidth - horizontalExtra

	if contentWidth < 10 {
		contentWidth = 10
	}

	// Wrap the content
	wrappedContent := wrapModalContent(content, contentWidth)

	// Apply the style with fixed width
	return config.Style.Width(targetWidth).Render(wrappedContent)
}

// wrapModalContent wraps text content to fit within specified width
// Preserves intentional newlines and handles words longer than width
func wrapModalContent(content string, width int) string {
	if width <= 0 {
		return content
	}

	var result strings.Builder
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Empty line - preserve it
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Wrap this line
		wrapped := wrapLine(line, width)
		result.WriteString(wrapped)
	}

	return result.String()
}

// wrapLine wraps a single line of text to specified width
func wrapLine(line string, width int) string {
	// Check if line already fits
	if ansi.StringWidth(line) <= width {
		return line
	}

	var result strings.Builder
	words := strings.Fields(line)

	if len(words) == 0 {
		return ""
	}

	currentLine := ""
	for _, word := range words {
		wordWidth := ansi.StringWidth(word)

		// Handle words longer than width - break them
		if wordWidth > width {
			if currentLine != "" {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = ""
			}
			// Break the long word
			broken := breakLongWord(word, width)
			result.WriteString(broken)
			continue
		}

		// Check if word fits on current line
		if currentLine == "" {
			currentLine = word
		} else {
			testLine := currentLine + " " + word
			if ansi.StringWidth(testLine) <= width {
				currentLine = testLine
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}
	}

	// Don't forget the last line
	if currentLine != "" {
		result.WriteString(currentLine)
	}

	return result.String()
}

// breakLongWord breaks a word that's longer than width into multiple lines
func breakLongWord(word string, width int) string {
	var result strings.Builder
	runes := []rune(word)

	currentWidth := 0
	lineStart := 0

	for i, r := range runes {
		charWidth := ansi.StringWidth(string(r))
		if currentWidth+charWidth > width && i > lineStart {
			if lineStart > 0 || result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(string(runes[lineStart:i]))
			lineStart = i
			currentWidth = charWidth
		} else {
			currentWidth += charWidth
		}
	}

	// Write remaining
	if lineStart < len(runes) {
		if lineStart > 0 || result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(string(runes[lineStart:]))
	}

	return result.String()
}

// ErrorModalConfig returns configuration for error modals
func ErrorModalConfig() ModalConfig {
	return ModalConfig{
		Width:    ModalWidth{Value: 60, Percent: true},
		MinWidth: 40,
		MaxWidth: 70,
		Style:    errorBoxStyle,
	}
}

// CommandMenuModalConfig returns configuration for command menu
func CommandMenuModalConfig() ModalConfig {
	return ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: false},
		MinWidth: 40,
		MaxWidth: 60,
		Style:    commandMenuStyle,
	}
}
