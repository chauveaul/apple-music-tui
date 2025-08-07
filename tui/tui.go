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

// New message type for full playlist data with tracks
type allPlaylistsMsg struct {
	playlists map[string]daemon.Playlist // Map from playlist name to playlist data
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
	// Add references to the main model's cache and loading state
	playlistCache    *map[string]daemon.Playlist
	playlistsLoading *bool
	// Song selection state
	selectedSong int
	scrollOffset int
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
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter play song • Space play/pause • s shuffle • r repeat", focusName[m.currentFocus])
	} else if m.currentFocus == focusSearch {
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter select • / search • Space play/pause • s shuffle • r repeat", focusName[m.currentFocus])
	} else {
		instructions = fmt.Sprintf("Focus: %s | 'q' quit • Tab cycle • Ctrl+W+hjkl vim nav • ↑↓ navigate • Enter select • Space play/pause • s shuffle • r repeat", focusName[m.currentFocus])
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

	// Initialize the cache and loading state
	playlistCache := make(map[string]daemon.Playlist)
	playlistsLoading := true

	// Create leaf nodes
	searchHelpLeaf, _ := boxer.CreateLeaf("searchHelp", searchHelpModel{width: 30, height: 4, textInput: ti})
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
					return main, nil
				})
			} else if m.currentFocus == focusMain {
				// Play the selected song
				if m.selectedPlaylist != "" {
					var selectedSongIndex int
					m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
						main := model.(mainContentModel)
						selectedSongIndex = main.selectedSong
						return main, nil
					})

					// Play song at position (1-based indexing for AppleScript)
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
	// Only proceed if we have a selected playlist
	if m.selectedPlaylist == "" {
		return
	}

	// Get the current song count from cache
	var songCount int
	if playlist, exists := m.playlistCache[m.selectedPlaylist]; exists {
		songCount = len(playlist.Tracks)
	} else {
		return // No tracks available
	}

	if songCount == 0 {
		return
	}

	m.boxer.EditLeaf("main", func(model tea.Model) (tea.Model, error) {
		main := model.(mainContentModel)

		// Update selected song
		newSelection := main.selectedSong + direction
		if newSelection < 0 {
			newSelection = 0
		} else if newSelection >= songCount {
			newSelection = songCount - 1
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
