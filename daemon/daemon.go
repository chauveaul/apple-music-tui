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
	trackName     string
	trackArtist   string
	trackAlbum    string
	trackDuration string
}

type Playlist struct {
	name   string
	tracks []Track
}

func run_script(script string) error {
	return exec.Command("osascript", "-e", script).Run()
}

func get_script_output(script string) ([]byte, error) {
	return exec.Command("osascript", "-e", script).Output()
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
	script := `tell application "Music" to get name of current track & "||" & artist of current track & "||" & album of current track & "||" & duration of current track as string`
	out, err := get_script_output(script)
	if err != nil {
		return Track{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "||")
	if len(parts) < 4 {
		return Track{}, errors.New("Invalid output from get_current_track()")
	}
	return Track{trackName: parts[0], trackArtist: parts[1], trackAlbum: parts[2], trackDuration: parts[3]}, nil
}

func (d *Daemon) PlayPlaylist(playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to play playlist "%s"`, playlist.name)
	return run_script(script)
}

func (d *Daemon) AddSongToPlaylist(song Track, playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to duplicate (first track whose name is "%s") to playlist "%s"`, song.trackName, playlist.name)
	return run_script(script)
}

func (d *Daemon) RemoveSongFromPlaylist(song Track, playlist Playlist) error {
	script := fmt.Sprintf(`tell application "Music" to delete (first track whose name is "%s") of playlist "%s"`, song.trackName, playlist.name)
	return run_script(script)
}

//TODO: Change this for apple music api
//func (d *Daemon) GetPlaylist(playlistName string) (Playlist, error) {
//	script := fmt.Sprintf(`tell application "Music" to get name of tracks of playlist "%s"`, playlistName)
//	out, err := get_script_output(script)
//	if err != nil {
//		return Playlist{}, err
//	}
//	names := strings.Split(strings.TrimSpace(string(out)), ", ")
//	tracks := make([]Track, 0, len(names))
//	fmt.Println(names)
//	for _, name := range names {
//		script = fmt.Sprintf(`
//			tell application "Music" to set t to (first track of playlist "%s" whose name is "%s")
//				set trackName to name of t
//				set trackArtist to artist of t
//				set trackAlbum to album of t
//				set trackDuration to duration of t as string
//				return trackName & "||" & trackArtist & "||" & trackArtist & "||" & trackAlbum & "||" & trackDuration
//			`, playlistName, name)
//		out, err := get_script_output(script)
//		if err != nil {
//			continue
//		}
//		parts := strings.Split(strings.TrimSpace(string(out)), "||")
//		if len(parts) == 4 {
//			tracks = append(tracks, Track{
//				trackName:     parts[0],
//				trackArtist:   parts[1],
//				trackAlbum:    parts[2],
//				trackDuration: parts[3],
//			})
//		}
//	}
//	return Playlist{name: playlistName, tracks: tracks}, nil
//}
//
//func (d *Daemon) GetAllPlaylists() ([]Playlist, error) {
//	script := `tell application "Music" to get name of playlists`
//	out, err := get_script_output(script)
//	if err != nil {
//		return []Playlist{}, err
//	}
//	names := strings.Split(strings.TrimSpace(string(out)), ", ")
//	playlists := make([]Playlist, 0, len(names))
//	for _, name := range names {
//		pl, err := d.GetPlaylist(name)
//		if err != nil {
//			continue
//		}
//		playlists = append(playlists, pl)
//	}
//	return playlists, nil
//}

//TODO: are those even possible?
//func (d *Daemon) get_queue_tracks() ([]Track, error) {}
//func (d *Daemon) add_song_to_queue(song Track) {}
//func (d *Daemon) remove_song_from_queue(song Track) {}
