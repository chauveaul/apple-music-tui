package lyrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LyricsResult represents the result of a lyrics search
type LyricsResult struct {
	PlainLyrics  string
	SyncedLyrics string // LRC format with timestamps
	Source       string // Which provider returned the lyrics
	Found        bool
}

// LyricsProvider interface allows multiple lyrics sources
type LyricsProvider interface {
	GetLyrics(trackName, artistName string) (LyricsResult, error)
	Name() string
}

// LyricsClient manages multiple providers with fallback
type LyricsClient struct {
	providers []LyricsProvider
	client    *http.Client
}

// NewLyricsClient creates a new client with all available providers
func NewLyricsClient() *LyricsClient {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	lc := &LyricsClient{
		client:    client,
		providers: make([]LyricsProvider, 0),
	}

	// Add LRCLIB as primary provider
	lc.providers = append(lc.providers, &LRCLIBProvider{client: client})

	// Future providers can be added here:
	// lc.providers = append(lc.providers, &MusixmatchProvider{client: client, apiKey: "..."})
	// lc.providers = append(lc.providers, &GeniusProvider{client: client, apiKey: "..."})

	return lc
}

// GetLyrics tries each provider in order until lyrics are found
func (lc *LyricsClient) GetLyrics(trackName, artistName string) (LyricsResult, error) {
	var lastError error

	for _, provider := range lc.providers {
		result, err := provider.GetLyrics(trackName, artistName)
		if err == nil && result.Found {
			return result, nil
		}
		lastError = err
	}

	if lastError != nil {
		return LyricsResult{Found: false}, fmt.Errorf("no lyrics found from any provider: %w", lastError)
	}

	return LyricsResult{Found: false}, errors.New("no lyrics found from any provider")
}

// LRCLIB Provider Implementation
type LRCLIBProvider struct {
	client *http.Client
}

func (p *LRCLIBProvider) Name() string {
	return "LRCLIB"
}

type lrclibResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func (p *LRCLIBProvider) GetLyrics(trackName, artistName string) (LyricsResult, error) {
	// Clean up track and artist names
	trackName = cleanSearchQuery(trackName)
	artistName = cleanSearchQuery(artistName)

	// Build API URL
	baseURL := "https://lrclib.net/api/get"
	params := url.Values{}
	params.Add("artist_name", artistName)
	params.Add("track_name", trackName)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Make request
	resp, err := p.client.Get(reqURL)
	if err != nil {
		return LyricsResult{Found: false}, fmt.Errorf("LRCLIB request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode == 404 {
		return LyricsResult{Found: false}, errors.New("lyrics not found in LRCLIB")
	}

	if resp.StatusCode != 200 {
		return LyricsResult{Found: false}, fmt.Errorf("LRCLIB returned status %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LyricsResult{Found: false}, fmt.Errorf("failed to read LRCLIB response: %w", err)
	}

	var lrcResp lrclibResponse
	if err := json.Unmarshal(body, &lrcResp); err != nil {
		return LyricsResult{Found: false}, fmt.Errorf("failed to parse LRCLIB response: %w", err)
	}

	// Check if instrumental
	if lrcResp.Instrumental {
		return LyricsResult{
			PlainLyrics:  "[Instrumental]",
			SyncedLyrics: "",
			Source:       "LRCLIB",
			Found:        true,
		}, nil
	}

	// Check if we got lyrics
	if lrcResp.PlainLyrics == "" && lrcResp.SyncedLyrics == "" {
		return LyricsResult{Found: false}, errors.New("LRCLIB returned empty lyrics")
	}

	return LyricsResult{
		PlainLyrics:  lrcResp.PlainLyrics,
		SyncedLyrics: lrcResp.SyncedLyrics,
		Source:       "LRCLIB",
		Found:        true,
	}, nil
}

// cleanSearchQuery removes extra information from track/artist names
func cleanSearchQuery(query string) string {
	// Remove common suffixes
	query = strings.TrimSpace(query)

	// Remove featuring info
	if idx := strings.Index(strings.ToLower(query), " feat"); idx != -1 {
		query = query[:idx]
	}
	if idx := strings.Index(strings.ToLower(query), " ft."); idx != -1 {
		query = query[:idx]
	}

	// Remove parenthetical info (Remastered, Live, etc.)
	if idx := strings.Index(query, "("); idx != -1 {
		query = query[:idx]
	}

	// Remove bracketed info
	if idx := strings.Index(query, "["); idx != -1 {
		query = query[:idx]
	}

	return strings.TrimSpace(query)
}
