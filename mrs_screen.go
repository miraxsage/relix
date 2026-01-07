package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// mrDelegate is a custom delegate for displaying MR items with 2-line titles
type mrDelegate struct {
	styles list.DefaultItemStyles
}

func newMRDelegate() mrDelegate {
	styles := list.NewDefaultItemStyles()
	styles.SelectedTitle = styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		BorderLeftForeground(lipgloss.Color("170"))
	styles.SelectedDesc = styles.SelectedDesc.
		Foreground(lipgloss.Color("170")).
		BorderLeftForeground(lipgloss.Color("170"))

	return mrDelegate{styles: styles}
}

func (d mrDelegate) Height() int                             { return 3 }
func (d mrDelegate) Spacing() int                            { return 1 }
func (d mrDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d mrDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mr, ok := item.(mrListItem)
	if !ok {
		return
	}

	// Total available width from list
	totalWidth := m.Width()
	// Content width after accounting for left border/padding (3 chars: border + space + padding)
	contentWidth := totalWidth - 3

	// Wrap title to 2 lines
	title := mr.Title()
	titleLines := wrapText(title, contentWidth)
	if len(titleLines) > 2 {
		titleLines = titleLines[:2]
		// Add ellipsis to second line if truncated
		line2Width := ansi.StringWidth(titleLines[1])
		if line2Width > 3 {
			// Truncate and add ellipsis
			runes := []rune(titleLines[1])
			for ansi.StringWidth(string(runes)) > contentWidth-3 && len(runes) > 0 {
				runes = runes[:len(runes)-1]
			}
			titleLines[1] = string(runes) + "..."
		}
	}

	// Pad to always have 2 lines
	for len(titleLines) < 2 {
		titleLines = append(titleLines, "")
	}

	desc := mr.Description()
	// Truncate description if too long
	if ansi.StringWidth(desc) > contentWidth {
		runes := []rune(desc)
		for ansi.StringWidth(string(runes)) > contentWidth-3 && len(runes) > 0 {
			runes = runes[:len(runes)-1]
		}
		desc = string(runes) + "..."
	}

	isSelected := index == m.Index()

	// Pad each line to same width for consistent borders
	padLine := func(s string, w int) string {
		sw := ansi.StringWidth(s)
		if sw >= w {
			return s
		}
		return s + strings.Repeat(" ", w-sw)
	}

	line1 := padLine(titleLines[0], contentWidth)
	line2 := padLine(titleLines[1], contentWidth)
	line3 := padLine(desc, contentWidth)

	if isSelected {
		style := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("170")).
			Foreground(lipgloss.Color("170")).
			Padding(0, 0, 0, 1)
		fmt.Fprint(w, style.Render(line1+"\n"+line2+"\n"+line3))
	} else {
		style := lipgloss.NewStyle().
			Padding(0, 0, 0, 2).
			Foreground(lipgloss.Color("252"))
		fmt.Fprint(w, style.Render(line1+"\n"+line2+"\n"+line3))
	}
}

// wrapText wraps text to specified width using display width (handles Unicode)
func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 40
	}

	var lines []string
	words := strings.Fields(text)

	if len(words) == 0 {
		return []string{""}
	}

	currentLine := words[0]
	currentWidth := ansi.StringWidth(currentLine)

	for _, word := range words[1:] {
		wordWidth := ansi.StringWidth(word)
		// +1 for space
		if currentWidth+1+wordWidth <= width {
			currentLine += " " + word
			currentWidth += 1 + wordWidth
		} else {
			lines = append(lines, currentLine)
			currentLine = word
			currentWidth = wordWidth
		}
	}
	lines = append(lines, currentLine)

	return lines
}

// initListScreen initializes the main list screen
func (m *model) initListScreen() {
	// Create empty list initially
	l := list.New([]list.Item{}, newMRDelegate(), 0, 0)
	l.Title = "Open MRs"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)

	m.list = l
	m.ready = false
}

// fetchMRs creates a command to fetch MRs from GitLab
func (m *model) fetchMRs() tea.Cmd {
	return func() tea.Msg {
		if m.creds == nil {
			return fetchMRsMsg{err: fmt.Errorf("no credentials")}
		}

		client := NewGitLabClient(m.creds.GitLabURL, m.creds.Token)

		var mrs []*MergeRequestDetails
		var err error

		if m.selectedProject != nil {
			mrs, err = client.GetProjectMergeRequests(m.selectedProject.ID)
		} else {
			mrs, err = client.GetOpenMergeRequests()
		}

		return fetchMRsMsg{mrs: mrs, err: err}
	}
}

// updateListSize updates the list and viewport dimensions
func (m *model) updateListSize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	sidebarWidth := m.width / 3
	contentWidth := m.width - sidebarWidth - 4

	m.list.SetSize(sidebarWidth-4, m.height-5)

	if !m.ready {
		m.viewport = viewport.New(contentWidth-4, m.height-5)
		m.viewport.SetContent(m.renderMarkdown())
		m.ready = true
	} else {
		m.viewport.Width = contentWidth - 4
		m.viewport.Height = m.height - 5
	}
}

// updateList handles key events on the main list screen
func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "r":
		m.list.Title = "Open MRs (loading...)"
		return m, m.fetchMRs()
	}

	// Handle list updates
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update content when selection changes
	if m.ready {
		m.viewport.SetContent(m.renderMarkdown())
	}

	// Handle viewport updates
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// renderMarkdown renders the markdown content for the selected MR
func (m model) renderMarkdown() string {
	selected := m.list.SelectedItem()
	if selected == nil {
		return "No merge requests found.\n\nPress 'r' to refresh."
	}

	mr, ok := selected.(mrListItem)
	if !ok {
		return ""
	}

	details := mr.MR()

	// Build info table row
	discussionInfo := fmt.Sprintf("%d/%d", details.DiscussionsResolved, details.DiscussionsTotal)
	if details.DiscussionsTotal == 0 {
		discussionInfo = "-"
	}

	changesCount := details.ChangesCount
	if changesCount == "" {
		changesCount = "-"
	}

	// Build markdown content
	markdown := fmt.Sprintf(`# %s

**%s (@%s)** | %s -> %s

| Overview | Commits | Changes |
|:--------:|:-------:|:-------:|
| %s | %d | %s |

---

%s
`,
		details.Title,
		details.Author.Name,
		details.Author.Username,
		details.SourceBranch,
		details.TargetBranch,
		discussionInfo,
		details.CommitsCount,
		changesCount,
		details.Description,
	)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.viewport.Width),
	)

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	return rendered
}

// viewList renders the main list screen
func (m model) viewList() string {
	if !m.ready {
		return "Loading..."
	}

	sidebarWidth := m.width / 3
	contentWidth := m.width - sidebarWidth - 4

	// Render sidebar
	sidebar := sidebarStyle.
		Width(sidebarWidth).
		Height(m.height - 4).
		Render(m.list.View())

	// Render content
	content := contentStyle.
		Width(contentWidth).
		Height(m.height - 4).
		Render(m.viewport.View())

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer (centered)
	helpText := "/: commands • q/esc: quit • ↑/↓: navigate • scroll: pgup/pgdn • r: refresh"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}
