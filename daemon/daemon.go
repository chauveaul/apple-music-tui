package daemon

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Daemon struct{}

type Track struct {
	Id       string
	Name     string
	Artist   string
	Album    string
	Duration string
}

type Playlist struct {
	Name   string
	Tracks []Track
}

type QueueInfo struct {
	QueueName       string
	Tracks          []Track
	CurrentTrack    *Track
	CurrentPosition int // Position of currently playing track (1-based)
	TotalTracks     int
}

func run_script(script string) error {
	return exec.Command("osascript", "-e", script).Run()
}

func get_script_output(script string) ([]byte, error) {
	return exec.Command("osascript", "-e", script).Output()
}

func parse_queue_output(out []byte) (*QueueInfo, error) {
	parts := strings.Split(string(out), "|")
	if len(parts) < 7 {
		return nil, fmt.Errorf("Invalid output format")
	}

	queueName := parts[0]
	totalTracks, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid track count %w", err)
	}

	currentPosition, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("Invalid current position %w", err)
	}

	var currentTrack *Track
	if parts[3] != "" {
		duration := parts[6]
		currentTrack = &Track{
			Name:     parts[3],
			Artist:   parts[4],
			Album:    parts[5],
			Duration: duration,
		}
	}

	var tracks []Track
	if len(parts) > 7 && parts[7] != "" {
		trackStrings := strings.Split(parts[7], "||")
		for _, trackStr := range trackStrings {
			trackParts := strings.Split(trackStr, "~")
			if len(trackParts) != 5 {
				continue
			}

			duration := trackParts[3]
			track := Track{
				Name:     trackParts[0],
				Artist:   trackParts[1],
				Album:    trackParts[2],
				Duration: duration,
			}
			tracks = append(tracks, track)
		}
	}

	return &QueueInfo{
		QueueName:       queueName,
		Tracks:          tracks,
		CurrentTrack:    currentTrack,
		CurrentPosition: currentPosition,
		TotalTracks:     totalTracks,
	}, nil
}

func (d *Daemon) Play() error {
	script := `tell application "Music" to play`
	return run_script(script)
}

func (d *Daemon) PlaySongById(id string) error {
	script := fmt.Sprintf(`tell application "Music" to play (some track whose persistent ID is "%s")`, id)
	return run_script(script)
}

func (d *Daemon) PlaySongInPlaylist(songName, playlistName string) error {
	script := fmt.Sprintf(`tell application "Music" to play (some track of playlist "%s" whose name is "%s")`, playlistName, songName)
	return run_script(script)
}

// PlaySongAtPosition plays a song at a specific position (1-based) in a playlist
func (d *Daemon) PlaySongAtPosition(playlistName string, position int) error {
	// Escape quotes in playlist name
	escapedPlaylistName := strings.ReplaceAll(playlistName, `"`, `\"`)
	
	script := fmt.Sprintf(`
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		-- Try to find the playlist
		set targetPlaylist to playlist "%s"
		set trackCount to count of tracks of targetPlaylist
		
		if %d > trackCount then
			return "ERROR: Track position " & %d & " exceeds playlist length of " & trackCount
		end if
		
		if %d < 1 then
			return "ERROR: Invalid track position: " & %d
		end if
		
		-- Store current shuffle state
		set originalShuffle to shuffle enabled
		
		-- Turn off shuffle to ensure correct track order
		set shuffle enabled to false
		
		-- Play the entire playlist from the beginning
		play targetPlaylist
		
		-- Wait a moment for playback to start
		delay 0.5
		
		-- Skip to the desired position if not already at track 1
		if %d > 1 then
			repeat (%d - 1) times
				next track
				delay 0.1
			end repeat
		end if
		
		-- Restore original shuffle state
		set shuffle enabled to originalShuffle
		
		return "SUCCESS: Playing track " & %d & " from playlist " & "%s" & " with remaining tracks in queue"
		
	on error errMsg
		return "ERROR: " & errMsg
	end try
end tell
	`, escapedPlaylistName, position, position, position, position, position, position, position, escapedPlaylistName)
	
	out, err := get_script_output(script)
	if err != nil {
		return fmt.Errorf("AppleScript execution failed: %w", err)
	}
	
	output := strings.TrimSpace(string(out))
	if strings.HasPrefix(output, "ERROR:") {
		return fmt.Errorf("AppleScript error: %s", output[7:]) // Remove "ERROR: " prefix
	}
	
	if !strings.HasPrefix(output, "SUCCESS:") {
		return fmt.Errorf("unexpected AppleScript output: %s", output)
	}
	
	return nil
}

func (d *Daemon) Pause() error {
	script := `tell application "Music" to pause`
	return run_script(script)
}

// TogglePlayPause toggles between play and pause based on current state
func (d *Daemon) TogglePlayPause() error {
	script := `
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		set playerState to player state as string
		
		if playerState is "playing" then
			pause
			return "PAUSED"
		else
			play
			return "PLAYING"
		end if
		
	on error errMsg
		return "ERROR: " & errMsg
	end try
end tell
	`
	
	out, err := get_script_output(script)
	if err != nil {
		return fmt.Errorf("AppleScript execution failed: %w", err)
	}
	
	output := strings.TrimSpace(string(out))
	if strings.HasPrefix(output, "ERROR:") {
		return fmt.Errorf("AppleScript error: %s", output[7:])
	}
	
	return nil
}

func (d *Daemon) Stop() error {
	script := `tell application "Music" to stop`
	return run_script(script)
}

func (d *Daemon) NextTrack() error {
	script := `tell application "Music" to next track`
	return run_script(script)
}

func (d *Daemon) PreviousTrack() error {
	script := `tell application "Music" to previous track`
	return run_script(script)
}

func (d *Daemon) SetVolume(volume int) error {
	script := fmt.Sprintf(`tell application "Music" to set sound volume to %d`, volume)
	return run_script(script)
}

func (d *Daemon) GetVolume() (int, error) {
	script := `tell application "Music" to sound volume`
	out, err := get_script_output(script)
	if err != nil {
		return 0, err
	}
	vol, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return int(vol), nil
}

func (d *Daemon) SetRepeat(repeatType string) error {
	script := fmt.Sprintf(`tell application "Music" to set song repeat to %s`, repeatType)
	return run_script(script)
}

func (d *Daemon) GetRepeatMode() (string, error) {
	script := `tell application "Music" to get song repeat`
	out, err := get_script_output(script)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *Daemon) SetShuffle(isShuffle bool) error {
	val := "false"
	if isShuffle {
		val = "true"
	}
	script := fmt.Sprintf(`tell application "Music" to set shuffle enabled to %s`, val)
	return run_script(script)
}

func (d *Daemon) GetShuffle() (bool, error) {
	script := `tell application "Music" to get shuffle enabled`
	out, err := get_script_output(script)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

type PlaybackStatus struct {
	Track        Track
	IsPlaying    bool
	Position     float64 // Current position in seconds
	Duration     float64 // Total duration in seconds
	Volume       int
	Shuffle      bool
	RepeatMode   string
	PlayerState  string // "playing", "paused", "stopped"
}

func (d *Daemon) GetCurrentTrack() (Track, error) {
	script := `tell application "Music" to get database ID of current track & "||" & name of current track & "||" & artist of current track & "||" & album of current track & "||" & duration of current track as string`
	out, err := get_script_output(script)
	if err != nil {
		return Track{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "||")
	if len(parts) < 5 {
		return Track{}, errors.New("Invalid output from get_current_track()")
	}
	return Track{Id: parts[0], Name: parts[1], Artist: parts[2], Album: parts[3], Duration: parts[4]}, nil
}

// GetPlaybackStatus returns comprehensive playback information
func (d *Daemon) GetPlaybackStatus() (PlaybackStatus, error) {
	script := `
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		-- Get player state
		set playerState to player state as string
		
		-- Get current track info if playing
		set trackName to ""
		set trackArtist to ""
		set trackAlbum to ""
		set trackDuration to 0
		set trackId to ""
		set currentPos to 0
		
		if playerState is not "stopped" then
			try
				set currentTrack to current track
				set trackName to name of currentTrack
				set trackArtist to artist of currentTrack
				set trackAlbum to album of currentTrack
				set trackDuration to duration of currentTrack
				set trackId to database ID of currentTrack
				set currentPos to player position
			end try
		end if
		
		-- Get other playback settings
		set currentVolume to sound volume
		set isShuffled to shuffle enabled
		set repeatSetting to song repeat as string
		
		-- Build result string
		return playerState & "|" & trackId & "|" & trackName & "|" & trackArtist & "|" & trackAlbum & "|" & trackDuration & "|" & currentPos & "|" & currentVolume & "|" & isShuffled & "|" & repeatSetting
		
	on error errMsg
		return "ERROR: " & errMsg
	end try
end tell
	`
	
	out, err := get_script_output(script)
	if err != nil {
		return PlaybackStatus{}, fmt.Errorf("AppleScript execution failed: %w", err)
	}
	
	output := strings.TrimSpace(string(out))
	if strings.HasPrefix(output, "ERROR:") {
		return PlaybackStatus{}, fmt.Errorf("AppleScript error: %s", output[7:])
	}
	
	parts := strings.Split(output, "|")
	if len(parts) < 10 {
		return PlaybackStatus{}, fmt.Errorf("invalid playback status output: expected 10 parts, got %d", len(parts))
	}
	
	// Parse the response
	playerState := parts[0]
	trackId := parts[1]
	trackName := parts[2]
	trackArtist := parts[3]
	trackAlbum := parts[4]
	
	// Parse numeric values
	trackDuration, _ := strconv.ParseFloat(parts[5], 64)
	currentPos, _ := strconv.ParseFloat(parts[6], 64)
	volume, _ := strconv.Atoi(parts[7])
	isShuffled := parts[8] == "true"
	repeatMode := parts[9]
	
	return PlaybackStatus{
		Track: Track{
			Id:       trackId,
			Name:     trackName,
			Artist:   trackArtist,
			Album:    trackAlbum,
			Duration: parts[5], // Keep as string for compatibility
		},
		IsPlaying:   playerState == "playing",
		Position:    currentPos,
		Duration:    trackDuration,
		Volume:      volume,
		Shuffle:     isShuffled,
		RepeatMode:  repeatMode,
		PlayerState: playerState,
	}, nil
}

func (d *Daemon) PlayPlaylist(playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to play playlist "%s"`, playlist.Name)
	return run_script(script)
}

func (d *Daemon) AddSongToPlaylist(song Track, playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to duplicate (first track whose name is "%s") to playlist "%s"`, song.Name, playlist.Name)
	return run_script(script)
}

func (d *Daemon) RemoveSongFromPlaylist(song Track, playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to delete (first track whose name is "%s") of playlist "%s"`, song.Name, playlist.Name)
	return run_script(script)
}

func (d *Daemon) GetPlaylist(playlistName string) (Playlist, error) {
	// Fetch all track data in a single AppleScript call (much faster!)
	script := fmt.Sprintf(`
tell application "Music"
	if it is not running then
		return "Music app is not running"
	end if
	
	try
		set targetPlaylist to playlist "%s"
		set trackCount to count of tracks of targetPlaylist
		
		if trackCount = 0 then
			return "NO_TRACKS"
		end if
		
		set outputResult to ""
		
		-- Get all tracks in one loop
		repeat with i from 1 to trackCount
			set currentTrack to track i of targetPlaylist
			set trackName to name of currentTrack
			set trackArtist to artist of currentTrack
			set trackAlbum to album of currentTrack
			set trackDuration to duration of currentTrack as string
			
			set outputResult to outputResult & trackName & "~" & trackArtist & "~" & trackAlbum & "~" & trackDuration
			if i < trackCount then set outputResult to outputResult & "||"
		end repeat
		
		return outputResult
		
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, playlistName)

	out, err := get_script_output(script)
	if err != nil {
		return Playlist{}, err
	}

	outputStr := strings.TrimSpace(string(out))
	if strings.HasPrefix(outputStr, "Error:") {
		return Playlist{}, fmt.Errorf("AppleScript error: %s", outputStr)
	}
	if strings.HasPrefix(outputStr, "Music app is not running") {
		return Playlist{}, fmt.Errorf("Music app is not running")
	}
	if outputStr == "NO_TRACKS" {
		return Playlist{Name: playlistName, Tracks: []Track{}}, nil
	}

	// Parse the track data
	tracks := make([]Track, 0)
	if outputStr != "" {
		trackStrings := strings.Split(outputStr, "||")
		for _, trackStr := range trackStrings {
			trackParts := strings.Split(trackStr, "~")
			if len(trackParts) == 4 {
				tracks = append(tracks, Track{
					Name:     trackParts[0],
					Artist:   trackParts[1],
					Album:    trackParts[2],
					Duration: trackParts[3],
				})
			}
		}
	}

	return Playlist{Name: playlistName, Tracks: tracks}, nil
}

func (d *Daemon) GetAllPlaylistNames() ([]string, error) {
	script := `tell application "Music" to get name of playlists`
	out, err := get_script_output(script)
	if err != nil {
		return []string{}, err
	}
	return strings.Split(strings.TrimSpace(string(out)), ", "), nil
}

func (d *Daemon) GetAllPlaylists() ([]Playlist, error) {
	//TODO: Cache these in local storage and on run, check if there are changes by looking at the length of names
	names, err := d.GetAllPlaylistNames()
	if err != nil {
		return []Playlist{}, err
	}
	playlists := make([]Playlist, 0, len(names))
	for _, name := range names[2:] {
		playlist, err := d.GetPlaylist(name)
		if err != nil {
			continue
		}
		playlists = append(playlists, playlist)
	}
	return playlists, nil
}

func (d *Daemon) GetQueueInfo() (*QueueInfo, error) {
	script := `
tell application "Music"
	if it is not running then
		return "Music app is not running"
	end if
	
	try
		set currentQueue to current playlist
		set queueName to name of currentQueue
		set trackCount to count of tracks of currentQueue
		
		-- Get current track info
		set currentTrackName to ""
		set currentTrackArtist to ""
		set currentTrackAlbum to ""
		set currentTrackDuration to 0
		set currentPosition to 0
		
		try
			set currentTrack to current track
			set currentTrackName to name of currentTrack
			set currentTrackArtist to artist of currentTrack
			set currentTrackAlbum to album of currentTrack
			set currentTrackDuration to duration of currentTrack
			
			-- Find position of current track in queue
			repeat with i from 1 to trackCount
				if track i of currentQueue is currentTrack then
					set currentPosition to i
					exit repeat
				end if
			end repeat
		end try
		
		-- Build result string
		set outputResult to queueName & "|" & trackCount & "|" & currentPosition & "|"
		set outputResult to outputResult & currentTrackName & "|" & currentTrackArtist & "|" & currentTrackAlbum & "|" & currentTrackDuration & "|"
		
		-- Get all tracks in queue
		repeat with i from 1 to trackCount
			set queueTrack to track i of currentQueue
			set trackName to name of queueTrack
			set trackArtist to artist of queueTrack
			set trackAlbum to album of queueTrack
			set trackDuration to duration of queueTrack
			
			set outputResult to outputResult & trackName & "~" & trackArtist & "~" & trackAlbum & "~" & trackDuration & "~" & i
			if i < trackCount then set outputResult to outputResult & "||"
		end repeat
		
		return outputResult
		
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`

	out, err := get_script_output(script)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(string(out), "Error:") || strings.HasPrefix(string(out), "Music app is not running") {
		return nil, fmt.Errorf("Encountered an error in apple script %s", string(out))
	}
	return parse_queue_output(out)
}

// ToggleShuffle toggles the shuffle setting
func (d *Daemon) ToggleShuffle() error {
	currentShuffle, err := d.GetShuffle()
	if err != nil {
		return fmt.Errorf("failed to get current shuffle state: %w", err)
	}
	return d.SetShuffle(!currentShuffle)
}

// CycleRepeatMode cycles through repeat modes: off -> all -> one -> off
func (d *Daemon) CycleRepeatMode() error {
	currentMode, err := d.GetRepeatMode()
	if err != nil {
		return fmt.Errorf("failed to get current repeat mode: %w", err)
	}
	
	var nextMode string
	switch strings.ToLower(currentMode) {
	case "off":
		nextMode = "all"
	case "all":
		nextMode = "one"
	case "one":
		nextMode = "off"
	default:
		// Default to "all" if we get an unexpected mode
		nextMode = "all"
	}
	
	return d.SetRepeat(nextMode)
}

func (d *Daemon) AddToQueue(track Track) error {
	// Build search criteria - we'll search by name and artist primarily
	searchQuery := track.Name
	if track.Artist != "" {
		searchQuery += " " + track.Artist
	}

	// Escape quotes in the search query and track details
	searchQuery = strings.ReplaceAll(searchQuery, `"`, `\"`)
	trackName := strings.ReplaceAll(track.Name, `"`, `\"`)
	trackArtist := strings.ReplaceAll(track.Artist, `"`, `\"`)

	script := fmt.Sprintf(`
	tell application "Music"
		if it is not running then
			error "Music app is not running"
		end if
		
		try
			-- Search your local library
			set localTracks to (tracks whose name is "%s" and artist is "%s")
			
			if (count of localTracks) = 0 then
				error "Track not found in your library"
			end if
			
			set targetTrack to item 1 of localTracks
			
			-- Check if playlist exists, create if it doesn't
			try
				set targetPlaylist to user playlist "amtui Queue"
			on error
				-- Create the playlist
				set targetPlaylist to (make new user playlist with properties {name:"amtui Queue"})
			end try
			
			-- Add track using duplicate instead of add
			duplicate targetTrack to targetPlaylist
			
			return "Added: " & (name of targetTrack) & " by " & (artist of targetTrack) & " to amtui Queue"
			
		on error errMsg
			error "Failed to add track: " & errMsg
		end try
	end tell
	`, trackName, trackArtist)
	out, err := get_script_output(script)
	if err != nil {
		return fmt.Errorf("failed to add track to queue: %w", err)
	}

	if strings.HasPrefix(string(out), "Failed to add track:") {
		return fmt.Errorf("Failed to add to queue with err %s", string(out))
	}

	fmt.Printf("✅ %s\n", out)
	return nil
}
