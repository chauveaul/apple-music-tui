package tui

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"main/daemon"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/treilik/bubbleboxer"
)

// Focus areas
type focusArea int

const (
	focusSearch focusArea = iota
	focusPlaylists
	focusMain
)

// Component models for bubbleboxer
type searchHelpModel struct {
	width, height int
	textInput     textinput.Model
	searching     bool
}

func (m searchHelpModel) Init() tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	return nil
}
func (m searchHelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	if m.searching {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}
func (m searchHelpModel) View() string {
	// Ensure we have valid dimensions
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Search"))
	lines = append(lines, "")
	if m.searching {
		lines = append(lines, m.textInput.View())
	} else {
		lines = append(lines, "[Search box]")
	}
	lines = append(lines, "Help: / search • Esc cancel")

	// Limit lines to fit within height constraint
	maxLines := m.height
	if maxLines > len(lines) {
		maxLines = len(lines)
	}
	if maxLines < 1 {
		maxLines = 1
	}

	var content strings.Builder
	for i := 0; i < maxLines; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		} else {
			line = ""
		}
		
		// Truncate line if too long
		if len(line) > m.width {
			if m.width > 3 {
				line = line[:m.width-3] + "..."
			} else if m.width > 0 {
				line = line[:m.width]
			}
		}
		
		content.WriteString(line)
		if i < maxLines-1 {
			content.WriteString("\n")
		}
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
	//Removing the queue that we made because it is not a user playlist
	if slices.Index(playlists, "amtui Queue") != -1 {
		playlists = slices.Delete(playlists, slices.Index(playlists, "amtui Queue"), slices.Index(playlists, "amtui Queue")+1)
	}
	//Taking the slice playlists[2:] to remove "Library" and "Music"
	return playlistsMsg{playlists: playlists[2:], err: err}
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
	// Ensure we have valid dimensions
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	// Use cached playlists if available, otherwise show error
	playlistItems := m.playlistItems
	if m.lastError != nil {
		// Return truncated error message if needed
		errorMsg := fmt.Sprintf("Error: %v", m.lastError)
		if len(errorMsg) > m.width {
			if m.width > 3 {
				errorMsg = errorMsg[:m.width-3] + "..."
			} else {
				errorMsg = errorMsg[:m.width]
			}
		}
		return errorMsg
	}
	if len(playlistItems) == 0 {
		return titleStyle.Render("Playlists") + "\n\nLoading..."
	}

	// Build all lines first
	var allLines []string
	allLines = append(allLines, titleStyle.Render("Playlists"))
	allLines = append(allLines, "")

	// Calculate how many items can be displayed (reserve space for header + empty line)
	headerLines := 2
	visibleItems := m.height - headerLines

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

	// Add visible playlist items
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
		allLines = append(allLines, line)
	}

	// Add scroll indicator if there are more items
	if len(m.playlistItems) > visibleItems && len(allLines) < m.height {
		scrollInfo := fmt.Sprintf("[%d/%d]", m.selectedItem+1, len(m.playlistItems))
		allLines = append(allLines, scrollInfo)
	}

	// Ensure we don't exceed height
	maxLines := m.height
	if maxLines > len(allLines) {
		maxLines = len(allLines)
	}

	var content strings.Builder
	for i := 0; i < maxLines; i++ {
		if i < len(allLines) {
			content.WriteString(allLines[i])
		}
		if i < maxLines-1 {
			content.WriteString("\n")
		}
	}

	return content.String()
}

type mainContentModel struct {
	width, height   int
	focused         bool
	currentPlaylist string
	cachedAsciiArt  []string // Cache ASCII art to prevent reshuffling
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
	// Ensure we have valid dimensions
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	var content strings.Builder
	var allLines []string

	// If a playlist is selected, show its name. Otherwise, show cached ASCII art.
	if m.currentPlaylist != "" {
		allLines = []string{
			"Main Content",
			"",
			fmt.Sprintf("Selected playlist: %s", m.currentPlaylist),
		}
	} else {
		// Use cached ASCII art if available, otherwise get a random one
		asciiLines := m.cachedAsciiArt
		if len(asciiLines) == 0 {
			asciiLines = getRandomAsciiArt()
		}
		
		// Build complete content with header
		allLines = append([]string{"Main Content", ""}, asciiLines...)
	}

	// Limit total lines to fit within height constraint
	maxLines := m.height
	if maxLines > len(allLines) {
		maxLines = len(allLines)
	}
	if maxLines < 1 {
		maxLines = 1
	}

	// Render the lines that fit
	for i := 0; i < maxLines; i++ {
		var line string
		if i < len(allLines) {
			if i == 0 {
				// Apply title styling to the first line (header)
				line = " " + titleStyle.Render(allLines[i])
			} else {
				line = " " + allLines[i] // Add left padding
			}
		} else {
			line = " " // Empty line with padding
		}
		
		// Truncate line if too long for the width
		if len(line) > m.width {
			if m.width > 3 {
				line = line[:m.width-3] + "..."
			} else if m.width > 0 {
				line = line[:m.width]
			}
		}
		
		content.WriteString(line)
		
		// Add newline except for the last line
		if i < maxLines-1 {
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
		focusSearch:    "Search",
		focusPlaylists: "Playlists",
		focusMain:      "Main",
	}

	// Don't apply additional styling - bubbleboxer handles layout
	return fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate", focusName[m.currentFocus])
}

// getRandomAsciiArt returns a random ASCII art from the available collection
func getRandomAsciiArt() []string {
	asciiArts := [][]string{
		// Original ASCII Art (your provided one)
		{
			" (`-')  _ <-. (`-')  (`-')                  _      ",
			" (OO ).-/    \\(OO )_ ( OO).->       .->    (_)     ",
			" / ,---.  ,--./  ,-.)/    '._  ,--.(,--.   ,-(`-') ",
			" | \\ /`.\\\\|   `.'   ||'--...__)|  | |(`-') | ( OO) ",
			" '-'|_.' ||  |'.'|  |`--.  .--'|  | |(OO ) |  |  ) ",
			"(|  .-.  ||  |   |  |   |  |   |  | | |  \\(|  |_/  ",
			" |  | |  ||  |   |  |   |  |   \\  '-'(_ .' |  |'-> ",
			" `--' `--'`--'   `--'   `--'    `-----'    `--'    ",
			"",
			"             Welcome to Apple Music TUI!",
			"",
			"               Controls:",
			"     [TAB] Navigate  [ENTER] Select",
			"     [↑↓] Move       [/] Search",
			"               [Q] Quit",
		},
		// Music Note ASCII
		{
			"                    __        .__ ",
			"    _____    ______/  |_ __ __|__|",
			"    \\__  \\  /     \\   __\\  |  \\  |",
			"     / __ \\|  Y Y  \\  | |  |  /  |",
			"    (____  /__|_|  /__| |____/|__|",
			"         \\/      \\/                  ",
			"",
			"           APPLE MUSIC TUI",
			"",
			"               Controls:",
			"     [TAB] Navigate  [ENTER] Select",
			"     [↑↓] Move       [/] Search",
			"               [Q] Quit",
		},
		// Simple Text Art
		{
			"                   _                _    ",
			"  __ _    _ __    | |_    _  _     (_)   ",
			" / _` |  | '  \\   |  _|  | +| |    | |   ",
			" \\__,_|  |_|_|_|  _\\__|   \\_,_|   _|_|_  ",
			"_|\"\"\"\"\"|_|\"\"\"\"\"|_|\"\"\"\"\"|_|\"\"\"\"\"|_|\"\"\"\"\"| ",
			"\"`-0-0-'\"`-0-0-'\"`-0-0-'\"`-0-0-'\"`-0-0-' ",
			"",
			"               Controls:",
			"     [TAB] Navigate  [ENTER] Select",
			"     [↑↓] Move       [/] Search",
			"               [Q] Quit",
		},
		// Retro Style
		{
			"                   __                   ",
			"                  /\\ \\__          __    ",
			"   __      ___ ___\\ \\ ,_\\  __  __/\\_\\   ",
			" /'__`\\  /' __` __`\\ \\ \\/ /\\ \\/\\ \\/\\ \\  ",
			"/\\ \\L\\.\\_/\\ \\/\\ \\/\\ \\ \\ \\_\\ \\ \\_\\ \\ \\ \\ ",
			"\\ \\__/.\\_\\ \\_\\ \\_\\ \\_\\ \\__\\\\ \\____/\\ \\_\\",
			"\\/__/\\/_/\\/_/\\/_/\\/_/\\/__/ \\/___/  \\/_/",
			"",
			"               Controls:",
			"     [TAB] Navigate  [ENTER] Select",
			"     [↑↓] Move       [/] Search",
			"               [Q] Quit",
		},
	}

	// Seed random number generator with current time
	rand.Seed(time.Now().UnixNano())

	// Return a random ASCII art
	return asciiArts[rand.Intn(len(asciiArts))]
}

// Model represents the application state using bubbleboxer
type Model struct {
	boxer                bubbleboxer.Boxer
	currentFocus         focusArea
	selectedPlaylistItem int
	ctrlWPressed         bool
	selectedPlaylist     string
	randomAscii          []string // Store the randomly selected ASCII art
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

	ti := textinput.New()
	ti.Placeholder = "Search for playlists..."
	ti.CharLimit = 156
	ti.Width = 20

	// Generate ASCII art once at startup
	cachedAscii := getRandomAsciiArt()

	// Create leaf nodes
	searchHelpLeaf, _ := boxer.CreateLeaf("searchHelp", searchHelpModel{width: 30, height: 4, textInput: ti})
	playlistsLeaf, _ := boxer.CreateLeaf("playlists", playlistsModel{width: 30, height: 12, selectedItem: 0, activeItem: -1, focused: true})
	mainLeaf, _ := boxer.CreateLeaf("main", mainContentModel{width: 50, height: 24, currentPlaylist: "", focused: false, cachedAsciiArt: cachedAscii})
	instructionsLeaf, _ := boxer.CreateLeaf("instructions", instructionsModel{width: 80, currentFocus: focusPlaylists})

	// Create the layout tree structure
	// Sidebar (vertical layout)
	sidebar := bubbleboxer.Node{
		Children:        []bubbleboxer.Node{searchHelpLeaf, playlistsLeaf},
		VerticalStacked: true,
		SizeFunc: func(node bubbleboxer.Node, widthOrHeight int) []int {
			// Fixed heights: search=4, rest for playlists
			remaining := widthOrHeight - 4
			if remaining < 8 {
				remaining = 8
			}
			return []int{4, remaining}
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
		currentFocus:         focusPlaylists,
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
				}
			case "l":
				if m.currentFocus == focusPlaylists {
					m.currentFocus = focusMain
				}
			}
			m.updateFocus()
			return m, nil
		}

		if m.currentFocus == focusSearch {
			switch msg.String() {
			case "enter":
				// TODO: Implement search filtering
				m.currentFocus = focusPlaylists
				m.updateFocus()
				return m, nil
			case "esc":
				// Clear search and return to playlists
				m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
					sh := model.(searchHelpModel)
					sh.textInput.SetValue("")
					return sh, nil
				})
				m.currentFocus = focusPlaylists
				m.updateFocus()
				return m, nil
			default:
				// Forward all other key events to the search input
				m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
					sh := model.(searchHelpModel)
					var inputCmd tea.Cmd
					sh.textInput, inputCmd = sh.textInput.Update(msg)
					if inputCmd != nil {
						cmd = inputCmd
					}
					return sh, nil
				})
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "/":
			m.currentFocus = focusSearch
			m.updateFocus()
			return m, nil

		case "ctrl+w":
			m.ctrlWPressed = true

		case "enter":
			if m.currentFocus == focusPlaylists {
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
			}

		case "tab":
			if m.currentFocus == focusPlaylists {
				m.currentFocus = focusMain
			} else {
				m.currentFocus = focusPlaylists
			}
			m.updateFocus()

		case "up", "k":
			if m.currentFocus == focusPlaylists {
				if m.selectedPlaylistItem > 0 {
					m.selectedPlaylistItem--
					m.updatePlaylistSelection()
				}
			}

		case "down", "j":
			if m.currentFocus == focusPlaylists {
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
	// Update search focus
	m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
		sh := model.(searchHelpModel)
		sh.searching = (m.currentFocus == focusSearch)
		if sh.searching {
			sh.textInput.Focus()
		} else {
			sh.textInput.Blur()
		}
		return sh, nil
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
