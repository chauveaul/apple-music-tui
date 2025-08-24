package tui

import (
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strings"
	"time"

	"main/daemon"

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
	focusQueue
)

// Component models for bubbleboxer
type searchHelpModel struct {
	width, height int
	searchText    string
	cursorPos     int
	searching     bool
}

func (m searchHelpModel) Init() tea.Cmd {
	return nil
}

func (m searchHelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "backspace":
				if len(m.searchText) > 0 && m.cursorPos > 0 {
					// Remove character before cursor
					m.searchText = m.searchText[:m.cursorPos-1] + m.searchText[m.cursorPos:]
					m.cursorPos--
				}
			case "delete":
				if m.cursorPos < len(m.searchText) {
					// Remove character at cursor
					m.searchText = m.searchText[:m.cursorPos] + m.searchText[m.cursorPos+1:]
				}
			case "left":
				if m.cursorPos > 0 {
					m.cursorPos--
				}
			case "right":
				if m.cursorPos < len(m.searchText) {
					m.cursorPos++
				}
			case "home", "ctrl+a":
				m.cursorPos = 0
			case "end", "ctrl+e":
				m.cursorPos = len(m.searchText)
			default:
				// Insert regular characters
				if len(msg.String()) == 1 && msg.String() != "\n" && msg.String() != "\r" {
					if len(m.searchText) < 156 { // Character limit
						// Insert character at cursor position
						m.searchText = m.searchText[:m.cursorPos] + msg.String() + m.searchText[m.cursorPos:]
						m.cursorPos++
					}
				}
			}
		}
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
		// Create custom search input display
		var searchDisplay strings.Builder
		if len(m.searchText) == 0 {
			// Show placeholder when empty
			searchDisplay.WriteString("Search...")
		} else {
			// Show actual text with cursor
			for i, char := range m.searchText {
				if i == m.cursorPos && m.searching {
					// Show cursor as underscore before character
					searchDisplay.WriteString("_")
				}
				searchDisplay.WriteRune(char)
			}
			// Show cursor at end if needed
			if m.cursorPos >= len(m.searchText) {
				searchDisplay.WriteString("_")
			}
		}

		// Wrap in simple brackets to indicate input field
		searchLine := "[" + searchDisplay.String() + "]"
		lines = append(lines, searchLine)
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

// New message type for full playlist data with tracks
type allPlaylistsMsg struct {
	playlists map[string]daemon.Playlist // Map from playlist name to playlist data
	err       error
}

func fetchPlaylists() tea.Msg {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in fetchPlaylists: %v\n", r)
		}
	}()

	d := daemon.Daemon{}
	playlists, err := d.GetAllPlaylistNames()
	if err != nil {
		fmt.Printf("Error in fetchPlaylists: %v\n", err)
		return playlistsMsg{playlists: nil, err: err}
	}

	//Removing the queue that we made because it is not a user playlist
	if slices.Index(playlists, "amtui Queue") != -1 {
		playlists = slices.Delete(playlists, slices.Index(playlists, "amtui Queue"), slices.Index(playlists, "amtui Queue")+1)
	}
	//Taking the slice playlists[2:] to remove "Library" and "Music"
	if len(playlists) >= 2 {
		playlists = playlists[2:]
	}
	return playlistsMsg{playlists: playlists, err: err}
}

// fetchAllPlaylists runs in a goroutine to fetch all playlist data with tracks
func fetchAllPlaylists() tea.Cmd {
	return func() tea.Msg {
		d := daemon.Daemon{}
		playlists, err := d.GetAllPlaylists()
		if err != nil {
			return allPlaylistsMsg{playlists: nil, err: err}
		}

		// Convert slice to map for quick lookup
		playlistMap := make(map[string]daemon.Playlist)
		for _, playlist := range playlists {
			playlistMap[playlist.Name] = playlist
		}

		return allPlaylistsMsg{playlists: playlistMap, err: nil}
	}
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
		// Return simple error message
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

		// Calculate available space for the playlist name (accounting for prefix and ellipsis)
		availableWidth := m.width - 2 // "  " or "> " prefix
		if availableWidth < 1 {
			availableWidth = 1
		}

		// Truncate the item name first, before applying any styling
		truncatedItem := item
		if runewidth.StringWidth(item) > availableWidth {
			// Reserve space for ellipsis
			truncatedItem = runewidth.Truncate(item, availableWidth-3, "...")
		}

		// Now apply styling only to the (possibly truncated) playlist name
		var line string
		if i == m.activeItem {
			// Only style the playlist name, not the prefix
			line = "> " + activeItemStyle.Render(truncatedItem)
		} else if m.focused && i == m.selectedItem {
			// Only style the playlist name, not the prefix
			line = "> " + unfocusedSelectedItemStyle.Render(truncatedItem)
		} else {
			line = "  " + truncatedItem
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
	// Add references to the main model's cache and loading state
	playlistCache    *map[string]daemon.Playlist
	playlistsLoading *bool
	// Song selection state
	selectedSong int
	scrollOffset int
	// Search results
	searchResults []daemon.Track
	searchQuery   string
	isSearchMode  bool
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

	// If in search mode, show search results
	if m.isSearchMode {
		return m.renderSearchResults()
	}

	// If no playlist is selected, show ASCII art
	if m.currentPlaylist == "" {
		// Use cached ASCII art if available, otherwise get a random one
		asciiLines := m.cachedAsciiArt
		if len(asciiLines) == 0 {
			asciiLines = getRandomAsciiArt()
		}

		// Build complete content with header
		allLines := append([]string{titleStyle.Render("Apple Music TUI"), ""}, asciiLines...)

		// Limit total lines to fit within height constraint
		maxLines := m.height
		if maxLines > len(allLines) {
			maxLines = len(allLines)
		}
		if maxLines < 1 {
			maxLines = 1
		}

		var content strings.Builder
		// Render the lines that fit
		for i := 0; i < maxLines; i++ {
			var line string
			if i < len(allLines) {
				line = " " + allLines[i] // Add left padding
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

	// Check if playlists are still loading
	if m.playlistsLoading != nil && *m.playlistsLoading {
		return " " + titleStyle.Render(m.currentPlaylist) + "\n\n Loading songs..."
	}

	// Get playlist data from cache
	var tracks []daemon.Track
	if m.playlistCache != nil {
		if playlist, exists := (*m.playlistCache)[m.currentPlaylist]; exists {
			tracks = playlist.Tracks
		} else {
			// Fallback to fetching playlist if not in cache
			d := &daemon.Daemon{}
			playlist, err := d.GetPlaylist(m.currentPlaylist)
			if err != nil {
				return " " + titleStyle.Render(m.currentPlaylist) + "\n\n" + fmt.Sprintf("Error fetching playlist: %v", err)
			}
			tracks = playlist.Tracks
		}
	} else {
		return " " + titleStyle.Render(m.currentPlaylist) + "\n\n Playlist cache not available."
	}

	if len(tracks) == 0 {
		return " " + titleStyle.Render(m.currentPlaylist) + "\n\n No tracks found in this playlist."
	}

	// Build the table
	var content strings.Builder

	// Add title
	content.WriteString(" " + titleStyle.Render(m.currentPlaylist) + "\n")

	// Calculate column widths based on available space
	// Reserve space for left padding (1) + separators between columns (3 spaces)
	durationWidth := 5 // Fixed 5 chars for duration (e.g., "3:45") - very conservative

	// Account for: left padding + 3 spaces between columns + duration width
	// Subtract 8 characters for safety margin to prevent bubbleboxer errors
	availableWidth := m.width - 1 - 3 - durationWidth - 8
	if availableWidth < 10 {
		availableWidth = 10 // Very conservative minimum
	}

	// Column width distribution (flexible based on content)
	nameWidth := availableWidth * 40 / 100   // 40% for name
	artistWidth := availableWidth * 30 / 100 // 30% for artist
	albumWidth := availableWidth * 30 / 100  // 30% for album

	// Ensure minimum widths
	minNameWidth := 8
	minArtistWidth := 6
	minAlbumWidth := 6

	if nameWidth < minNameWidth {
		nameWidth = minNameWidth
	}
	if artistWidth < minArtistWidth {
		artistWidth = minArtistWidth
	}
	if albumWidth < minAlbumWidth {
		albumWidth = minAlbumWidth
	}

	// Final check: ensure total doesn't exceed available space
	totalNeeded := 1 + nameWidth + 1 + artistWidth + 1 + albumWidth + 1 + durationWidth // padding + columns + spaces
	if totalNeeded > m.width {
		// Reduce all flexible columns proportionally but protect duration
		excess := totalNeeded - m.width
		flexibleTotal := nameWidth + artistWidth + albumWidth
		if flexibleTotal > excess {
			reduction := float64(excess) / float64(flexibleTotal)
			nameWidth = nameWidth - int(float64(nameWidth)*reduction)
			artistWidth = artistWidth - int(float64(artistWidth)*reduction)
			albumWidth = albumWidth - int(float64(albumWidth)*reduction)

			// Enforce absolute minimums
			if nameWidth < 4 {
				nameWidth = 4
			}
			if artistWidth < 4 {
				artistWidth = 4
			}
			if albumWidth < 4 {
				albumWidth = 4
			}
		}
	}

	// Table header (remove lipgloss styling to avoid width interference)
	header := fmt.Sprintf(" %-*s %-*s %-*s %*s",
		nameWidth, "Name",
		artistWidth, "Artist",
		albumWidth, "Album",
		durationWidth, "Duration")
	content.WriteString(header + "\n")

	// Add a separator line
	separator := strings.Repeat("─", m.width-2)
	content.WriteString(" " + separator + "\n")

	// Calculate visible tracks (reserve space for header + separator + title)
	headerLines := 3 // title + header + separator
	visibleTracks := m.height - headerLines
	if visibleTracks < 1 {
		visibleTracks = 1
	}

	// Handle scrolling
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleTracks
	if endIdx > len(tracks) {
		endIdx = len(tracks)
	}

	// Add track rows
	for i := startIdx; i < endIdx; i++ {
		track := tracks[i]

		// Format duration (convert from seconds string to mm:ss)
		durationStr := "0:00"
		if track.Duration != "" {
			var seconds float64
			if n, err := fmt.Sscanf(track.Duration, "%f", &seconds); err == nil && n > 0 {
				minutes := int(seconds) / 60
				secs := int(seconds) % 60
				durationStr = fmt.Sprintf("%d:%02d", minutes, secs)
			} else {
				durationStr = "0:00"
			}
		}

		// Truncate fields to fit in their columns using proper Unicode width handling
		name := track.Name
		if runewidth.StringWidth(name) > nameWidth {
			name = runewidth.Truncate(name, nameWidth, "...")
		}

		artist := track.Artist
		if runewidth.StringWidth(artist) > artistWidth {
			artist = runewidth.Truncate(artist, artistWidth, "...")
		}

		album := track.Album
		if runewidth.StringWidth(album) > albumWidth {
			album = runewidth.Truncate(album, albumWidth, "...")
		}

		// Format the row using Unicode-aware padding
		row := fmt.Sprintf(" %s %s %s %s",
			padRight(name, nameWidth),
			padRight(artist, artistWidth),
			padRight(album, albumWidth),
			padLeft(durationStr, durationWidth))

		// Apply selection styling if this row is selected and main content is focused
		if i == m.selectedSong && m.focused {
			row = selectedSongStyle.Render(row)
		}

		// Final safety check: ensure row doesn't exceed width
		if len(row) > m.width {
			row = row[:m.width-1] // Truncate with 1 char safety margin
		}

		content.WriteString(row + "\n")
	}

	// Add scroll indicator if needed (only if we have space)
	totalLinesUsed := headerLines + (endIdx - startIdx)
	if len(tracks) > visibleTracks && totalLinesUsed < m.height-1 {
		scrollInfo := fmt.Sprintf(" [%d/%d songs]", m.selectedSong+1, len(tracks))
		content.WriteString("\n" + scrollInfo)
	}

	// Only ensure we don't exceed the height limit
	result := content.String()
	lines := strings.Split(result, "\n")
	if len(lines) > m.height {
		// Truncate to fit within height constraint
		lines = lines[:m.height]
		result = strings.Join(lines, "\n")
	}

	return result
}

// renderSearchResults renders the search results in table format
func (m mainContentModel) renderSearchResults() string {
	// Build the table for search results
	var content strings.Builder

	// Add title
	title := fmt.Sprintf("Search Results for: \"%s\"", m.searchQuery)
	content.WriteString(" " + titleStyle.Render(title) + "\n")

	if len(m.searchResults) == 0 {
		content.WriteString("\n No results found.")
		return content.String()
	}

	// Calculate column widths - same logic as playlist view
	durationWidth := 5
	availableWidth := m.width - 1 - 3 - durationWidth - 8
	if availableWidth < 10 {
		availableWidth = 10
	}

	nameWidth := availableWidth * 40 / 100
	artistWidth := availableWidth * 30 / 100
	albumWidth := availableWidth * 30 / 100

	// Ensure minimum widths
	minNameWidth := 8
	minArtistWidth := 6
	minAlbumWidth := 6

	if nameWidth < minNameWidth {
		nameWidth = minNameWidth
	}
	if artistWidth < minArtistWidth {
		artistWidth = minArtistWidth
	}
	if albumWidth < minAlbumWidth {
		albumWidth = minAlbumWidth
	}

	// Final check: ensure total doesn't exceed available space
	totalNeeded := 1 + nameWidth + 1 + artistWidth + 1 + albumWidth + 1 + durationWidth
	if totalNeeded > m.width {
		excess := totalNeeded - m.width
		flexibleTotal := nameWidth + artistWidth + albumWidth
		if flexibleTotal > excess {
			reduction := float64(excess) / float64(flexibleTotal)
			nameWidth = nameWidth - int(float64(nameWidth)*reduction)
			artistWidth = artistWidth - int(float64(artistWidth)*reduction)
			albumWidth = albumWidth - int(float64(albumWidth)*reduction)

			if nameWidth < 4 {
				nameWidth = 4
			}
			if artistWidth < 4 {
				artistWidth = 4
			}
			if albumWidth < 4 {
				albumWidth = 4
			}
		}
	}

	// Table header
	header := fmt.Sprintf(" %-*s %-*s %-*s %*s",
		nameWidth, "Name",
		artistWidth, "Artist",
		albumWidth, "Album",
		durationWidth, "Duration")
	content.WriteString(header + "\n")

	// Add a separator line
	separator := strings.Repeat("─", m.width-2)
	content.WriteString(" " + separator + "\n")

	// Calculate visible tracks
	headerLines := 3 // title + header + separator
	visibleTracks := m.height - headerLines
	if visibleTracks < 1 {
		visibleTracks = 1
	}

	// Handle scrolling
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleTracks
	if endIdx > len(m.searchResults) {
		endIdx = len(m.searchResults)
	}

	// Add track rows
	for i := startIdx; i < endIdx; i++ {
		track := m.searchResults[i]

		// Format duration
		durationStr := "0:00"
		if track.Duration != "" {
			var seconds float64
			if n, err := fmt.Sscanf(track.Duration, "%f", &seconds); err == nil && n > 0 {
				minutes := int(seconds) / 60
				secs := int(seconds) % 60
				durationStr = fmt.Sprintf("%d:%02d", minutes, secs)
			} else {
				durationStr = "0:00"
			}
		}

		// Truncate fields to fit in their columns
		name := track.Name
		if runewidth.StringWidth(name) > nameWidth {
			name = runewidth.Truncate(name, nameWidth, "...")
		}

		artist := track.Artist
		if runewidth.StringWidth(artist) > artistWidth {
			artist = runewidth.Truncate(artist, artistWidth, "...")
		}

		album := track.Album
		if runewidth.StringWidth(album) > albumWidth {
			album = runewidth.Truncate(album, albumWidth, "...")
		}

		// Format the row
		row := fmt.Sprintf(" %s %s %s %s",
			padRight(name, nameWidth),
			padRight(artist, artistWidth),
			padRight(album, albumWidth),
			padLeft(durationStr, durationWidth))

		// Apply selection styling if this row is selected and main content is focused
		if i == m.selectedSong && m.focused {
			row = selectedSongStyle.Render(row)
		}

		// Final safety check
		if len(row) > m.width {
			row = row[:m.width-1]
		}

		content.WriteString(row + "\n")
	}

	// Add scroll indicator if needed
	totalLinesUsed := headerLines + (endIdx - startIdx)
	if len(m.searchResults) > visibleTracks && totalLinesUsed < m.height-1 {
		scrollInfo := fmt.Sprintf(" [%d/%d results]", m.selectedSong+1, len(m.searchResults))
		content.WriteString("\n" + scrollInfo)
	}

	// Ensure we don't exceed height limit
	result := content.String()
	lines := strings.Split(result, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
		result = strings.Join(lines, "\n")
	}

	return result
}

type playbackModel struct {
	width, height int
	status        daemon.PlaybackStatus
	lastUpdate    time.Time
}

// Message type for playback status updates
type playbackStatusMsg struct {
	status daemon.PlaybackStatus
	err    error
}

// Message type for periodic size checks
type sizeCheckMsg struct{}

// actualSizeMsg represents the actual measured terminal size
type actualSizeMsg struct {
	width, height int
}

// checkTerminalSize creates a command to periodically check terminal size
func checkTerminalSize() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return sizeCheckMsg{}
	})
}

// measureTerminalSize directly queries the terminal for its actual size using system calls
func measureTerminalSize() tea.Cmd {
	return func() tea.Msg {
		// Try to force a fresh terminal size measurement
		// This bypasses any caching that might occur in the terminal or Bubble Tea
		return tea.WindowSize() // Force a new measurement
	}
}

// fetchPlaybackStatus fetches the current playback status from Apple Music
func fetchPlaybackStatus() tea.Cmd {
	return func() tea.Msg {
		d := daemon.Daemon{}
		status, err := d.GetPlaybackStatus()
		return playbackStatusMsg{status: status, err: err}
	}
}

func (m playbackModel) Init() tea.Cmd {
	return fetchPlaybackStatus()
}

func (m playbackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case playbackStatusMsg:
		if msg.err == nil {
			m.status = msg.status
			m.lastUpdate = time.Now()
		}
		// Return a command to fetch status again after 1 second
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg {
			return fetchPlaybackStatus()()
		})
	}
	return m, nil
}

func (m playbackModel) View() string {
	// Ensure we have valid dimensions
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	// Check if we have any status data
	if m.status.Track.Name == "" {
		// No playback info available
		info := "♪ No track playing"
		if len(info) > m.width {
			info = info[:m.width]
		}
		// Center the "no track playing" message
		padding := (m.width - len(info)) / 2
		if padding > 0 {
			info = strings.Repeat(" ", padding) + info
		}
		return info
	}

	// Build the playback status display
	var content strings.Builder

	// Line 1: Track name and artist (centered)
	trackInfo := fmt.Sprintf("♪ %s - %s", m.status.Track.Name, m.status.Track.Artist)
	if len(trackInfo) > m.width {
		if m.width > 10 {
			trackInfo = trackInfo[:m.width-3] + "..."
		} else {
			trackInfo = trackInfo[:m.width]
		}
	}
	// Center the track info
	trackPadding := (m.width - len(trackInfo)) / 2
	if trackPadding > 0 {
		trackInfo = strings.Repeat(" ", trackPadding) + trackInfo
	}
	content.WriteString(trackInfo)

	// Line 2: Large progress bar (if we have height for it)
	if m.height > 1 {
		content.WriteString("\n")

		// Calculate progress percentage
		progressPercent := 0.0
		if m.status.Duration > 0 {
			progressPercent = float64(m.status.Position) / float64(m.status.Duration)
			if progressPercent > 1.0 {
				progressPercent = 1.0
			}
		}

		// Format time strings
		positionStr := formatDuration(int(m.status.Position))
		durationStr := formatDuration(int(m.status.Duration))
		timeInfo := fmt.Sprintf("%s/%s", positionStr, durationStr)

		// Make progress bar much larger - use most of the available width
		// Only reserve minimal space for time info
		timeInfoLen := len(timeInfo)
		// Use 80% of width for progress bar, leave 20% for time and padding
		progressBarWidth := int(float64(m.width) * 0.8)
		// Ensure we have at least some space for time
		if progressBarWidth > m.width-timeInfoLen-2 {
			progressBarWidth = m.width - timeInfoLen - 2
		}
		// Set minimum progress bar width
		if progressBarWidth < 20 {
			progressBarWidth = 20
		}

		// Calculate filled portion of progress bar
		filledWidth := 0
		if progressPercent > 0.0 && progressBarWidth > 0 {
			filledWidth = int(progressPercent*float64(progressBarWidth) + 0.5) // Round to nearest
			if filledWidth > progressBarWidth {
				filledWidth = progressBarWidth
			}
			if filledWidth < 0 {
				filledWidth = 0
			}
		}

		// Build larger progress bar with ASCII characters
		var progressBar strings.Builder
		for i := 0; i < progressBarWidth; i++ {
			if i < filledWidth {
				progressBar.WriteString("█") // Use block character for filled portion
			} else {
				progressBar.WriteString("░") // Use light shade for empty portion
			}
		}

		// Construct the line with progress bar and time
		progressLine := progressBar.String() + " " + timeInfo

		// Final safety check - ensure it doesn't exceed width
		if len(progressLine) > m.width {
			// Fallback to ASCII characters if Unicode causes issues
			progressBar.Reset()
			for i := 0; i < progressBarWidth; i++ {
				if i < filledWidth {
					progressBar.WriteString("=")
				} else {
					progressBar.WriteString("-")
				}
			}
			progressLine = progressBar.String() + " " + timeInfo

			// If still too long, truncate the progress bar
			if len(progressLine) > m.width {
				maxBarWidth := m.width - timeInfoLen - 1
				if maxBarWidth > 0 && maxBarWidth < len(progressBar.String()) {
					progressLine = progressBar.String()[:maxBarWidth] + " " + timeInfo
				} else {
					progressLine = timeInfo // Just show time if no space
				}
			}
		}

		// Center the progress line
		progressPadding := (m.width - len(progressLine)) / 2
		if progressPadding > 0 {
			progressLine = strings.Repeat(" ", progressPadding) + progressLine
		}
		content.WriteString(progressLine)
	}

	// Line 3: Additional info (shuffle, repeat, volume) if we have height
	if m.height > 2 {
		content.WriteString("\n")

		var infoItems []string

		// Add player state
		switch m.status.PlayerState {
		case "playing":
			infoItems = append(infoItems, "Playing")
		case "paused":
			infoItems = append(infoItems, "Paused")
		case "stopped":
			infoItems = append(infoItems, "Stopped")
		}

		// Add shuffle state
		if m.status.Shuffle {
			infoItems = append(infoItems, "Shuffle: On")
		} else {
			infoItems = append(infoItems, "Shuffle: Off")
		}

		// Add repeat state
		switch m.status.RepeatMode {
		case "one":
			infoItems = append(infoItems, "Repeat: One")
		case "all":
			infoItems = append(infoItems, "Repeat: All")
		case "off":
			infoItems = append(infoItems, "Repeat: Off")
		default:
			if m.status.RepeatMode == "" {
				infoItems = append(infoItems, "Repeat: Off")
			} else {
				infoItems = append(infoItems, fmt.Sprintf("Repeat: %s", m.status.RepeatMode))
			}
		}

		// Add volume
		infoItems = append(infoItems, fmt.Sprintf("Volume: %d%%", m.status.Volume))

		statusInfo := strings.Join(infoItems, " • ")
		if len(statusInfo) > m.width {
			statusInfo = statusInfo[:m.width]
		}
		// Center the status info
		statusPadding := (m.width - len(statusInfo)) / 2
		if statusPadding > 0 {
			statusInfo = strings.Repeat(" ", statusPadding) + statusInfo
		}
		content.WriteString(statusInfo)
	}

	return content.String()
}

// formatDuration converts seconds to MM:SS format
func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// padRight pads a string to the right with spaces to reach the target width (Unicode-aware)
func padRight(s string, width int) string {
	currentWidth := runewidth.StringWidth(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

// padLeft pads a string to the left with spaces to reach the target width (Unicode-aware)
func padLeft(s string, width int) string {
	currentWidth := runewidth.StringWidth(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return strings.Repeat(" ", padding) + s
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

	// Build the instruction text based on current focus
	var instructions string
	if m.currentFocus == focusMain {
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter play song • Space play/pause • s shuffle • r repeat • +/- volume", focusName[m.currentFocus])
	} else if m.currentFocus == focusSearch {
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter select • / search • Space play/pause • s shuffle • r repeat • +/- volume", focusName[m.currentFocus])
	} else {
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter select • Space play/pause • s shuffle • r repeat • +/- volume", focusName[m.currentFocus])
	}

	// Truncate if the instructions are too long for the available width
	if len(instructions) > m.width {
		if m.width > 3 {
			instructions = instructions[:m.width-3] + "..."
		} else {
			instructions = instructions[:m.width]
		}
	}

	return instructions
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
			"                      Controls:",
			"            [TAB] Navigate  [ENTER] Select",
			"            [↑↓] Move       [/] Search",
			"                      [Q] Quit",
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
			"	Welcome to Apple Music TUI!",
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
			"	Welcome to Apple Music TUI!",
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
			"	     Welcome to Apple Music TUI!",
			"",
			"                     Controls:",
			"           [TAB] Navigate  [ENTER] Select",
			"           [↑↓] Move       [/] Search",
			"                     [Q] Quit",
		},
	}

	// Seed random number generator with current time
	rand.Seed(time.Now().UnixNano())

	// Return a random ASCII art
	return asciiArts[rand.Intn(len(asciiArts))]
}

// QueueModel represents the queue overlay
type queueModel struct {
	width, height int
	queueInfo     *daemon.QueueInfo
	selectedItem  int
	scrollOffset  int
	visible       bool
	loading       bool
	lastError     error
}

// Message for queue info
type queueInfoMsg struct {
	info *daemon.QueueInfo
	err  error
}

// Message for search results
type searchResultsMsg struct {
	tracks []daemon.Track
	query  string
	err    error
}

// fetchQueueInfo gets the current queue information
func fetchQueueInfo() tea.Cmd {
	return func() tea.Msg {
		d := daemon.Daemon{}
		info, err := d.GetQueueInfo()
		return queueInfoMsg{info: info, err: err}
	}
}

// fetchSearchResults searches for tracks by query
func fetchSearchResults(query string) tea.Cmd {
	return func() tea.Msg {
		d := daemon.Daemon{}
		tracks, err := d.SearchTracks(query)
		return searchResultsMsg{tracks: tracks, query: query, err: err}
	}
}

func (m queueModel) Init() tea.Cmd {
	return fetchQueueInfo()
}

func (m queueModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case queueInfoMsg:
		m.queueInfo = msg.info
		m.lastError = msg.err
		m.loading = false
	}
	return m, nil
}

func (m queueModel) View() string {
	if !m.visible {
		return ""
	}

	// Calculate overlay dimensions (80% of screen)
	overlayWidth := int(float64(m.width) * 0.8)
	overlayHeight := int(float64(m.height) * 0.8)
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	if overlayHeight < 10 {
		overlayHeight = 10
	}

	// Ensure overlay doesn't exceed terminal bounds
	if overlayWidth > m.width {
		overlayWidth = m.width
	}
	if overlayHeight > m.height {
		overlayHeight = m.height
	}

	// Center the overlay
	leftPadding := (m.width - overlayWidth) / 2
	topPadding := (m.height - overlayHeight) / 2

	// Create the full screen overlay with transparent background
	var content strings.Builder

	// Render each line of the full terminal
	for row := 0; row < m.height; row++ {
		if row > 0 {
			content.WriteString("\n")
		}

		// Check if this row is within the overlay area
		if row >= topPadding && row < topPadding+overlayHeight {
			// This row contains overlay content
			overlayRow := row - topPadding

			// Add left transparent padding
			for col := 0; col < leftPadding; col++ {
				content.WriteString(" ")
			}

			// Add overlay content
			if overlayRow == 0 {
				// Top border
				content.WriteString("┌" + strings.Repeat("─", overlayWidth-2) + "┐")
			} else if overlayRow == overlayHeight-1 {
				// Bottom border
				content.WriteString("└" + strings.Repeat("─", overlayWidth-2) + "┘")
			} else {
				// Content area
				content.WriteString("│")

				// Add content based on line
				contentLine := m.getContentLine(overlayRow-1, overlayWidth-2)

				// Use Unicode-aware width calculation for proper padding
				contentWidth := runewidth.StringWidth(contentLine)
				availableContentWidth := overlayWidth - 2 // Account for left and right borders

				// Truncate content if it's too wide
				if contentWidth > availableContentWidth {
					contentLine = runewidth.Truncate(contentLine, availableContentWidth, "")
					contentWidth = availableContentWidth
				}

				// Add the content
				content.WriteString(contentLine)

				// Add padding to fill remaining space
				padding := availableContentWidth - contentWidth
				if padding > 0 {
					content.WriteString(strings.Repeat(" ", padding))
				}

				content.WriteString("│")
			}

			// Add right transparent padding
			rightPadding := m.width - leftPadding - overlayWidth
			for col := 0; col < rightPadding; col++ {
				content.WriteString(" ")
			}
		} else {
			// This row is outside the overlay - make it transparent
			for col := 0; col < m.width; col++ {
				content.WriteString(" ")
			}
		}
	}

	return content.String()
}

func (m queueModel) getContentLine(lineIndex int, maxWidth int) string {
	if m.loading {
		if lineIndex == 1 {
			return " Loading queue information..."
		}
		return ""
	}

	if m.lastError != nil {
		if lineIndex == 1 {
			return fmt.Sprintf(" Error: %v", m.lastError)
		} else if lineIndex == 3 {
			return " Press 'u' to refresh or 'Esc' to close"
		}
		return ""
	}

	if m.queueInfo == nil {
		if lineIndex == 1 {
			return " No queue available - play a playlist to create one"
		} else if lineIndex == 3 {
			return " Press 'Esc' to close"
		}
		return ""
	}

	// Header lines
	if lineIndex == 0 {
		queueTitle := "amtui Queue"
		if m.queueInfo.QueueName == "amtui Queue" {
			return fmt.Sprintf(" 🎵 %s (%d tracks)", queueTitle, m.queueInfo.TotalTracks)
		} else {
			return fmt.Sprintf(" 🎵 Current Playlist: %s (%d tracks)", m.queueInfo.QueueName, m.queueInfo.TotalTracks)
		}
	}
	if lineIndex == 1 {
		return ""
	}

	// Current track info
	if lineIndex == 2 {
		if m.queueInfo.CurrentTrack != nil {
			currentInfo := fmt.Sprintf(" ♪ Now Playing: %s - %s (Track %d)",
				m.queueInfo.CurrentTrack.Name, m.queueInfo.CurrentTrack.Artist, m.queueInfo.CurrentPosition)
			if len(currentInfo) > maxWidth {
				currentInfo = currentInfo[:maxWidth-3] + "..."
			}
			return currentInfo
		} else {
			return " ♪ No track currently playing"
		}
	}

	// Separator
	if lineIndex == 3 {
		return " " + strings.Repeat("─", maxWidth-2)
	}

	// Instructions
	if lineIndex == 4 {
		return " Navigation: ↑↓ select • Enter skip to track • Esc close • u refresh"
	}

	// Empty line for spacing
	if lineIndex == 5 {
		return ""
	}

	// Queue tracks header
	if lineIndex == 6 {
		return " Upcoming Tracks in Queue:"
	}

	if lineIndex >= 7 {
		// Show only upcoming tracks (excluding currently playing song)
		if m.queueInfo.CurrentPosition <= 0 {
			// If no current position, show all tracks
			trackIndex := lineIndex - 7 + m.scrollOffset
			if trackIndex < len(m.queueInfo.Tracks) {
				track := m.queueInfo.Tracks[trackIndex]
				prefix := "   "

				// Highlight selected item
				if trackIndex == m.selectedItem {
					prefix = " > "
				}

				// Show track info with position number
				trackInfo := fmt.Sprintf("%s%d. %s - %s", prefix, trackIndex+1, track.Name, track.Artist)
				if len(trackInfo) > maxWidth {
					trackInfo = trackInfo[:maxWidth-3] + "..."
				}
				return trackInfo
			}
		} else {
			// Show tracks starting AFTER the current position (exclude currently playing)
			currentPosIndex := m.queueInfo.CurrentPosition - 1 // Convert to 0-based
			upcomingTrackIndex := lineIndex - 7 + m.scrollOffset
			actualTrackIndex := currentPosIndex + 1 + upcomingTrackIndex // +1 to skip current track

			if actualTrackIndex < len(m.queueInfo.Tracks) {
				track := m.queueInfo.Tracks[actualTrackIndex]
				prefix := "   "

				// Adjust selected item to work with upcoming tracks display (exclude current)
				adjustedSelectedItem := m.selectedItem - currentPosIndex - 1 // -1 to account for skipped current track
				if upcomingTrackIndex == adjustedSelectedItem {
					prefix = " > "
				}

				// Show track info with original position number
				trackInfo := fmt.Sprintf("%s%d. %s - %s", prefix, actualTrackIndex+1, track.Name, track.Artist)
				if len(trackInfo) > maxWidth {
					trackInfo = trackInfo[:maxWidth-3] + "..."
				}
				return trackInfo
			}
		}
	}

	return ""
}

// Context menu options
type contextMenuOption int

const (
	contextPlay contextMenuOption = iota
	contextAddToQueue
)

// Context menu model
type contextMenuModel struct {
	width, height   int
	visible         bool
	selectedOption  int
	x, y            int // Position of the context menu
	targetSong      daemon.Track
	targetPlaylist  string
	targetSongIndex int
}

func (m contextMenuModel) Init() tea.Cmd { return nil }

func (m contextMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m contextMenuModel) View() string {
	if !m.visible {
		return ""
	}

	// Calculate overlay dimensions (50% of screen width, auto height based on content)
	overlayWidth := int(float64(m.width) * 0.5)
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	if overlayWidth > 60 {
		overlayWidth = 60 // Max width
	}

	// Calculate content height: song info (3 lines) + separator (1) + options (3) + borders (2) + spacing
	overlayHeight := 10 // Fixed height for context menu

	// Ensure overlay doesn't exceed terminal bounds
	if overlayWidth > m.width {
		overlayWidth = m.width
	}
	if overlayHeight > m.height {
		overlayHeight = m.height
	}

	// Center the overlay
	leftPadding := (m.width - overlayWidth) / 2
	topPadding := (m.height - overlayHeight) / 2

	// Create the full screen overlay with transparent background
	var content strings.Builder

	// Render each line of the full terminal
	for row := 0; row < m.height; row++ {
		if row > 0 {
			content.WriteString("\n")
		}

		// Check if this row is within the overlay area
		if row >= topPadding && row < topPadding+overlayHeight {
			// This row contains overlay content
			overlayRow := row - topPadding

			// Add left transparent padding
			for col := 0; col < leftPadding; col++ {
				content.WriteString(" ")
			}

			// Add overlay content
			if overlayRow == 0 {
				// Top border
				content.WriteString("┌" + strings.Repeat("─", overlayWidth-2) + "┐")
			} else if overlayRow == overlayHeight-1 {
				// Bottom border
				content.WriteString("└" + strings.Repeat("─", overlayWidth-2) + "┘")
			} else {
				// Content area
				content.WriteString("│")

				// Add content based on line
				contentLine := m.getContentLine(overlayRow-1, overlayWidth-2)

				// Use Unicode-aware width calculation for proper padding
				contentWidth := runewidth.StringWidth(contentLine)
				availableContentWidth := overlayWidth - 2 // Account for left and right borders

				// Truncate content if it's too wide
				if contentWidth > availableContentWidth {
					contentLine = runewidth.Truncate(contentLine, availableContentWidth, "...")
					contentWidth = runewidth.StringWidth(contentLine)
				}

				// Add the content
				content.WriteString(contentLine)

				// Add padding to fill remaining space
				padding := availableContentWidth - contentWidth
				if padding > 0 {
					content.WriteString(strings.Repeat(" ", padding))
				}

				content.WriteString("│")
			}

			// Add right transparent padding
			rightPadding := m.width - leftPadding - overlayWidth
			for col := 0; col < rightPadding; col++ {
				content.WriteString(" ")
			}
		} else {
			// This row is outside the overlay - make it transparent
			for col := 0; col < m.width; col++ {
				content.WriteString(" ")
			}
		}
	}

	return content.String()
}

// Model represents the application state using bubbleboxer
type Model struct {
	boxer                bubbleboxer.Boxer
	currentFocus         focusArea
	selectedPlaylistItem int
	ctrlWPressed         bool
	selectedPlaylist     string
	randomAscii          []string                   // Store the randomly selected ASCII art
	playlistCache        map[string]daemon.Playlist // Cache of all playlists
	playlistsLoading     bool                       // Flag to track if playlists are still loading
	// Track terminal size for yabai compatibility
	lastWidth, lastHeight int
	// Queue overlay
	queueOverlay queueModel
	queueVisible bool
	// Context menu
	contextMenu    contextMenuModel
	contextVisible bool
	// Track change detection for automatic queue cleanup
	lastPlayingTrack string // Track ID of the last playing track to detect changes
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

	// Song table styles
	selectedSongStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#2D2D2D")).
				Foreground(textColor)

	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Bold(true)

	// Queue overlay styles
	queueOverlayStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1A1A1A")).
				Foreground(textColor).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(focusedBorder)
)

// NewModel creates and returns a new TUI model
func NewModel() Model {
	boxer := bubbleboxer.Boxer{
		ModelMap: make(map[string]tea.Model),
	}

	// Generate ASCII art once at startup
	cachedAscii := getRandomAsciiArt()

	// Initialize the cache and loading state
	playlistCache := make(map[string]daemon.Playlist)
	playlistsLoading := true

	// Create leaf nodes
	searchHelpLeaf, _ := boxer.CreateLeaf("searchHelp", searchHelpModel{width: 30, height: 4, searchText: "", cursorPos: 0, searching: false})
	playlistsLeaf, _ := boxer.CreateLeaf("playlists", playlistsModel{width: 30, height: 12, selectedItem: 0, activeItem: -1, focused: true})
	mainLeaf, _ := boxer.CreateLeaf("main", mainContentModel{width: 50, height: 24, currentPlaylist: "", focused: false, cachedAsciiArt: cachedAscii, playlistCache: &playlistCache, playlistsLoading: &playlistsLoading})
	playbackLeaf, _ := boxer.CreateLeaf("playback", playbackModel{width: 80, height: 3})
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
			// Responsive sidebar sizing based on terminal width
			var sidebarWidth int
			if widthOrHeight <= 80 {
				// Small screens: sidebar gets 1/3 but minimum 25
				sidebarWidth = widthOrHeight / 3
				if sidebarWidth < 25 {
					sidebarWidth = 25
				}
			} else if widthOrHeight <= 120 {
				// Medium screens: fixed sidebar width
				sidebarWidth = 35
			} else if widthOrHeight <= 160 {
				// Large screens: slightly larger sidebar
				sidebarWidth = 40
			} else {
				// Very large screens: cap sidebar but allow more space
				sidebarWidth = 45
			}

			mainWidth := widthOrHeight - sidebarWidth
			return []int{sidebarWidth, mainWidth}
		},
	}

	// Root layout (vertical) - now includes playback viewer
	root := bubbleboxer.Node{
		Children:        []bubbleboxer.Node{mainContent, playbackLeaf, instructionsLeaf},
		VerticalStacked: true,
		SizeFunc: func(node bubbleboxer.Node, widthOrHeight int) []int {
			// Main content gets most space, playback gets 3 lines, instructions get 2 lines
			mainHeight := widthOrHeight - 3 - 2
			if mainHeight < 10 {
				mainHeight = 10
			}
			return []int{mainHeight, 3, 2}
		},
	}

	boxer.LayoutTree = root

	return Model{
		boxer:                boxer,
		currentFocus:         focusPlaylists,
		selectedPlaylistItem: 0,
		ctrlWPressed:         false,
		selectedPlaylist:     "",
		playlistCache:        make(map[string]daemon.Playlist),
		playlistsLoading:     true,
		queueOverlay:         queueModel{visible: false, loading: false},
		queueVisible:         false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchPlaylists,        // Fetch playlist names quickly for UI
		fetchAllPlaylists(),   // Start background fetch of all playlist data
		fetchPlaybackStatus(), // Start fetching playback status
		checkTerminalSize(),   // Start periodic size checking for yabai compatibility
	)
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
	case allPlaylistsMsg:
		// Cache the full playlist data
		if msg.err != nil {
			// Handle error - could show a notification or log it
			fmt.Printf("Error loading playlists: %v\n", msg.err)
		} else {
			m.playlistCache = msg.playlists
		}
		m.playlistsLoading = false
	case playbackStatusMsg:
		// Forward playback status messages to the playback model
		var playbackCmd tea.Cmd
		m.boxer.EditLeaf("playback", func(model tea.Model) (tea.Model, error) {
			pb := model.(playbackModel)
			updatedPb, pbCmd := pb.Update(msg)
			playbackCmd = pbCmd // Capture the command for scheduling next update
			return updatedPb, nil
		})
		// Combine any existing command with the playback command
		if playbackCmd != nil {
			if cmd != nil {
				cmd = tea.Batch(cmd, playbackCmd)
			} else {
				cmd = playbackCmd
			}
		}
	case queueInfoMsg:
		// Update the queue overlay with the new information
		m.queueOverlay.queueInfo = msg.info
		m.queueOverlay.lastError = msg.err
		m.queueOverlay.loading = false
		// Update dimensions based on current terminal size
		m.queueOverlay.width = m.lastWidth
		m.queueOverlay.height = m.lastHeight
	case searchResultsMsg:
		// Handle search results
		m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
			main := model.(mainContentModel)
			if msg.err != nil {
				// Error occurred during search - show empty results with error message
				main.searchResults = []daemon.Track{}
				main.searchQuery = fmt.Sprintf("Error: %v", msg.err)
				main.isSearchMode = true // Still show search mode to display the error
				main.selectedSong = 0
				main.scrollOffset = 0
			} else {
				// Update search results
				main.searchResults = msg.tracks
				main.searchQuery = msg.query
				main.isSearchMode = true
				main.selectedSong = 0 // Reset selection to first result
				main.scrollOffset = 0 // Reset scroll position
			}
			return main, nil
		})
		// Switch focus to main content to show search results or error
		m.currentFocus = focusMain
		m.updateFocus()
	case sizeCheckMsg:
		// Aggressive size check for yabai compatibility
		// Force immediate refresh to catch size changes
		return m, tea.Batch(
			cmd,
			checkTerminalSize(),
			measureTerminalSize(),
			tea.Sequence(tea.WindowSize(), tea.WindowSize()), // Double-check
		)
	case tea.WindowSizeMsg:
		// Always force an update for yabai compatibility, even if size appears the same
		prevWidth, prevHeight := m.lastWidth, m.lastHeight
		m.lastWidth = msg.Width
		m.lastHeight = msg.Height

		// Force boxer update - let bubbleboxer handle sizing properly
		// This is critical for yabai resize detection
		m.boxer.Update(msg)

		// Force a second update to ensure all components get the size message
		// This helps with yabai compatibility
		m.boxer.Update(msg)

		// Log size changes for debugging
		if prevWidth != msg.Width || prevHeight != msg.Height {
			fmt.Printf("\rTerminal size changed: %dx%d -> %dx%d\n", prevWidth, prevHeight, msg.Width, msg.Height)
		}
	case tea.KeyMsg:
		// Handle context menu navigation first
		if m.contextVisible {
			switch msg.String() {
			case "esc", "q":
				// Close context menu
				m.contextVisible = false
				m.contextMenu.visible = false
				return m, nil
			case "up", "k":
				// Navigate up in context menu
				if m.contextMenu.selectedOption > 0 {
					m.contextMenu.selectedOption--
				}
				return m, nil
			case "down", "j":
				// Navigate down in context menu
				if m.contextMenu.selectedOption < 2 { // 3 options total (0-2)
					m.contextMenu.selectedOption++
				}
				return m, nil
			case "enter":
				// Execute selected context menu option
				return m, m.executeContextMenuAction()
			default:
				// Ignore other keys when context menu is visible
				return m, nil
			}
		}

		// Handle queue overlay navigation
		if m.queueVisible {
			switch msg.String() {
			case "q", "esc":
				// Close queue overlay
				m.queueVisible = false
				m.queueOverlay.visible = false
				return m, nil
			case "u":
				// Refresh queue info
				m.queueOverlay.loading = true
				return m, fetchQueueInfo()
			case "up", "k":
				// Navigate up in queue (upcoming tracks only - excluding current)
				if m.queueOverlay.queueInfo != nil && len(m.queueOverlay.queueInfo.Tracks) > 0 {
					// Calculate minimum position for upcoming tracks (after current track)
					minPosition := 0
					if m.queueOverlay.queueInfo.CurrentPosition > 0 {
						minPosition = m.queueOverlay.queueInfo.CurrentPosition // First upcoming track (0-based)
					}

					if m.queueOverlay.selectedItem > minPosition {
						m.queueOverlay.selectedItem--
						// Update scroll offset if needed
						if m.queueOverlay.selectedItem < m.queueOverlay.scrollOffset {
							m.queueOverlay.scrollOffset = m.queueOverlay.selectedItem
						}
					}
				}
				return m, nil
			case "down", "j":
				// Navigate down in queue (upcoming tracks only)
				if m.queueOverlay.queueInfo != nil && len(m.queueOverlay.queueInfo.Tracks) > 0 {
					if m.queueOverlay.selectedItem < len(m.queueOverlay.queueInfo.Tracks)-1 {
						m.queueOverlay.selectedItem++
						// Update scroll offset if needed
						visibleTracks := 15 // Approximate visible tracks in overlay (accounting for header)
						if m.queueOverlay.selectedItem >= m.queueOverlay.scrollOffset+visibleTracks {
							m.queueOverlay.scrollOffset = m.queueOverlay.selectedItem - visibleTracks + 1
						}
					}
				}
				return m, nil
			case "enter":
				// Skip to selected song in queue
				if m.queueOverlay.queueInfo != nil && len(m.queueOverlay.queueInfo.Tracks) > 0 {
					// Use the selected item directly as the track index (0-based)
					if m.queueOverlay.selectedItem >= 0 && m.queueOverlay.selectedItem < len(m.queueOverlay.queueInfo.Tracks) {
						// Skip to the selected track using daemon (1-based indexing)
						// When playing from queue, we want to disable shuffle to maintain queue order
						d := daemon.Daemon{}
						go func() {
							// Temporarily disable shuffle for queue playback
							currentShuffle, shuffleErr := d.GetShuffle()
							if shuffleErr == nil && currentShuffle {
								d.SetShuffle(false)
							}

							err := d.SkipToQueuePosition(m.queueOverlay.selectedItem + 1) // Convert to 1-based
							if err != nil {
								fmt.Printf("Error skipping to track: %v\n", err)
							}

							// Keep shuffle disabled for queue playback
							// Don't restore it since we want the queue to play in order
						}()
						// Close overlay after action
						m.queueVisible = false
						m.queueOverlay.visible = false
					}
				}
				return m, nil
			default:
				// Ignore other keys when queue overlay is visible
				return m, nil
			}
		}

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
				// Get search text and perform search
				var searchQuery string
				m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
					sh := model.(searchHelpModel)
					searchQuery = strings.TrimSpace(sh.searchText)
					return sh, nil
				})

				// Only perform search if there's a query
				if searchQuery != "" {
					// Trigger search
					return m, fetchSearchResults(searchQuery)
				} else {
					// Empty search - exit search mode
					m.currentFocus = focusPlaylists
					m.updateFocus()
					return m, nil
				}
			case "esc":
				// Clear search and return to playlists
				m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
					sh := model.(searchHelpModel)
					sh.searchText = ""
					sh.cursorPos = 0
					return sh, nil
				})
				m.currentFocus = focusPlaylists
				m.updateFocus()
				return m, nil
			case " ":
				// Space key: toggle play/pause (even in search mode)
				d := daemon.Daemon{}
				go func() {
					err := d.TogglePlayPause()
					if err != nil {
						// Could add error handling here, maybe show in UI
						fmt.Printf("Error toggling play/pause: %v\n", err)
					}
				}()
				return m, nil
			default:
				// Forward all other key events to the search input for custom handling
				m.boxer.EditLeaf("searchHelp", func(model tea.Model) (tea.Model, error) {
					sh := model.(searchHelpModel)
					// The custom input handling is already done in the searchHelpModel.Update method
					// This will be handled by our custom search input logic
					updatedSh, inputCmd := sh.Update(msg)
					sh = updatedSh.(searchHelpModel)
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

		case "Q":
			// Toggle queue overlay with capital Q
			if m.queueVisible {
				m.queueVisible = false
				m.queueOverlay.visible = false
			} else {
				m.queueVisible = true
				m.queueOverlay.visible = true
				// Update overlay dimensions
				m.queueOverlay.width = m.lastWidth
				m.queueOverlay.height = m.lastHeight
				// Start loading queue info
				m.queueOverlay.loading = true
				return m, fetchQueueInfo()
			}
			return m, nil

		case "shift+k", "K":
			// Show context menu for currently selected song (only in main focus)
			if m.currentFocus == focusMain && m.selectedPlaylist != "" {
				// Get the currently selected song info and calculate position
				var selectedSong daemon.Track
				var selectedSongIndex int
				var menuX, menuY int

				m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
					main := model.(mainContentModel)
					selectedSongIndex = main.selectedSong

					// Calculate the position of the selected song row
					// Main content area position calculation
					// Get sidebar width from the boxer layout
					var sidebarWidth int
					if m.lastWidth <= 80 {
						sidebarWidth = m.lastWidth / 3
						if sidebarWidth < 25 {
							sidebarWidth = 25
						}
					} else if m.lastWidth <= 120 {
						sidebarWidth = 35
					} else if m.lastWidth <= 160 {
						sidebarWidth = 40
					} else {
						sidebarWidth = 45
					}

					// Calculate the Y position of the selected song
					headerLines := 3 // title + header + separator
					visibleSongRow := selectedSongIndex - main.scrollOffset
					songRowY := headerLines + visibleSongRow

					// Improved positioning logic
					// First, calculate preferred position (to the right of song name)
					preferredMenuX := sidebarWidth + 30 // Place further right, after song name column
					preferredMenuY := songRowY + 1      // +1 for base style margin

					// Menu dimensions (estimated)
					menuWidth := 16 // Width needed for "Add To Queue" + borders
					menuHeight := 5 // 3 options + 2 borders

					// Check boundaries and adjust if needed
					// X position: ensure menu doesn't go off right edge
					if preferredMenuX+menuWidth > m.lastWidth {
						// Try placing to the left of the song name instead
						preferredMenuX = sidebarWidth - menuWidth - 2
						// If that's still off-screen, place it at a safe position
						if preferredMenuX < 0 {
							preferredMenuX = 2 // Minimum padding from left edge
						}
					}

					// Y position: ensure menu doesn't go off bottom edge
					if preferredMenuY+menuHeight > m.lastHeight {
						// Move menu up so it fits
						preferredMenuY = m.lastHeight - menuHeight - 1
						// Ensure it doesn't go above the top either
						if preferredMenuY < 1 {
							preferredMenuY = 1
						}
					}

					menuX = preferredMenuX
					menuY = preferredMenuY

					return main, nil
				})

				// Get the song from the playlist cache
				if playlist, exists := m.playlistCache[m.selectedPlaylist]; exists {
					if selectedSongIndex >= 0 && selectedSongIndex < len(playlist.Tracks) {
						selectedSong = playlist.Tracks[selectedSongIndex]

						// Set up context menu
						m.contextMenu.targetSong = selectedSong
						m.contextMenu.targetPlaylist = m.selectedPlaylist
						m.contextMenu.targetSongIndex = selectedSongIndex
						m.contextMenu.selectedOption = 0 // Reset to first option
						m.contextMenu.visible = true
						m.contextMenu.width = m.lastWidth
						m.contextMenu.height = m.lastHeight

						// Position menu next to the selected song
						m.contextMenu.x = menuX
						m.contextMenu.y = menuY

						m.contextVisible = true
					}
				}
			}
			return m, nil

		case " ":
			// Space key: toggle play/pause (works in any focus area except search)
			if m.currentFocus != focusSearch {
				d := daemon.Daemon{}
				go func() {
					err := d.TogglePlayPause()
					if err != nil {
						// Could add error handling here, maybe show in UI
						fmt.Printf("Error toggling play/pause: %v\n", err)
					}
				}()
				return m, nil
			}

		case "s":
			// S key: toggle shuffle (works in any focus area except search)
			if m.currentFocus != focusSearch {
				d := daemon.Daemon{}
				go func() {
					err := d.ToggleShuffle()
					if err != nil {
						// Could add error handling here, maybe show in UI
						fmt.Printf("Error toggling shuffle: %v\n", err)
					}
				}()
				return m, nil
			}

		case "r":
			// R key: cycle repeat mode (works in any focus area except search)
			if m.currentFocus != focusSearch {
				d := daemon.Daemon{}
				go func() {
					err := d.CycleRepeatMode()
					if err != nil {
						// Could add error handling here, maybe show in UI
						fmt.Printf("Error cycling repeat mode: %v\n", err)
					}
				}()
				return m, nil
			}

		case "+", "=":
			// + key: volume up (works in any focus area except search)
			if m.currentFocus != focusSearch {
				d := daemon.Daemon{}
				go func() {
					// Get current volume first
					currentVol, err := d.GetVolume()
					if err != nil {
						fmt.Printf("Error getting volume: %v\n", err)
						return
					}

					// Increase by 10%, max at 100
					newVol := currentVol + 10
					if newVol > 100 {
						newVol = 100
					}

					err = d.SetVolume(newVol)
					if err != nil {
						fmt.Printf("Error setting volume: %v\n", err)
					}
				}()
				return m, nil
			}

		case "-":
			// - key: volume down (works in any focus area except search)
			if m.currentFocus != focusSearch {
				d := daemon.Daemon{}
				go func() {
					// Get current volume first
					currentVol, err := d.GetVolume()
					if err != nil {
						fmt.Printf("Error getting volume: %v\n", err)
						return
					}

					// Decrease by 10%, min at 0
					newVol := currentVol - 10
					if newVol < 0 {
						newVol = 0
					}

					err = d.SetVolume(newVol)
					if err != nil {
						fmt.Printf("Error setting volume: %v\n", err)
					}
				}()
				return m, nil
			}

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
				// Update the main content view and reset song selection
				m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
					main := model.(mainContentModel)
					main.currentPlaylist = m.selectedPlaylist
					main.selectedSong = 0 // Reset to first song
					main.scrollOffset = 0 // Reset scroll position
					main.isSearchMode = false // Exit search mode when viewing playlist
					return main, nil
				})
				// Automatically switch focus to main content for better UX
				m.currentFocus = focusMain
				m.updateFocus()
			} else if m.currentFocus == focusMain {
				// Check if we're in search mode or playlist mode
				var isSearchMode bool
				var selectedTrack daemon.Track
				var selectedSongIndex int
				
				m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
					main := model.(mainContentModel)
					isSearchMode = main.isSearchMode
					selectedSongIndex = main.selectedSong
					
					if isSearchMode && len(main.searchResults) > 0 {
						// Play selected search result
						if selectedSongIndex >= 0 && selectedSongIndex < len(main.searchResults) {
							selectedTrack = main.searchResults[selectedSongIndex]
						}
					}
					return main, nil
				})
				
				if isSearchMode {
					// Play the selected search result directly
					if selectedTrack.Name != "" {
						d := daemon.Daemon{}
						go func() {
							// Use PlaySongById if we have an ID, otherwise try by name/artist
							if selectedTrack.Id != "" {
								err := d.PlaySongById(selectedTrack.Id)
								if err != nil {
									fmt.Printf("Error playing song by ID: %v\n", err)
								}
							} else {
								// Fallback: try to find and play by name/artist
								fmt.Printf("Playing search result: %s by %s\n", selectedTrack.Name, selectedTrack.Artist)
								// Could implement additional logic here if needed
							}
						}()
					}
				} else if m.selectedPlaylist != "" {
					// Play song from playlist (original logic)
					d := daemon.Daemon{}
					go func() {
						err := d.PlaySongAtPosition(m.selectedPlaylist, selectedSongIndex+1)
						if err != nil {
							// Could add error handling here, maybe show in UI
							fmt.Printf("Error playing song: %v\n", err)
						}
					}()
				}
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
			} else if m.currentFocus == focusMain {
				m.updateSongSelection(-1)
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
			} else if m.currentFocus == focusMain {
				m.updateSongSelection(1)
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
		// No need to focus/blur since we're using custom input handling
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
		// Update cache references to current state
		main.playlistCache = &m.playlistCache
		main.playlistsLoading = &m.playlistsLoading
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

func (m *Model) updateSongSelection(direction int) {
	// Get the current main content model to check if we're in search mode
	var isSearchMode bool
	var searchResultCount int
	var playlistSongCount int
	
	m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
		main := model.(mainContentModel)
		isSearchMode = main.isSearchMode
		searchResultCount = len(main.searchResults)
		return main, nil
	})

	if isSearchMode {
		// Handle navigation in search results
		if searchResultCount == 0 {
			return // No search results to navigate
		}
		
		m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
			main := model.(mainContentModel)
			
			// Update selected song in search results
			newSelection := main.selectedSong + direction
			if newSelection < 0 {
				newSelection = 0
			} else if newSelection >= searchResultCount {
				newSelection = searchResultCount - 1
			}
			main.selectedSong = newSelection
			
			// Calculate visible tracks and update scroll offset for search results
			headerLines := 3 // title + header + separator
			visibleTracks := main.height - headerLines
			if visibleTracks < 1 {
				visibleTracks = 1
			}
			
			// Update scroll offset if needed
			if main.selectedSong < main.scrollOffset {
				// Song is above visible area, scroll up
				main.scrollOffset = main.selectedSong
			} else if main.selectedSong >= main.scrollOffset+visibleTracks {
				// Song is below visible area, scroll down
				main.scrollOffset = main.selectedSong - visibleTracks + 1
			}
			
			return main, nil
		})
		return
	}

	// Handle navigation in playlist mode (original logic)
	if m.selectedPlaylist == "" {
		return
	}

	// Get the current song count from cache
	if playlist, exists := m.playlistCache[m.selectedPlaylist]; exists {
		playlistSongCount = len(playlist.Tracks)
	} else {
		return // No tracks available
	}

	if playlistSongCount == 0 {
		return
	}

	m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
		main := model.(mainContentModel)

		// Update selected song
		newSelection := main.selectedSong + direction
		if newSelection < 0 {
			newSelection = 0
		} else if newSelection >= playlistSongCount {
			newSelection = playlistSongCount - 1
		}
		main.selectedSong = newSelection

		// Calculate visible tracks and update scroll offset
		headerLines := 3 // title + header + separator
		visibleTracks := main.height - headerLines
		if visibleTracks < 1 {
			visibleTracks = 1
		}

		// Update scroll offset if needed
		if main.selectedSong < main.scrollOffset {
			// Song is above visible area, scroll up
			main.scrollOffset = main.selectedSong
		} else if main.selectedSong >= main.scrollOffset+visibleTracks {
			// Song is below visible area, scroll down
			main.scrollOffset = main.selectedSong - visibleTracks + 1
		}

		return main, nil
	})
}

// executeContextMenuAction executes the selected context menu action
func (m *Model) executeContextMenuAction() tea.Cmd {
	// Close context menu first
	m.contextVisible = false
	m.contextMenu.visible = false

	// Execute the selected action
	switch contextMenuOption(m.contextMenu.selectedOption) {
	case contextPlay:
		// Play: Clear queue and play the selected song
		return func() tea.Msg {
			d := daemon.Daemon{}
			go func() {
				err := d.PlaySongAtPosition(m.contextMenu.targetPlaylist, m.contextMenu.targetSongIndex+1)
				if err != nil {
					fmt.Printf("Error playing song: %v\n", err)
				}
			}()
			return nil
		}
	case contextAddToQueue:
		// Add To Queue: Append to end of queue
		return func() tea.Msg {
			d := daemon.Daemon{}
			go func() {
				err := d.AddToQueue(m.contextMenu.targetSong)
				if err != nil {
					fmt.Printf("Error adding song to queue: %v\n", err)
				} else {
					fmt.Printf("✅ Added '%s' by %s to queue\n",
						m.contextMenu.targetSong.Name, m.contextMenu.targetSong.Artist)
				}
			}()
			return nil
		}
	default:
		return nil
	}
}

func (m Model) View() string {
	// Create a temporary model to update focus state
	tempModel := m
	tempModel.updateFocus()

	// Get the base layout from bubbleboxer
	baseView := tempModel.boxer.View()

	// If queue overlay is visible, render it on top
	if m.queueVisible {
		// Update the queue overlay dimensions to match current terminal size
		m.queueOverlay.width = m.lastWidth
		m.queueOverlay.height = m.lastHeight
		// Render the queue overlay on top of the base view
		queueOverlayView := m.queueOverlay.View()
		if queueOverlayView != "" {
			// The queue overlay should completely cover the base view
			return queueOverlayView
		}
	}

	// If context menu is visible, render it on top of existing content
	if m.contextVisible {
		// Update the context menu dimensions to match current terminal size
		m.contextMenu.width = m.lastWidth
		m.contextMenu.height = m.lastHeight
		// Render the context menu overlay on top of the base view
		contextMenuView := m.contextMenu.View()
		if contextMenuView != "" {
			// The context menu should completely cover the base view
			return contextMenuView
		}
	}

	// Use bubbleboxer to render the layout
	return baseStyle.Render(baseView)
}


// getContentLine returns the content for a specific line in the context menu
func (m contextMenuModel) getContentLine(lineIndex int, maxWidth int) string {
	// Song information section
	if lineIndex == 0 {
		// Song title
		songTitle := fmt.Sprintf(" 🎵 %s", m.targetSong.Name)
		if len(songTitle) > maxWidth {
			songTitle = songTitle[:maxWidth-3] + "..."
		}
		return songTitle
	}
	if lineIndex == 1 {
		// Artist
		artistLine := fmt.Sprintf(" 🎤 %s", m.targetSong.Artist)
		if len(artistLine) > maxWidth {
			artistLine = artistLine[:maxWidth-3] + "..."
		}
		return artistLine
	}
	if lineIndex == 2 {
		// Album
		albumLine := fmt.Sprintf(" 💿 %s", m.targetSong.Album)
		if len(albumLine) > maxWidth {
			albumLine = albumLine[:maxWidth-3] + "..."
		}
		return albumLine
	}
	if lineIndex == 3 {
		// Separator
		return " " + strings.Repeat("─", maxWidth-2)
	}
	if lineIndex == 4 {
		// Empty line for spacing
		return ""
	}

	// Options section
	options := []string{"Play", "Add To Queue"}
	optionIndex := lineIndex - 5 // Offset for song info + separator + spacing

	if optionIndex >= 0 && optionIndex < len(options) {
		var prefix string
		if optionIndex == m.selectedOption {
			prefix = " ► " // Use arrow for selection
		} else {
			prefix = "   " // Three spaces for alignment
		}

		return prefix + options[optionIndex]
	}

	// Empty line
	return ""
}

// Run starts the TUI application
func Run() error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in TUI: %v\n", r)
			// You can add stack trace here if needed
			// debug.PrintStack()
			os.Exit(1)
		}
	}()

	fmt.Println("Starting TUI application...")

	// Create model with error handling
	model := NewModel()
	fmt.Println("Model created successfully")

	// Initialize program
	p := tea.NewProgram(model, tea.WithAltScreen())
	fmt.Println("Program initialized successfully")

	// Run program
	_, err := p.Run()
	if err != nil {
		fmt.Printf("Program run error: %v\n", err)
	}
	return err
}
