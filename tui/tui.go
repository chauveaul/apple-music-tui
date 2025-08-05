package tui

import (
	"fmt"
	"strings"

	"main/daemon"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/treilik/bubbleboxer"
)

// Focus areas
type focusArea int

const (
	focusLibrary focusArea = iota
	focusPlaylists
	focusMain
)

// Component models for bubbleboxer
type searchHelpModel struct {
	width, height int
}

func (m searchHelpModel) Init() tea.Cmd { return nil }
func (m searchHelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}
func (m searchHelpModel) View() string {
	var content strings.Builder

	// Bold header with title
	content.WriteString(titleStyle.Render("Search"))
	content.WriteString("\n\n")
	content.WriteString("[Search box]")
	content.WriteString("\n")
	content.WriteString("Help: Tab/Ctrl+W+hjkl")

	return content.String()
}

type libraryModel struct {
	width, height int
	selectedItem  int
	focused       bool
	activeItem    int
	scrollOffset  int
}

func (m libraryModel) Init() tea.Cmd { return nil }
func (m libraryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}
func (m libraryModel) View() string {
	libraryItems := []string{"Made For You", "Recently Played", "Liked Songs", "Albums", "Artists", "Podcasts"}

	var content strings.Builder

	// Bold header with title
	content.WriteString(titleStyle.Render("Library"))
	content.WriteString("\n\n")

	// Calculate visible items based on height (height - 2 for header)
	visibleItems := m.height - 2
	showScrollbar := len(libraryItems) > visibleItems

	if showScrollbar {
		visibleItems-- // Make space for scrollbar
	}
	if visibleItems < 0 {
		visibleItems = 0
	}

	// Calculate scroll bounds
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleItems
	if endIdx > len(libraryItems) {
		endIdx = len(libraryItems)
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		item := libraryItems[i]
			var line string
			if i == m.activeItem {
				line = activeItemStyle.Render("> " + item)
			} else if m.focused && i == m.selectedItem {
				line = unfocusedSelectedItemStyle.Render("> " + item)
			} else {
				line = "  " + item
			}

			// Truncate line if too long
			if runewidth.StringWidth(line) > m.width {
				line = runewidth.Truncate(line, m.width-3, "...")
			}
			content.WriteString(line)

		if i < endIdx-1 {
			content.WriteString("\n")
		}
	}

	// Add scroll indicator if there are more items
	if showScrollbar {
		if endIdx > startIdx {
			content.WriteString("\n")
		}
		scrollInfo := fmt.Sprintf("[%d/%d]", m.selectedItem+1, len(libraryItems))
		content.WriteString(scrollInfo)
	}

	return content.String()
}

type playlistsModel struct {
	width, height int
	selectedItem  int
	activeItem    int
	focused       bool
	scrollOffset  int
	playlistItems []string
	lastError     error
}

type playlistsMsg struct {
	playlists []string
	err       error
}

func fetchPlaylists() tea.Msg {
	d := daemon.Daemon{}
	playlists, err := d.GetAllPlaylistNames()
	return playlistsMsg{playlists: playlists, err: err}
}

func (m playlistsModel) Init() tea.Cmd {
	return fetchPlaylists
}
func (m playlistsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case playlistsMsg:
		m.playlistItems = msg.playlists
		m.lastError = msg.err
	}
	return m, nil
}
func (m playlistsModel) View() string {
	// Use cached playlists if available, otherwise show error
	playlistItems := m.playlistItems
	if m.lastError != nil {
		return fmt.Sprintf("Error: %v", m.lastError)
	}
	if len(playlistItems) == 0 {
		return titleStyle.Render("Playlists") + "\n\nLoading..."
	}

	var content strings.Builder

	// Bold header with title
	content.WriteString(titleStyle.Render("Playlists"))
	content.WriteString("\n\n")

	// Calculate how many items can be displayed
	visibleItems := m.height - 2

	if len(m.playlistItems) > visibleItems {
		visibleItems-- // Make space for scrollbar
	}
	if visibleItems < 0 {
		visibleItems = 0
	}

	// Calculate scroll bounds
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleItems
	if endIdx > len(playlistItems) {
		endIdx = len(playlistItems)
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		item := playlistItems[i]
			var line string
			if i == m.activeItem {
				line = activeItemStyle.Render("> " + item)
			} else if m.focused && i == m.selectedItem {
				line = unfocusedSelectedItemStyle.Render("> " + item)
			} else {
				line = "  " + item
			}

			// Truncate line if too long
			if runewidth.StringWidth(line) > m.width {
				line = runewidth.Truncate(line, m.width-3, "...")
			}
			content.WriteString(line)

		if i < endIdx-1 {
			content.WriteString("\n")
		}
	}

	// Add scroll indicator if there are more items
	if len(m.playlistItems) > visibleItems {
		if endIdx > startIdx {
			content.WriteString("\n")
		}
		scrollInfo := fmt.Sprintf("[%d/%d]", m.selectedItem+1, len(m.playlistItems))
		content.WriteString(scrollInfo)
	}

	return content.String()
}

type mainContentModel struct {
	width, height int
	focused       bool
	currentPlaylist string
}

func (m mainContentModel) Init() tea.Cmd { return nil }
func (m mainContentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}
func (m mainContentModel) View() string {
	var content strings.Builder

	// Bold header with title
	content.WriteString(titleStyle.Render("Main Content"))
	content.WriteString("\n\n")

	// If a playlist is selected, show its name. Otherwise, show default content.
	if m.currentPlaylist != "" {
		content.WriteString(fmt.Sprintf("Selected playlist: %s", m.currentPlaylist))
	} else {
		// Content lines
		lines := []string{
			"Please report bugs or missing features to",
			"https://github.com/Rigellute/spotify-tui",
			"",
			"# Changelog",
			"",
			"## [0.25.0] - 2021-08-24",
			"",
			"### Fixed",
			"",
			"- Fixed rate limiting issue [#852]",
			"",
			"- Fix double navigation to same route [#826]",
		}

		// Limit lines to fit within height constraint (height - 2 for header)
		maxLines := m.height - 2
		if maxLines > len(lines) {
			maxLines = len(lines)
		}
		if maxLines < 1 {
			maxLines = 1
		}

		for i := 0; i < maxLines; i++ {
			var line string
			if i < len(lines) {
				line = " " + lines[i] // Add left padding
			} else {
				line = " " // Empty line with padding
			}
			// Truncate line if too long
			if len(line) > m.width {
				line = line[:m.width]
			}
			padding := m.width - len(line)
			if padding < 0 {
				padding = 0
			}
			content.WriteString(line + strings.Repeat(" ", padding))
			content.WriteString("\n")
		}
	}

	return content.String()
}

type instructionsModel struct {
	width        int
	currentFocus focusArea
}

func (m instructionsModel) Init() tea.Cmd { return nil }
func (m instructionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}
func (m instructionsModel) View() string {
	focusName := map[focusArea]string{
		focusLibrary:   "Library",
		focusPlaylists: "Playlists",
		focusMain:      "Main",
	}

	// Don't apply additional styling - bubbleboxer handles layout
	return fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate", focusName[m.currentFocus])
}

// Model represents the application state using bubbleboxer
type Model struct {
	boxer                bubbleboxer.Boxer
	currentFocus         focusArea
	selectedLibraryItem  int
	selectedPlaylistItem int
	ctrlWPressed         bool
	selectedPlaylist     string
}

// Styles
var (
	// Colors
	primaryColor    = lipgloss.Color("#1DB954") // Spotify green
	backgroundColor = lipgloss.Color("#191414")
	sidebarColor    = lipgloss.Color("#121212")
	textColor       = lipgloss.Color("#FFFFFF")
	mutedColor      = lipgloss.Color("#B3B3B3")
	accentColor     = lipgloss.Color("#1ED760")
	focusedBorder   = lipgloss.Color("#1DB954")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Margin(1, 2)

	// For currently selected item
	activeItemStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)

	// For navigated-to but not selected item
	unfocusedSelectedItemStyle = lipgloss.NewStyle().Foreground(accentColor)

	// Focused and unfocused border styles
	focusedStyle = lipgloss.NewStyle().
			Background(sidebarColor).
			Foreground(textColor).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(focusedBorder)

	unfocusedStyle = lipgloss.NewStyle().
			Background(sidebarColor).
			Foreground(textColor).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor)

	mainFocusedStyle = lipgloss.NewStyle().
				Background(backgroundColor).
				Foreground(textColor).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(focusedBorder)

	mainUnfocusedStyle = lipgloss.NewStyle().
				Background(backgroundColor).
				Foreground(textColor).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(mutedColor)

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Bold(true).
			MarginBottom(1)

	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A9EFF")).
			Underline(true)

	searchBoxStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#2A2A2A")).
			Padding(0, 1).
			MarginBottom(1)
)

// NewModel creates and returns a new TUI model
func NewModel() Model {
	boxer := bubbleboxer.Boxer{
		ModelMap: make(map[string]tea.Model),
	}

	// Create leaf nodes
	searchHelpLeaf, _ := boxer.CreateLeaf("searchHelp", searchHelpModel{width: 30, height: 4})
	libraryLeaf, _ := boxer.CreateLeaf("library", libraryModel{width: 30, height: 8, selectedItem: 0, activeItem: -1, focused: true})
	playlistsLeaf, _ := boxer.CreateLeaf("playlists", playlistsModel{width: 30, height: 12, selectedItem: 0, activeItem: -1, focused: false})
	mainLeaf, _ := boxer.CreateLeaf("main", mainContentModel{width: 50, height: 24, currentPlaylist: "", focused: false})
	instructionsLeaf, _ := boxer.CreateLeaf("instructions", instructionsModel{width: 80, currentFocus: focusLibrary})

	// Create the layout tree structure
	// Sidebar (vertical layout)
	sidebar := bubbleboxer.Node{
		Children:        []bubbleboxer.Node{searchHelpLeaf, libraryLeaf, playlistsLeaf},
		VerticalStacked: true,
		SizeFunc: func(node bubbleboxer.Node, widthOrHeight int) []int {
			// Fixed heights: search=4, library=8, rest for playlists
			remaining := widthOrHeight - 4 - 8
			if remaining < 8 {
				remaining = 8
			}
			return []int{4, 8, remaining}
		},
	}

	// Main content area (horizontal layout)
	mainContent := bubbleboxer.Node{
		Children:        []bubbleboxer.Node{sidebar, mainLeaf},
		VerticalStacked: false,
		SizeFunc: func(node bubbleboxer.Node, widthOrHeight int) []int {
			// Sidebar gets 1/3, main gets 2/3
			sidebarWidth := widthOrHeight / 3
			if sidebarWidth < 30 {
				sidebarWidth = 30
			}
			if sidebarWidth > 40 {
				sidebarWidth = 40
			}
			mainWidth := widthOrHeight - sidebarWidth
			return []int{sidebarWidth, mainWidth}
		},
	}

	// Root layout (vertical)
	root := bubbleboxer.Node{
		Children:        []bubbleboxer.Node{mainContent, instructionsLeaf},
		VerticalStacked: true,
		SizeFunc: func(node bubbleboxer.Node, widthOrHeight int) []int {
			// Main content gets most space, instructions get 2 lines
			return []int{widthOrHeight - 2, 2}
		},
	}

	boxer.LayoutTree = root

	return Model{
		boxer:                boxer,
		currentFocus:         focusLibrary,
		selectedLibraryItem:  0,
		selectedPlaylistItem: 0,
		ctrlWPressed:         false,
		selectedPlaylist:     "",
	}
}

func (m Model) Init() tea.Cmd {
	return fetchPlaylists
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Update the boxer first
	var cmd tea.Cmd
	updatedBoxer, boxerCmd := m.boxer.Update(msg)
	m.boxer = updatedBoxer.(bubbleboxer.Boxer)
	if boxerCmd != nil {
		cmd = boxerCmd
	}

	// Handle playlist messages specifically
	switch msg := msg.(type) {
	case playlistsMsg:
		// Forward the message to the playlists model
		m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
			pl := model.(playlistsModel)
			pl.playlistItems = msg.playlists
			pl.lastError = msg.err
			return pl, nil
		})
	case tea.KeyMsg:
		// Handle Ctrl+W combinations
		if m.ctrlWPressed {
			m.ctrlWPressed = false
			switch msg.String() {
			case "h":
				if m.currentFocus == focusMain {
					m.currentFocus = focusPlaylists
				} else if m.currentFocus == focusPlaylists {
					m.currentFocus = focusLibrary
				}
			case "j":
				if m.currentFocus == focusLibrary {
					m.currentFocus = focusPlaylists
				}
			case "k":
				if m.currentFocus == focusPlaylists {
					m.currentFocus = focusLibrary
				}
			case "l":
				if m.currentFocus != focusMain {
					m.currentFocus = focusMain
				}
			}
			m.updateFocus()
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "ctrl+w":
			m.ctrlWPressed = true

		case "enter":
			switch m.currentFocus {
			case focusPlaylists:
				// Get the selected playlist name
				m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
					pl := model.(playlistsModel)
					if m.selectedPlaylistItem >= 0 && m.selectedPlaylistItem < len(pl.playlistItems) {
						m.selectedPlaylist = pl.playlistItems[m.selectedPlaylistItem]
						pl.activeItem = m.selectedPlaylistItem
					}
					return pl, nil
				})
				// Update the main content view
				m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
					main := model.(mainContentModel)
					main.currentPlaylist = m.selectedPlaylist
					return main, nil
				})
				// Un-select the library item
				m.boxer.EditLeaf("library", func(model tea.Model) (tea.Model, error) {
					lib := model.(libraryModel)
					lib.activeItem = -1
					return lib, nil
				})
			case focusLibrary:
				m.boxer.EditLeaf("library", func(model tea.Model) (tea.Model, error) {
					lib := model.(libraryModel)
					lib.activeItem = m.selectedLibraryItem
					return lib, nil
				})
				// Un-select the playlist item
				m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
					pl := model.(playlistsModel)
					pl.activeItem = -1
					return pl, nil
				})
				// Clear the main content view
				m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
					main := model.(mainContentModel)
					main.currentPlaylist = ""
					return main, nil
				})
			}

		case "tab":
			switch m.currentFocus {
			case focusLibrary:
				m.currentFocus = focusPlaylists
			case focusPlaylists:
				m.currentFocus = focusMain
			case focusMain:
				m.currentFocus = focusLibrary
			}
			m.updateFocus()

		case "up", "k":
			switch m.currentFocus {
			case focusLibrary:
				if m.selectedLibraryItem > 0 {
					m.selectedLibraryItem--
					m.updateLibrarySelection()
				}
			case focusPlaylists:
				if m.selectedPlaylistItem > 0 {
					m.selectedPlaylistItem--
					m.updatePlaylistSelection()
				}
			}

		case "down", "j":
			switch m.currentFocus {
			case focusLibrary:
				libraryItems := []string{"Made For You", "Recently Played", "Liked Songs", "Albums", "Artists", "Podcasts"}
				if m.selectedLibraryItem < len(libraryItems)-1 {
					m.selectedLibraryItem++
					m.updateLibrarySelection()
				}
			case focusPlaylists:
				// Get playlist count from the cached model
				var playlistCount int
				m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
					pl := model.(playlistsModel)
					playlistCount = len(pl.playlistItems)
					return pl, nil
				})
				if m.selectedPlaylistItem < playlistCount-1 {
					m.selectedPlaylistItem++
					m.updatePlaylistSelection()
				}
			}
		}
	}

	return m, cmd
}

// Helper methods to update focus and selections
func (m *Model) updateFocus() {
	// Update library focus
	m.boxer.EditLeaf("library", func(model tea.Model) (tea.Model, error) {
		lib := model.(libraryModel)
		lib.focused = (m.currentFocus == focusLibrary)
		lib.selectedItem = m.selectedLibraryItem
		return lib, nil
	})

	// Update playlists focus
	m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
		pl := model.(playlistsModel)
		pl.focused = (m.currentFocus == focusPlaylists)
		pl.selectedItem = m.selectedPlaylistItem
		return pl, nil
	})

	// Update main focus
	m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
		main := model.(mainContentModel)
		main.focused = (m.currentFocus == focusMain)
		return main, nil
	})

	// Update instructions
	m.boxer.EditLeaf("instructions", func(model tea.Model) (tea.Model, error) {
		instr := model.(instructionsModel)
		instr.currentFocus = m.currentFocus
		return instr, nil
	})
}

func (m *Model) updateLibrarySelection() {
	libraryItems := []string{"Made For You", "Recently Played", "Liked Songs", "Albums", "Artists", "Podcasts"}

	m.boxer.EditLeaf("library", func(model tea.Model) (tea.Model, error) {
		lib := model.(libraryModel)
		lib.selectedItem = m.selectedLibraryItem

		// Update scroll offset using same logic as View()
		visibleItems := lib.height - 2
		showScrollbar := len(libraryItems) > visibleItems

		if showScrollbar {
			visibleItems-- // Make space for scrollbar
		}
		if visibleItems < 0 {
			visibleItems = 0
		}

		// If selected item is above visible area, scroll up
		if m.selectedLibraryItem < lib.scrollOffset {
			lib.scrollOffset = m.selectedLibraryItem
		}
		// If selected item is below visible area, scroll down
		if m.selectedLibraryItem >= lib.scrollOffset+visibleItems {
			lib.scrollOffset = m.selectedLibraryItem - visibleItems + 1
		}

		return lib, nil
	})
}

func (m *Model) updatePlaylistSelection() {
	m.boxer.EditLeaf("playlists", func(model tea.Model) (tea.Model, error) {
		pl := model.(playlistsModel)
		pl.selectedItem = m.selectedPlaylistItem

		// Update scroll offset using same logic as View()
		visibleItems := pl.height - 2
		if len(pl.playlistItems) > visibleItems {
			visibleItems-- // Make space for scrollbar
		}
		if visibleItems < 0 {
			visibleItems = 0
		}

		// If selected item is above visible area, scroll up
		if m.selectedPlaylistItem < pl.scrollOffset {
			pl.scrollOffset = m.selectedPlaylistItem
		}
		// If selected item is below visible area, scroll down
		if m.selectedPlaylistItem >= pl.scrollOffset+visibleItems {
			pl.scrollOffset = m.selectedPlaylistItem - visibleItems + 1
		}

		return pl, nil
	})
}

func (m Model) View() string {
	// Create a temporary model to update focus state
	tempModel := m
	tempModel.updateFocus()

	// Use bubbleboxer to render the layout
	return baseStyle.Render(tempModel.boxer.View())
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
