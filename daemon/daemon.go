package daemon

type Daemon struct{}

type Track struct {
	trackName     string
	trackArtist   string
	trackAlbum    string
	trackDuration int //Is it int?
}

type Playlist struct {
	name   string
	tracks []Track
}

func (d *Daemon) play() {}

func (d *Daemon) play_song_by_id(id string) {}

func (d *Daemon) play_song_in_playlist(songName, playlistName string) {}

func (d *Daemon) pause() {}

func (d *Daemon) stop() {}

func (d *Daemon) next_track() {}

func (d *Daemon) previous_track() {}

func (d *Daemon) set_volume(volume int) {}

func (d *Daemon) get_volume() int { return 0 }

func (d *Daemon) set_repeat(repeatType string) {}

func (d *Daemon) get_repeat_mode() string { return "" }

func (d *Daemon) set_shuffle(isShuffle bool) {}

func (d *Daemon) get_shuffle() bool { return false }

func (d *Daemon) get_current_track() Track { return Track{} }

func (d *Daemon) get_playlist(playlistName string) Playlist { return Playlist{} }

func (d *Daemon) get_all_playlists() []Playlist { return []Playlist{} }

func (d *Daemon) play_playlist(playlist Playlist) {}

func (d *Daemon) add_song_to_playlist(song Track) {}

func (d *Daemon) remove_song_from_playlist(song Track) {}

func (d *Daemon) get_queue_tracks() []Track { return []Track{} }

func (d *Daemon) add_song_to_queue(song Track) {}

func (d *Daemon) remove_song_from_queue(song Track) {}
