package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// historyDelegate implements list.ItemDelegate for history list items
type historyDelegate struct {
	width       int
	selectMode  bool
	selectedIDs map[string]bool
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

	// Column widths
	tagW := 15
	envW := 10
	dateW := 20
	mrsW := 10

	// Format values
	tag := truncateWithEllipsis(entry.Tag, tagW)
	env := entry.Environment
	dateStr := entry.DateTime.Format("02.01.2006 15:04")
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

	// Build checkbox prefix in select mode
	checkbox := ""
	if d.selectMode {
		if d.selectedIDs[entry.ID] {
			checkbox = "[✓] "
		} else {
			checkbox = "[ ] "
		}
	}

	// Build line
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))

	line := checkbox + statusDot + " " + textStyle.Render(tag) + " " + styledEnv + " " + dateStyle.Render(dateStr) + " " + textStyle.Render(mrs)

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
	l.SetShowTitle(false) // Hide title, we render it separately

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

// selectedHistoryCount returns the number of selected history entries
func (m *model) selectedHistoryCount() int {
	count := 0
	for _, v := range m.historySelectedIDs {
		if v {
			count++
		}
	}
	return count
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
	// Handle delete confirmation modal first
	if m.showHistoryDeleteConfirm {
		switch msg.String() {
		case "y", "Y":
			return m.executeHistoryDelete()
		case "n", "N", "esc":
			m.showHistoryDeleteConfirm = false
			return m, nil
		case "enter":
			if m.historyDeleteConfirmIndex == 0 {
				return m.executeHistoryDelete()
			}
			m.showHistoryDeleteConfirm = false
			return m, nil
		case "tab", "left", "right", "h", "l":
			if m.historyDeleteConfirmIndex == 0 {
				m.historyDeleteConfirmIndex = 1
			} else {
				m.historyDeleteConfirmIndex = 0
			}
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+q":
		m.screen = screenHome
		m.historySelectMode = false
		m.historySelectedIDs = nil
		return m, nil
	case "esc":
		// If filtering, let the list handle it
		if m.historyList.FilterState() == list.Filtering {
			break
		}
		// If select mode, exit select mode instead of going back
		if m.historySelectMode {
			m.historySelectMode = false
			m.historySelectedIDs = nil
			return m, nil
		}
		m.screen = screenHome
		return m, nil
	case "v":
		if m.historyList.FilterState() == list.Filtering {
			break
		}
		if m.historySelectMode {
			// Exit select mode
			m.historySelectMode = false
			m.historySelectedIDs = nil
		} else {
			// Enter select mode
			m.historySelectMode = true
			m.historySelectedIDs = make(map[string]bool)
		}
		return m, nil
	case " ":
		if m.historySelectMode {
			selected := m.historyList.SelectedItem()
			if selected != nil {
				if hi, ok := selected.(historyListItem); ok {
					id := hi.entry.ID
					if m.historySelectedIDs[id] {
						delete(m.historySelectedIDs, id)
					} else {
						m.historySelectedIDs[id] = true
					}
				}
			}
			return m, nil
		}
	case "d":
		if m.historySelectMode && m.selectedHistoryCount() > 0 {
			m.showHistoryDeleteConfirm = true
			m.historyDeleteConfirmIndex = 1 // Cancel focused by default
			return m, nil
		}
	case "enter":
		// If filtering, let the list handle it
		if m.historyList.FilterState() == list.Filtering {
			break
		}
		// In select mode, toggle like space
		if m.historySelectMode {
			selected := m.historyList.SelectedItem()
			if selected != nil {
				if hi, ok := selected.(historyListItem); ok {
					id := hi.entry.ID
					if m.historySelectedIDs[id] {
						delete(m.historySelectedIDs, id)
					} else {
						m.historySelectedIDs[id] = true
					}
				}
			}
			return m, nil
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

// executeHistoryDelete performs the actual deletion of selected history entries
func (m model) executeHistoryDelete() (tea.Model, tea.Cmd) {
	err := DeleteHistoryEntries(m.historySelectedIDs)
	if err != nil {
		m.showHistoryDeleteConfirm = false
		m.showErrorModal = true
		m.errorModalMsg = "Failed to delete entries: " + err.Error()
		return m, nil
	}

	// Remove deleted entries from in-memory list
	filtered := make([]HistoryIndexEntry, 0, len(m.historyEntries))
	for _, entry := range m.historyEntries {
		if !m.historySelectedIDs[entry.ID] {
			filtered = append(filtered, entry)
		}
	}
	m.historyEntries = filtered

	// Rebuild list items
	items := make([]list.Item, len(filtered))
	for i, entry := range filtered {
		items[i] = historyListItem{entry: entry}
	}
	m.historyList.SetItems(items)
	m.historyList.Title = fmt.Sprintf("Releases History (%d)", len(filtered))

	// Exit select mode and close modal
	m.historySelectMode = false
	m.historySelectedIDs = nil
	m.showHistoryDeleteConfirm = false

	return m, nil
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

	// Update delegate with current select mode state
	listWidth := m.width - 6
	if listWidth < 40 {
		listWidth = 40
	}
	d := historyDelegate{
		width:       listWidth,
		selectMode:  m.historySelectMode,
		selectedIDs: m.historySelectedIDs,
	}
	m.historyList.SetDelegate(d)

	// Render title
	title := m.historyList.Styles.Title.Render(m.historyList.Title)

	// Render header with column labels
	tagW := 15
	envW := 10
	dateW := 20
	mrsW := 10

	headerPrefix := "  "
	if m.historySelectMode {
		headerPrefix = "      "
	}

	header := "  " + historyHeaderStyle.Render(
		headerPrefix+padColumn("TAG", tagW)+" "+padColumn("ENV", envW)+" "+padColumn("DATE", dateW)+" "+padColumn("MRS", mrsW),
	)

	listContent := m.historyList.View()

	// Render with spacing: title, empty line, header, list
	content := contentStyle.
		Width(m.width - 2).
		Height(m.height - 4).
		Render(title + "\n\n" + header + "\n" + listContent)

	// Help footer
	var helpText string
	if m.historySelectMode {
		helpText = "v: exit select • space: toggle • d: delete • esc: cancel"
	} else {
		helpText = "j/k: nav • enter: view • /: search • v: select • C+q: back • C+c: quit"
	}
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, content, help)
}

// overlayHistoryDeleteConfirm renders the delete history confirmation modal
func (m model) overlayHistoryDeleteConfirm(background string) string {
	var sb strings.Builder

	count := m.selectedHistoryCount()

	title := errorTitleStyle.Render("Delete Releases?")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Delete %d selected release(s)?\n", count))
	sb.WriteString("This cannot be undone.\n\n")

	var deleteBtn, cancelBtn string
	if m.historyDeleteConfirmIndex == 0 {
		deleteBtn = buttonDangerStyle.Render("Delete")
		cancelBtn = buttonStyle.Render("Cancel")
	} else {
		deleteBtn = buttonStyle.Render("Delete")
		cancelBtn = buttonActiveStyle.Render("Cancel")
	}
	sb.WriteString(fmt.Sprintf("       %s       %s", deleteBtn, cancelBtn))

	config := ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: false},
		MinWidth: 40,
		MaxWidth: 60,
		Style:    errorBoxStyle,
	}

	modal := renderModal(sb.String(), config, m.width)
	return placeOverlayCenter(modal, background, m.width, m.height)
}
