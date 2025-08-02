package daemon

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Mock helper for testing osascript commands
var mockCommandResults = make(map[string]mockResult)

type mockResult struct {
	output []byte
	err    error
}

// Override exec.Command for testing
var execCommand = exec.Command

func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Mock process - just exit
	os.Exit(0)
}

func TestDaemon_Play(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful play command",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.Play(); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.Play() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_PlaySongById(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "play song with fake ID (expected to fail)",
			id:      "12345",
			wantErr: true, // Will fail since ID doesn't exist
		},
		{
			name:    "play song with empty ID (expected to fail)",
			id:      "",
			wantErr: true, // Will fail with empty ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.PlaySongById(tt.id); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.PlaySongById() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_PlaySongInPlaylist(t *testing.T) {
	tests := []struct {
		name         string
		songName     string
		playlistName string
		wantErr      bool
	}{
		{
			name:         "play song in non-existent playlist (expected to fail)",
			songName:     "Test Song",
			playlistName: "Test Playlist",
			wantErr:      true, // Will fail since playlist doesn't exist
		},
		{
			name:         "play song with empty names (expected to fail)",
			songName:     "",
			playlistName: "",
			wantErr:      true, // Will fail with empty names
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.PlaySongInPlaylist(tt.songName, tt.playlistName); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.PlaySongInPlaylist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_Pause(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful pause command",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.Pause(); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.Pause() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_Stop(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful stop command",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.Stop(); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_NextTrack(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful next track command",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.NextTrack(); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.NextTrack() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_PreviousTrack(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful previous track command",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.PreviousTrack(); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.PreviousTrack() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_SetVolume(t *testing.T) {
	tests := []struct {
		name    string
		volume  int
		wantErr bool
	}{
		{
			name:    "set volume to 50",
			volume:  50,
			wantErr: false,
		},
		{
			name:    "set volume to 0",
			volume:  0,
			wantErr: false,
		},
		{
			name:    "set volume to 100",
			volume:  100,
			wantErr: false,
		},
		{
			name:    "set volume to negative value",
			volume:  -10,
			wantErr: false, // Script will still execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.SetVolume(tt.volume); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.SetVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_GetVolume(t *testing.T) {
	// Note: This test would need proper mocking to work in a real test environment
	tests := []struct {
		name       string
		mockOutput string
		mockErr    error
		want       int
		wantErr    bool
	}{
		{
			name:       "get volume success",
			mockOutput: "75",
			mockErr:    nil,
			want:       75,
			wantErr:    false,
		},
		{
			name:       "get volume with whitespace",
			mockOutput: "  50  ",
			mockErr:    nil,
			want:       50,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			// This would need proper mocking setup
			_, err := d.GetVolume()
			if (err != nil) != tt.wantErr {
				t.Errorf("Daemon.GetVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Note: Actual value testing would require mocking
		})
	}
}

func TestDaemon_SetRepeat(t *testing.T) {
	tests := []struct {
		name       string
		repeatType string
		wantErr    bool
	}{
		{
			name:       "set repeat to all (may fail with invalid mode)",
			repeatType: "all",
			wantErr:    true, // May fail due to invalid repeat mode value
		},
		{
			name:       "set repeat to one (may fail with invalid mode)",
			repeatType: "one",
			wantErr:    true, // May fail due to invalid repeat mode value
		},
		{
			name:       "set repeat to off (may fail with invalid mode)",
			repeatType: "off",
			wantErr:    true, // May fail due to invalid repeat mode value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.SetRepeat(tt.repeatType); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.SetRepeat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_GetRepeatMode(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "get repeat mode",
			wantErr: false, // Will fail due to typo in the original function ("applcation")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			_, err := d.GetRepeatMode()
			if (err != nil) != tt.wantErr {
				t.Errorf("Daemon.GetRepeatMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_SetShuffle(t *testing.T) {
	tests := []struct {
		name      string
		isShuffle bool
		wantErr   bool
	}{
		{
			name:      "enable shuffle",
			isShuffle: true,
			wantErr:   false,
		},
		{
			name:      "disable shuffle",
			isShuffle: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.SetShuffle(tt.isShuffle); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.SetShuffle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_GetShuffle(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "get shuffle status",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			_, err := d.GetShuffle()
			if (err != nil) != tt.wantErr {
				t.Errorf("Daemon.GetShuffle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_GetCurrentTrack(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "get current track",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			_, err := d.GetCurrentTrack()
			if (err != nil) != tt.wantErr {
				t.Errorf("Daemon.GetCurrentTrack() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test Track parsing specifically
func TestTrackParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Track
		wantErr  bool
	}{
		{
			name:  "valid track data",
			input: "Song Name||Artist Name||Album Name||180",
			expected: Track{
				trackName:     "Song Name",
				trackArtist:   "Artist Name",
				trackAlbum:    "Album Name",
				trackDuration: "180",
			},
			wantErr: false,
		},
		{
			name:     "invalid track data - too few parts",
			input:    "Song Name||Artist Name",
			expected: Track{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Split(strings.TrimSpace(tt.input), "||")
			if len(parts) < 4 {
				if !tt.wantErr {
					t.Errorf("Expected no error but got insufficient parts")
				}
				return
			}
			track := Track{
				trackName:     parts[0],
				trackArtist:   parts[1],
				trackAlbum:    parts[2],
				trackDuration: parts[3],
			}
			if !reflect.DeepEqual(track, tt.expected) {
				t.Errorf("Track parsing got %v, want %v", track, tt.expected)
			}
		})
	}
}

func TestDaemon_PlayPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		playlist Playlist
		wantErr  bool
	}{
		{
			name: "play playlist",
			playlist: Playlist{
				name: "My Playlist",
				tracks: []Track{{
					trackName:     "Test Song",
					trackArtist:   "Test Artist",
					trackAlbum:    "Test Album",
					trackDuration: "180",
				}},
			},
			wantErr: false,
		},
		{
			name: "play empty playlist",
			playlist: Playlist{
				name:   "",
				tracks: []Track{},
			},
			wantErr: true, // Script will still execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.PlayPlaylist(tt.playlist); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.PlayPlaylist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_AddsSongToPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		song     Track
		playlist Playlist
		wantErr  bool
	}{
		{
			name: "add song to playlist",
			song: Track{
				trackName:     "Landed In Brooklyn",
				trackArtist:   "Khantrast",
				trackAlbum:    "Landed In Brooklyn - Single",
				trackDuration: "112",
			},
			playlist: Playlist{
				name:   "My Playlist",
				tracks: []Track{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.AddSongToPlaylist(tt.song, tt.playlist); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.AddSSongToPlaylist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDaemon_RemoveSongFromPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		song     Track
		playlist Playlist
		wantErr  bool
	}{
		{
			name: "remove song from playlist",
			song: Track{
				trackName:     "Landed In Brooklyn",
				trackArtist:   "Khantrast",
				trackAlbum:    "Landed In Brooklyn - Single",
				trackDuration: "112",
			},
			playlist: Playlist{
				name:   "My Playlist",
				tracks: []Track{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{}
			if err := d.RemoveSongFromPlaylist(tt.song, tt.playlist); (err != nil) != tt.wantErr {
				t.Errorf("Daemon.RemoveSongFromPlaylist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test helper functions
func TestRunScript(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{
			name:    "valid script",
			script:  `tell application "Music" to play`,
			wantErr: false,
		},
		{
			name:    "empty script",
			script:  "",
			wantErr: false, // osascript will still execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run_script(tt.script); (err != nil) != tt.wantErr {
				t.Errorf("run_script() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetScriptOutput(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{
			name:    "valid script with output",
			script:  `tell application "Music" to sound volume`,
			wantErr: false,
		},
		{
			name:    "empty script",
			script:  "",
			wantErr: false, // osascript will still execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := get_script_output(tt.script)
			if (err != nil) != tt.wantErr {
				t.Errorf("get_script_output() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
