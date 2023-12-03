package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

type Config struct {
	QBittorrentUrl      string `json:"qbittorrentUrl"`
	QBittorrentUsername string `json:"qbittorrentUsername"`
	QBittorrentPassword string `json:"qbittorrentPassword"`
	JellyfinUrl         string `json:"jellyfinUrl"`
	JellyfinApiKey      string `json:"jellyfinApiKey"`
}

type Sleeper struct {
	caffeinaters    []Caffeinater
	lastCaffeinated time.Time
	threshold       time.Duration
}

func (s *Sleeper) tryToSleep() {
	for _, caffeinater := range s.caffeinaters {
		shouldCaffeinate, err := caffeinater.shouldCaffeinate()
		if err != nil {
			log.Printf("Hit an error when determining whether or not to sleep: %v", err)
			continue
		}
		if shouldCaffeinate {
			s.lastCaffeinated = time.Now()
		}
	}

	if time.Now().Sub(s.lastCaffeinated) > s.threshold {
		log.Printf("Putting system to sleep. Last caffeintaed %s, current time %s", s.lastCaffeinated.String(), time.Now().String())
		cmd := exec.Command("systemctl", "suspend")
		err := cmd.Run()
		if err != nil {
			log.Printf("failed to suspend the server: %v", err)
		}
	} else {
		log.Printf("Not sleeping! Last caffeinated %s, current time %s", s.lastCaffeinated.String(), time.Now().String())
	}
}

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
			log.Printf("Not sleeping: torrent %s is currently downloading at a rate of %d\n", torrent.Name, torrent.DlSpeed)
			return true, nil
		}
	}

	return false, nil
}

func (q *QBittorrentCaffeinater) login() (*http.Cookie, error) {
	data := url.Values{}
	data.Set("username", q.credentials.Username)
	data.Set("password", q.credentials.Password)
	requestBody := bytes.NewBufferString(data.Encode())

	url := q.url + "/api/v2/auth/login"

	req, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	url := q.url + "/api/v2/torrents/info"
	req, err := http.NewRequest("GET", url, nil)
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

	log.Printf("Torrents: %v", torrents)

	return torrents, nil
}

type JellyfinCaffeinater struct {
	url    string
	apiKey string
}

type JellyfinDevice struct {
	Name             string    `json:"Name"`
	DateLastActivity time.Time `json:"DateLastActivity"`
}

type JellyfinDeviceResponse struct {
	Items []JellyfinDevice `json:"Items"`
}

func (j *JellyfinCaffeinater) shouldCaffeinate() (bool, error) {
	devices, err := j.getJellyfinDevices()
	if err != nil {
		return false, err
	}

	log.Println("devices: %v", devices)

	currTime := time.Now()
	// if any device has been active in the last 10 minutes, call that caffeinated.
	// TODO: Probably we should directly query the sessions API, and then check NowPlayingItem.
	// But I'm loath to do that.
	for _, device := range devices {
		if currTime.Sub(device.DateLastActivity) < 10*time.Minute {
			log.Printf("Not sleeping: jellyfin device %s was active less than 10 minutes ago.\n", device.Name)
			return true, err
		}
	}

	return false, err
}

func (j *JellyfinCaffeinater) getJellyfinDevices() ([]JellyfinDevice, error) {
	url := j.url + "/Devices"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Add API key header
	req.Header.Add("X-Emby-Token", j.apiKey)

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
	log.SetFlags(log.LstdFlags)

	configPath := flag.String("config", "config.json", "Path to the configuration file")
	flag.Parse()

	// Read the configuration file
	configFile, err := os.ReadFile(*configPath)
	if err != nil {
		log.Println("Error reading config file:", err)
		os.Exit(1)
	}

	// Unmarshal the configuration
	var config Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Println("Error parsing config file:", err)
		os.Exit(1)
	}

	log.Println("Loaded config.")

	jellyfinCaffeinater := JellyfinCaffeinater{
		url:    config.JellyfinUrl,
		apiKey: config.JellyfinApiKey,
	}
	qBittorrentCaffeinater := QBittorrentCaffeinater{
		url: config.QBittorrentUrl,
		credentials: QBittorrentCredentials{
			Username: config.QBittorrentUsername,
			Password: config.QBittorrentPassword,
		},
	}

	caffeinaters := []Caffeinater{&jellyfinCaffeinater, &qBittorrentCaffeinater}

	sleeper := Sleeper{
		caffeinaters:    caffeinaters,
		lastCaffeinated: time.Now(),
		threshold:       20 * time.Minute,
	}

	ticker := time.NewTicker(5 * time.Second)
	// Blocks here, and continually runs.
	for range ticker.C {
		sleeper.tryToSleep()
	}
}
