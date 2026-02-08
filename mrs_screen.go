package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// mrItemColors defines colors for different MR item states
type mrItemColors struct {
	titleFg  lipgloss.Color
	descFg   lipgloss.Color
	borderFg lipgloss.Color
}

// MR item color schemes
var (
	draftSelectedColors = mrItemColors{
		titleFg:  lipgloss.Color("60"),
		descFg:   lipgloss.Color("60"),
		borderFg: lipgloss.Color("60"),
	}
	draftNormalColors = mrItemColors{
		titleFg:  lipgloss.Color("60"),
		descFg:   lipgloss.Color("60"),
		borderFg: lipgloss.Color("60"),
	}
	checkedSelectedColors = mrItemColors{
		titleFg:  lipgloss.Color("220"),
		descFg:   lipgloss.Color("214"),
		borderFg: lipgloss.Color("220"),
	}
	checkedNormalColors = mrItemColors{
		titleFg:  lipgloss.Color("214"),
		descFg:   lipgloss.Color("172"),
		borderFg: lipgloss.Color("214"),
	}
	normalSelectedColors = mrItemColors{
		titleFg:  lipgloss.Color("105"),
		descFg:   lipgloss.Color("62"),
		borderFg: lipgloss.Color("105"),
	}
	normalNormalColors = mrItemColors{
		titleFg: lipgloss.Color("189"),
		descFg:  lipgloss.Color("103"),
	}
)

// getItemColors returns the appropriate colors based on item state
func getItemColors(isSelected, isDraft, isChecked bool) mrItemColors {
	if isDraft {
		if isSelected {
			return draftSelectedColors
		}
		return draftNormalColors
	}
	if isChecked {
		if isSelected {
			return checkedSelectedColors
		}
		return checkedNormalColors
	}
	if isSelected {
		return normalSelectedColors
	}
	return normalNormalColors
}

// buildTitleStyle creates a title style based on colors and selection state
func buildTitleStyle(colors mrItemColors, isSelected bool) lipgloss.Style {
	if isSelected {
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colors.borderFg).
			Foreground(colors.titleFg).
			Padding(0, 0, 0, 1)
	}
	return lipgloss.NewStyle().
		Padding(0, 0, 0, 2).
		Foreground(colors.titleFg)
}

// buildDescStyle creates a description style based on colors and selection state
func buildDescStyle(colors mrItemColors, isSelected bool) lipgloss.Style {
	if isSelected {
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colors.borderFg).
			Foreground(colors.descFg).
			Padding(0, 0, 0, 1)
	}
	return lipgloss.NewStyle().
		Padding(0, 0, 0, 2).
		Foreground(colors.descFg)
}

// padLine pads a string to specified width for consistent borders
func padLine(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

// truncateWithEllipsis truncates text to fit within width, adding ellipsis if needed
func truncateWithEllipsis(text string, maxWidth int) string {
	if ansi.StringWidth(text) <= maxWidth {
		return text
	}
	runes := []rune(text)
	for ansi.StringWidth(string(runes)) > maxWidth-3 && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

// mrDelegate is a custom delegate for displaying MR items with 2-line titles
type mrDelegate struct {
	selectedMRs map[int]bool
}

func newMRDelegate(selectedMRs map[int]bool) mrDelegate {
	return mrDelegate{selectedMRs: selectedMRs}
}

func (d mrDelegate) Height() int                             { return 3 }
func (d mrDelegate) Spacing() int                            { return 1 }
func (d mrDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d mrDelegate) HeightForItem(item list.Item, m list.Model) int {
	mr, ok := item.(mrListItem)
	if !ok {
		return 3
	}

	title := mr.Title()
	width := m.Width() - 3

	// Adjust width for marker if not draft
	if !mr.MR().Draft {
		width -= 4
	}

	titleLines := wrapText(title, width)
	if len(titleLines) >= 2 {
		return 3
	}
	return 2
}

func (d mrDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mr, ok := item.(mrListItem)
	if !ok {
		return
	}

	contentWidth := m.Width() - 3
	isSelected := index == m.Index()
	isDraft := mr.MR().Draft

	isItemChecked := false
	if !isDraft && d.selectedMRs != nil {
		isItemChecked = d.selectedMRs[mr.MR().IID]
	}

	// Get colors and build styles
	colors := getItemColors(isSelected, isDraft, isItemChecked)
	titleStyle := buildTitleStyle(colors, isSelected)
	descStyle := buildDescStyle(colors, isSelected)

	// Prepare marker
	marker := ""
	wrapWidth := contentWidth

	if !isDraft {
		if isItemChecked {
			marker = "[✓] "
		} else {
			marker = "[ ] "
		}
		wrapWidth -= 4
	}

	// Prepare title lines
	titleLines := wrapText(mr.Title(), wrapWidth)
	if len(titleLines) > 2 {
		titleLines = titleLines[:2]
		if len(titleLines[1]) > 0 {
			titleLines[1] = truncateWithEllipsis(titleLines[1], contentWidth)
		}
	}

	// Prepare description
	desc := truncateWithEllipsis(mr.Description(), contentWidth)

	// Build rendered lines
	var lines []string

	// First title line with marker
	lines = append(lines, titleStyle.Render(padLine(marker+titleLines[0], contentWidth)))

	// Second title line if exists
	if len(titleLines) > 1 {
		lines = append(lines, titleStyle.Render(padLine(titleLines[1], contentWidth)))
	}

	// Description line
	lines = append(lines, descStyle.Render(padLine(desc, contentWidth)))

	fmt.Fprint(w, strings.Join(lines, "\n"))
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
	// Clear selections
	if m.selectedMRs == nil {
		m.selectedMRs = make(map[int]bool)
	} else {
		for k := range m.selectedMRs {
			delete(m.selectedMRs, k)
		}
	}
	l := list.New([]list.Item{}, newMRDelegate(m.selectedMRs), 0, 0)
	l.Title = "Open MRs"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("231")).PaddingLeft(1).PaddingRight(1)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)

	// Disable default quit keybindings (q, esc)
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	// Style "no items" text as white
	l.Styles.NoItems = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("189"))

	m.list = l
	m.ready = false
	m.mrsLoaded = false
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

	m.list.SetSize(sidebarWidth-4, m.height-6)

	if !m.ready {
		m.viewport = viewport.New(contentWidth-4, m.height-6)
		m.viewport.SetContent(m.renderMarkdown())
		m.ready = true
	} else {
		m.viewport.Width = contentWidth - 4
		m.viewport.Height = m.height - 6
	}
}

// updateList handles key events on the main list screen
func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {
	case "q", "esc":
		// Ignore q and esc - only ctrl+c quits
		return m, nil
	case "enter":
		// Don't proceed if MRs failed to load
		if m.mrsLoadError {
			return m, nil
		}
		// Proceed to environment selection (MRs selection is optional for prod releases)
		m.screen = screenEnvSelect
		// Only reset index if no environment was previously selected
		if m.selectedEnv == nil {
			m.envSelectIndex = 0
		}
		return m, nil
	case "o":
		// Open selected MR in browser
		selected := m.list.SelectedItem()
		if selected != nil {
			if mr, ok := selected.(mrListItem); ok {
				return m, openInBrowser(mr.MR().WebURL)
			}
		}
		return m, nil
	case "r":
		// If no project selected, open project selector instead
		if m.selectedProject == nil {
			m.showProjectSelector = true
			m.loadingProjects = true
			return m, tea.Batch(m.spinner.Tick, m.fetchProjects())
		}
		// Refresh MRs with loading modal
		m.loadingMRs = true
		return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
	case " ":
		// Toggle selection for currently focused MR (only for non-drafts)
		selected := m.list.SelectedItem()
		if selected != nil {
			if mr, ok := selected.(mrListItem); ok && !mr.MR().Draft {
				iid := mr.MR().IID
				if m.selectedMRs[iid] {
					delete(m.selectedMRs, iid)
				} else {
					m.selectedMRs[iid] = true
				}
			}
		}
		return m, nil
	case "d":
		// Half page down in viewport
		m.viewport.HalfViewDown()
		return m, nil
	case "u":
		// Half page up in viewport
		m.viewport.HalfViewUp()
		return m, nil
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
	// Show empty content before first load
	if !m.mrsLoaded {
		return ""
	}

	selected := m.list.SelectedItem()
	if selected == nil {
		return lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("189")).Render("No merge requests found.\nPress 'r' to refresh.")
	}

	style := styles.DarkStyleConfig
	style.Document.StylePrimitive.Color = stringPtr("189")
	style.Strong.Color = stringPtr("220")
	style.H1.Prefix = " "
	style.H1.BackgroundColor = stringPtr("62")
	style.H1.Color = stringPtr("231")
	style.H2.Prefix = ""
	style.H3.Prefix = ""
	style.H3.Color = stringPtr("105")

	mr, ok := selected.(mrListItem)
	if !ok {
		return ""
	}

	details := mr.MR()

	// Clean up author name (replace multiple spaces with single space)
	authorName := strings.Join(strings.Fields(details.Author.Name), " ")

	// Build info table row
	discussionInfo := fmt.Sprintf(
		"%d/%d",
		details.DiscussionsResolved,
		details.DiscussionsTotal)
	if details.DiscussionsTotal == 0 {
		discussionInfo = "0/0"
	}

	changesCount := details.ChangesCount
	if changesCount == "" {
		changesCount = "0"
	}

	// Build markdown content
	markdown := fmt.Sprintf(`# %s 

### %s (@%s)
**%s** -> %s (at %s)
 
 | Overview | Commits | Changes |
 |:--------:|:-------:|:-------:|
 | %s | %d | %s |
 
 %s
 `,
		details.Title,
		authorName,
		details.Author.Username,
		details.SourceBranch,
		details.TargetBranch,
		details.CreatedAt.Format("2006 Jan 02 15:04"),
		discussionInfo,
		details.CommitsCount,
		changesCount,
		details.Description,
	)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(m.viewport.Width),
		glamour.WithPreservedNewLines(),
	)

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	return strings.Trim(rendered, "\n")
}

// viewList renders the main list screen
func (m model) viewList() string {
	if !m.ready {
		return ""
	}

	sidebarWidth := m.width / 3
	contentWidth := m.width - sidebarWidth - 4

	var sidebarContent, contentContent string

	// Before first MR load, show empty areas
	if !m.mrsLoaded {
		sidebarContent = ""
		contentContent = ""
	} else {
		sidebarContent = m.list.View()
		contentContent = m.viewport.View()
	}

	// Render sidebar
	sidebar := sidebarStyle.
		Width(sidebarWidth).
		Height(m.height - 4).
		PaddingTop(1).
		PaddingBottom(1).
		Render(sidebarContent)

	// Render content
	content := contentStyle.
		Width(contentWidth).
		Height(m.height - 4).
		PaddingTop(1).
		PaddingBottom(1).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer (centered)
	helpText := "j/k/g/G: nav • space: select • enter: proceed • o: open • r: reload • /: commands"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}
