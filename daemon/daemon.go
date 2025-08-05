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

func (d *Daemon) Pause() error {
	script := `tell application "Music" to pause`
	return run_script(script)
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
	script := fmt.Sprintf(`tell application "Music" to set repeat mode to "%s"`, repeatType)
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
	script := fmt.Sprintf(`tell application "Music" to get name of tracks of playlist "%s"`, playlistName)
	out, err := get_script_output(script)
	if err != nil {
		return Playlist{}, err
	}
	names := strings.Split(strings.TrimSpace(string(out)), ", ")
	tracks := make([]Track, 0, len(names))
	for _, name := range names {
		script = fmt.Sprintf(`tell application "Music"
	set t to (first track of playlist "%s" whose name is "%s")
	set trackName to name of t
	set trackArtist to artist of t
	set trackAlbum to album of t
	set trackDuration to duration of t as string
	return trackName & "||" & trackArtist & "||" & trackAlbum & "||" & trackDuration
end tell`, playlistName, name)
		out, err := get_script_output(script)
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimSpace(string(out)), "||")
		if len(parts) == 4 {
			tracks = append(tracks, Track{
				Name:     parts[0],
				Artist:   parts[1],
				Album:    parts[2],
				Duration: parts[3],
			})
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
		fmt.Println("Playlist name:", name)
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
