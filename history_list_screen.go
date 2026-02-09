package main

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// historyDelegate implements list.ItemDelegate for history list items
type historyDelegate struct {
	width int
}

func newHistoryDelegate(width int) historyDelegate {
	return historyDelegate{width: width}
}

func (d historyDelegate) Height() int                             { return 1 }
func (d historyDelegate) Spacing() int                            { return 0 }
func (d historyDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d historyDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	hi, ok := item.(historyListItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	entry := hi.entry

	// Calculate column widths: distribute available width across 4 columns
	availWidth := d.width - 8 // padding and separators
	if availWidth < 40 {
		availWidth = 40
	}

	// Column proportions: tag 30%, env 20%, datetime 30%, mrs 20%
	tagW := availWidth * 30 / 100
	envW := availWidth * 20 / 100
	dateW := availWidth * 30 / 100
	mrsW := availWidth - tagW - envW - dateW

	// Format values
	tag := truncateWithEllipsis(entry.Tag, tagW)
	env := entry.Environment
	dateStr := entry.DateTime.Format("2006-01-02 15:04")
	mrs := fmt.Sprintf("%d mrs", entry.MRCount)

	// Pad columns
	tag = padColumn(tag, tagW)
	env = padColumn(env, envW)
	dateStr = padColumn(dateStr, dateW)
	mrs = padColumn(mrs, mrsW)

	// Style env with color
	envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(getEnvBranchColor(entry.Environment)))
	styledEnv := envStyle.Render(env)

	// Style status indicator
	var statusDot string
	if entry.Status == "completed" {
		statusDot = historyStatusCompletedStyle.Render("●")
	} else {
		statusDot = historyStatusAbortedStyle.Render("●")
	}

	// Build line
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))

	line := statusDot + " " + textStyle.Render(tag) + " " + styledEnv + " " + dateStyle.Render(dateStr) + " " + textStyle.Render(mrs)

	// Apply selection style
	if isSelected {
		selectedStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("105")).
			PaddingLeft(1)
		line = selectedStyle.Render(line)
	} else {
		line = lipgloss.NewStyle().PaddingLeft(2).Render(line)
	}

	fmt.Fprint(w, line)
}

// padColumn pads a string to a fixed width (delegates to padLine from mrs_screen.go)
func padColumn(s string, width int) string {
	return padLine(s, width)
}

// initHistoryListScreen initializes the history list
func (m *model) initHistoryListScreen() {
	listWidth := m.width - 6
	if listWidth < 40 {
		listWidth = 40
	}

	l := list.New([]list.Item{}, newHistoryDelegate(listWidth), listWidth, m.height-8)
	l.Title = "Releases History"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("231")).PaddingLeft(1).PaddingRight(1)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)

	// Disable default quit keybindings
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	l.Styles.NoItems = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("189"))

	m.historyList = l
}

// updateHistoryListSize updates list dimensions on resize
func (m *model) updateHistoryListSize() {
	if m.width == 0 || m.height == 0 {
		return
	}
	listWidth := m.width - 6
	if listWidth < 40 {
		listWidth = 40
	}
	m.historyList.SetSize(listWidth, m.height-8)
	m.historyList.SetDelegate(newHistoryDelegate(listWidth))
}

// fetchHistory creates a command to load history index
func (m *model) fetchHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := LoadHistoryIndex()
		return fetchHistoryMsg{entries: entries, err: err}
	}
}

// updateHistoryList handles key events on the history list screen
func (m model) updateHistoryList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u", "q", "esc":
		// If filtering, let the list handle it; otherwise go back
		if m.historyList.FilterState() == list.Filtering {
			break
		}
		m.screen = screenHome
		return m, nil
	case "enter":
		// If filtering, let the list handle it
		if m.historyList.FilterState() == list.Filtering {
			break
		}
		// Load selected history detail
		selected := m.historyList.SelectedItem()
		if selected != nil {
			if hi, ok := selected.(historyListItem); ok {
				m.loadingHistory = true
				return m, tea.Batch(m.spinner.Tick, m.loadHistoryDetail(hi.entry.ID))
			}
		}
		return m, nil
	}

	// Handle list updates (navigation, filtering)
	var cmd tea.Cmd
	m.historyList, cmd = m.historyList.Update(msg)
	return m, cmd
}

// loadHistoryDetail creates a command to load history detail
func (m *model) loadHistoryDetail(id string) tea.Cmd {
	return func() tea.Msg {
		entry, err := LoadHistoryDetail(id)
		return loadHistoryDetailMsg{entry: entry, err: err}
	}
}

// viewHistoryList renders the history list screen
func (m model) viewHistoryList() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Render header with column labels
	availWidth := m.width - 14
	if availWidth < 40 {
		availWidth = 40
	}
	tagW := availWidth * 30 / 100
	envW := availWidth * 20 / 100
	dateW := availWidth * 30 / 100
	mrsW := availWidth - tagW - envW - dateW

	header := "  " + historyHeaderStyle.Render(
		"  "+padColumn("TAG", tagW)+" "+padColumn("ENV", envW)+" "+padColumn("DATE", dateW)+" "+padColumn("MRS", mrsW),
	)

	listContent := m.historyList.View()

	content := contentStyle.
		Width(m.width - 2).
		Height(m.height - 4).
		PaddingTop(1).
		Render(header + "\n" + listContent)

	// Help footer
	helpText := "j/k: nav • enter: view • /: search • u: home • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, content, help)
}
