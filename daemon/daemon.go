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
	if len(parts) > 7 {
		// Tracks are in parts[7], parts[9], parts[11], etc. (every odd index after 7)
		// because the AppleScript uses || as separator, which creates empty parts when split by |
		for i := 7; i < len(parts); i += 2 {
			if parts[i] == "" {
				continue
			}
			trackParts := strings.Split(parts[i], "~")
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
// and creates/updates the amtui Queue with all tracks from the playlist
func (d *Daemon) PlaySongAtPosition(playlistName string, position int) error {
	// Get the target song info before creating the queue
	playlist, err := d.GetPlaylist(playlistName)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}
	if position < 1 || position > len(playlist.Tracks) {
		return fmt.Errorf("invalid position %d for playlist with %d tracks", position, len(playlist.Tracks))
	}
	targetTrack := playlist.Tracks[position-1] // Convert to 0-based index
	
	// Create/update the queue with the selected song at the top, followed by shuffled remaining tracks
	if err := d.CreateOrUpdateQueueWithSelectedFirst(playlistName, position); err != nil {
		return fmt.Errorf("failed to create queue from playlist: %w", err)
	}
	
	// Now play the queue from the beginning (selected song is at position 1)
	script := fmt.Sprintf(`
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		-- Use the amtui Queue playlist
		set targetPlaylist to user playlist "amtui Queue"
		
		-- Disable shuffle to maintain queue order
		set shuffle enabled to false
		
		-- Play the queue from the beginning (selected song is now at position 1)
		play targetPlaylist
		
		return "SUCCESS: Playing track \"" & "%s" & "\" from shuffled queue (source: %s)"
		
	on error errMsg
		return "ERROR: " & errMsg
	end try
end tell
	`, strings.ReplaceAll(targetTrack.Name, `"`, `\"`), playlistName)
	
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

// CreateOrUpdateQueue creates or updates the amtui Queue playlist with tracks from the specified playlist
// If shuffle is enabled, it will shuffle the tracks before adding them to the queue
func (d *Daemon) CreateOrUpdateQueue(sourcePlaylist string) error {
	// Escape quotes in playlist name
	escapedSourcePlaylist := strings.ReplaceAll(sourcePlaylist, `"`, `\"`)
	
	script := fmt.Sprintf(`
	tell application "Music"
		if it is not running then
			error "Music app is not running"
		end if
		
		try
			-- Check if source playlist exists
			set sourcePlaylist to playlist "%s"
			set sourceTracks to tracks of sourcePlaylist
			set trackCount to count of sourceTracks
			
			-- Check if shuffle is enabled
			set isShuffled to shuffle enabled
			
			-- Check if amtui Queue exists, create if it doesn't
			try
				set queuePlaylist to user playlist "amtui Queue"
				-- Clear existing tracks from queue
				delete tracks of queuePlaylist
			on error
				-- Create the playlist if it doesn't exist
				set queuePlaylist to (make new user playlist with properties {name:"amtui Queue"})
			end try
			
			if isShuffled then
				-- Create a list of track indices for shuffling
				set trackIndices to {}
				repeat with i from 1 to trackCount
					set end of trackIndices to i
				end repeat
				
				-- Shuffle the indices using Fisher-Yates algorithm
				repeat with i from trackCount to 2 by -1
					set j to (random number from 1 to i)
					set temp to item i of trackIndices
					set item i of trackIndices to item j of trackIndices
					set item j of trackIndices to temp
				end repeat
				
				-- Add tracks in shuffled order
				repeat with shuffledIndex in trackIndices
					set sourceTrack to track shuffledIndex of sourcePlaylist
					duplicate sourceTrack to queuePlaylist
				end repeat
			else
				-- Add all tracks in original order
				repeat with sourceTrack in sourceTracks
					duplicate sourceTrack to queuePlaylist
				end repeat
			end if
			
			if isShuffled then
				return "SUCCESS: Created shuffled amtui Queue with " & trackCount & " tracks from " & "%s"
			else
				return "SUCCESS: Created amtui Queue with " & trackCount & " tracks from " & "%s"
			end if
			
		on error errMsg
			error "Failed to create queue: " & errMsg
	end try
end tell
	`, escapedSourcePlaylist, escapedSourcePlaylist, escapedSourcePlaylist)
	
	out, err := get_script_output(script)
	if err != nil {
		return fmt.Errorf("AppleScript execution failed: %w", err)
	}
	
	output := strings.TrimSpace(string(out))
	if strings.HasPrefix(output, "Failed to create queue:") {
		return fmt.Errorf("Queue creation failed: %s", output[23:]) // Remove "Failed to create queue: " prefix
	}
	
	if !strings.HasPrefix(output, "SUCCESS:") {
		return fmt.Errorf("unexpected AppleScript output: %s", output)
	}
	
	return nil
}

// CreateOrUpdateQueueWithSelectedFirst creates a queue with the selected song first, followed by shuffled remaining tracks
func (d *Daemon) CreateOrUpdateQueueWithSelectedFirst(sourcePlaylist string, selectedPosition int) error {
	// Escape quotes in playlist name
	escapedSourcePlaylist := strings.ReplaceAll(sourcePlaylist, `"`, `\"`)
	
	script := fmt.Sprintf(`
	tell application "Music"
		if it is not running then
			error "Music app is not running"
		end if
		
		try
			-- Check if source playlist exists
			set sourcePlaylist to playlist "%s"
			set sourceTracks to tracks of sourcePlaylist
			set trackCount to count of sourceTracks
			
			if %d < 1 or %d > trackCount then
				error "Invalid selected position: " & %d
			end if
			
			-- Get the selected track
			set selectedTrack to track %d of sourcePlaylist
			
			-- Check if amtui Queue exists, create if it doesn't
			try
				set queuePlaylist to user playlist "amtui Queue"
				-- Clear existing tracks from queue
				delete tracks of queuePlaylist
			on error
				-- Create the playlist if it doesn't exist
				set queuePlaylist to (make new user playlist with properties {name:"amtui Queue"})
			end try
			
			-- First, add the selected track at the top
			duplicate selectedTrack to queuePlaylist
			
			-- Create a list of remaining track indices (excluding the selected one)
			set remainingIndices to {}
			repeat with i from 1 to trackCount
				if i is not %d then
					set end of remainingIndices to i
				end if
			end repeat
			
			-- Shuffle the remaining indices using Fisher-Yates algorithm
			set remainingCount to count of remainingIndices
			repeat with i from remainingCount to 2 by -1
				set j to (random number from 1 to i)
				set temp to item i of remainingIndices
				set item i of remainingIndices to item j of remainingIndices
				set item j of remainingIndices to temp
			end repeat
			
			-- Add the shuffled remaining tracks
			repeat with shuffledIndex in remainingIndices
				set sourceTrack to track shuffledIndex of sourcePlaylist
				duplicate sourceTrack to queuePlaylist
			end repeat
			
			return "SUCCESS: Created amtui Queue with selected song first, followed by " & (trackCount - 1) & " shuffled tracks from " & "%s"
			
		on error errMsg
			error "Failed to create queue: " & errMsg
	end try
end tell
	`, escapedSourcePlaylist, selectedPosition, selectedPosition, selectedPosition, selectedPosition, selectedPosition, escapedSourcePlaylist)
	
	out, err := get_script_output(script)
	if err != nil {
		return fmt.Errorf("AppleScript execution failed: %w", err)
	}
	
	output := strings.TrimSpace(string(out))
	if strings.HasPrefix(output, "Failed to create queue:") {
		return fmt.Errorf("Queue creation failed: %s", output[23:]) // Remove "Failed to create queue: " prefix
	}
	
	if !strings.HasPrefix(output, "SUCCESS:") {
		return fmt.Errorf("unexpected AppleScript output: %s", output)
	}
	
	return nil
}

// PlayQueuePlaylist plays the amtui Queue playlist and optionally creates it from a source playlist
func (d *Daemon) PlayQueuePlaylist(sourcePlaylist string) error {
	// First create/update the queue
	if err := d.CreateOrUpdateQueue(sourcePlaylist); err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}
	
	// Now play the queue playlist
	script := `
	tell application "Music"
		if it is not running then
			return "ERROR: Music app is not running"
		end if
		
		try
			set queuePlaylist to user playlist "amtui Queue"
			play queuePlaylist
			return "SUCCESS: Playing amtui Queue"
			
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
		return fmt.Errorf("AppleScript error: %s", output[7:]) // Remove "ERROR: " prefix
	}
	
	if !strings.HasPrefix(output, "SUCCESS:") {
		return fmt.Errorf("unexpected AppleScript output: %s", output)
	}
	
	return nil
}

// SkipToQueuePosition skips to a specific position (1-based) in the current queue
func (d *Daemon) SkipToQueuePosition(position int) error {
	script := fmt.Sprintf(`
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		set currentQueue to current playlist
		set trackCount to count of tracks of currentQueue
		
		if %d > trackCount then
			return "ERROR: Position " & %d & " exceeds queue length of " & trackCount
		end if
		
		if %d < 1 then
			return "ERROR: Invalid position: " & %d
		end if
		
		-- Get the target track at the specified position
		set targetTrack to track %d of currentQueue
		
		-- Play the specific track
		play targetTrack
		
		return "SUCCESS: Skipped to position " & %d & " in queue"
		
	on error errMsg
		return "ERROR: " & errMsg
	end try
end tell
	`, position, position, position, position, position, position)
	
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

// CleanupQueue removes tracks from the amtui Queue that have already been played
// This keeps the queue showing only upcoming tracks
func (d *Daemon) CleanupQueue() error {
	script := `
tell application "Music"
	if it is not running then
		return "ERROR: Music app is not running"
	end if
	
	try
		-- Check if we're currently playing from amtui Queue
		set currentPlaylistName to name of current playlist
		if currentPlaylistName is not "amtui Queue" then
			return "INFO: Not playing from amtui Queue, no cleanup needed"
		end if
		
		-- Get the amtui Queue playlist
		set queuePlaylist to user playlist "amtui Queue"
		set queueTracks to tracks of queuePlaylist
		set totalTracks to count of queueTracks
		
		if totalTracks = 0 then
			return "INFO: Queue is empty, no cleanup needed"
		end if
		
		-- Get current track info
		try
			set currentTrack to current track
			set currentTrackName to name of currentTrack
			set currentTrackArtist to artist of currentTrack
			
			-- Find the position of current track in the queue
			set currentTrackPosition to 0
			repeat with i from 1 to totalTracks
				set queueTrack to track i of queuePlaylist
				if (name of queueTrack is currentTrackName) and (artist of queueTrack is currentTrackArtist) then
					set currentTrackPosition to i
					exit repeat
				end if
			end repeat
			
			-- If current track is found and it's not the first track, remove previous tracks
			if currentTrackPosition > 1 then
				set tracksToRemove to currentTrackPosition - 1
				-- Remove tracks from the beginning (tracks 1 through currentTrackPosition-1)
				repeat with i from 1 to tracksToRemove
					delete track 1 of queuePlaylist -- Always delete track 1 as indices shift
				end repeat
				return "SUCCESS: Removed " & tracksToRemove & " completed tracks from queue"
			else
				return "INFO: Current track is first in queue, no cleanup needed"
			end if
			
		on error trackErr
			return "INFO: No current track playing, no cleanup needed"
		end try
		
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
		return fmt.Errorf("AppleScript error: %s", output[7:]) // Remove "ERROR: " prefix
	}
	
	// INFO and SUCCESS messages are not errors
	if strings.HasPrefix(output, "INFO:") || strings.HasPrefix(output, "SUCCESS:") {
		return nil
	}
	
	return fmt.Errorf("unexpected AppleScript output: %s", output)
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
