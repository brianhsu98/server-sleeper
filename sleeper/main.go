package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Caffeinater interface {
	shouldCaffeinate() (bool, error)
}

type QBittorrentCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type QBittorrentCaffeinater struct {
	url         string
	credentials QBittorrentCredentials
}

// Torrent represents the structure of the torrent data in the response
type Torrent struct {
	Name          string  `json:"name"`
	Size          int64   `json:"size"`
	Progress      float64 `json:"progress"`
	DlSpeed       int64   `json:"dlspeed"`
	NumSeeds      int     `json:"num_seeds"`
	NumComplete   int     `json:"num_complete"`
	NumLeechs     int     `json:"num_leechs"`
	NumIncomplete int     `json:"num_incomplete"`
	Eta           int     `json:"eta"`
	State         string  `json:"state"`
}

func (q *QBittorrentCaffeinater) shouldCaffeinate() (bool, error) {
	// maybe we shouldn't login every time, but who care
	cookie, err := q.login()
	if err != nil {
		return false, err
	}

	torrents, err := q.queryTorrents(cookie)
	if err != nil {
		return false, err
	}

	for _, torrent := range torrents {
		// > 100kbps and currently downloading.
		if torrent.State == "downloading" && torrent.DlSpeed > 100000 {
			return true, nil
		}
	}
	return false, nil
}

func (q *QBittorrentCaffeinater) login() (*http.Cookie, error) {
	jsonData, err := json.Marshal(q.credentials)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", q.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			return cookie, nil
		}
	}

	return nil, fmt.Errorf("SID cookie not found in response")
}

func (q *QBittorrentCaffeinater) queryTorrents(cookie *http.Cookie) ([]Torrent, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", q.url, nil)
	if err != nil {
		return nil, err
	}

	req.AddCookie(cookie)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON response into a slice of Torrent structs
	var torrents []Torrent
	err = json.Unmarshal(body, &torrents)
	if err != nil {
		return nil, err
	}

	return torrents, nil
}

type JellyfinCaffeinater struct {
}

type JellyfinDevice struct {
	Name             string    `json:"Name"`
	DateLastActivity time.Time `json:"DateLastActivity"`
}

type JellyfinDeviceResponse struct {
	Items []JellyfinDevice `json:"Items"`
}

func (j *JellyfinCaffeinater) shouldCaffeinate() (bool, error) {
	devices, err := getJellyfinDevices()
	if err != nil {
		return false, err
	}

	currTime := time.Now()
	// if any device has been active in the last 30 minutes, do not sleep.
	for _, device := range devices {
		if currTime.Sub(device.DateLastActivity) < 30*time.Minute {
			return true, err
		}
	}

	return false, err
}

func getJellyfinDevices() ([]JellyfinDevice, error) {
	url := "http://brian-server.local:8096/Devices"
	// this is bad security, but this is only on my local network :)
	apiKey := "ca0c5282d8be40e6a8477892db9bf1a7"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Add API key header
	req.Header.Add("X-Emby-Token", apiKey)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var devices JellyfinDeviceResponse
	err = json.Unmarshal(body, &devices)
	if err != nil {
		return nil, err
	}

	return devices.Items, nil
}

func main() {
}
