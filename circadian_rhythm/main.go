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
	"strings"
	"time"

	"github.com/go-ping/ping"
)

type Config struct {
	QBittorrentUrl      string `json:"qbittorrentUrl"`
	QBittorrentUsername string `json:"qbittorrentUsername"`
	QBittorrentPassword string `json:"qbittorrentPassword"`
	JellyfinUrl         string `json:"jellyfinUrl"`
	JellyfinApiKey      string `json:"jellyfinApiKey"`
	// TODO: Probably we should use DNS
	TargetIpAddress  string `json:"targetIpAddress"`
	TargetPort       int    `json:"targetPort"`
	TargetMacAddress string `json:"targetMacAddress"`
	TVIpAddress      string `json:"tvIpAddress"`
}

type CircadianRhythm struct {
	caffeinaters     []Caffeinater
	lastCaffeinated  time.Time
	threshold        time.Duration
	targetMacAddress string
	targetIpAddress  string
	targetPort       int
}

// TODO: We should run the cycle every 5 seconds or so
func (c *CircadianRhythm) cycle() {
	for _, caffeinater := range c.caffeinaters {
		shouldCaffeinate, err := caffeinater.shouldCaffeinate()
		if err != nil {
			log.Printf("Hit an error when determining whether or not to sleep: %v. Server may be asleep.", err)
			continue
		}
		if shouldCaffeinate {
			c.lastCaffeinated = time.Now()
		}
	}

	if time.Now().Sub(c.lastCaffeinated) > c.threshold {
		log.Printf("Putting system to sleep. Last caffeintaed %s, current time %s", c.lastCaffeinated.String(), time.Now().String())
		c.sleep()
	} else {
		log.Printf("Waking! Last caffeinated %s, current time %s", c.lastCaffeinated.String(), time.Now().String())
		c.wake()
	}
}

func (c *CircadianRhythm) wake() error {
	cmd := exec.Command("wakeonlan", c.targetMacAddress)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (c *CircadianRhythm) sleep() error {
	url := fmt.Sprintf("http://%s:%d/sleep", c.targetIpAddress, c.targetPort)

	// Create a client
	client := &http.Client{
		Timeout: time.Second,
	}

	// Create a new request
	req, err := http.NewRequest("POST", url, strings.NewReader(""))
	if err != nil {
		return err
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Log the response status
	log.Printf("Response status: %s", resp.Status)

	return nil
}

type Caffeinater interface {
	shouldCaffeinate() (bool, error)
}

type TVPingerCaffeinater struct {
	ip string
}

func (t *TVPingerCaffeinater) shouldCaffeinate() (bool, error) {
	pinger, err := ping.NewPinger(t.ip)
	if err != nil {
		return false, err
	}
	pinger.Count = 1
	pinger.Timeout = time.Second
	err = pinger.Run()

	if err != nil {
		return false, err
	}

	stats := pinger.Statistics()

	return stats.PacketsRecv > 0, nil
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

	client := &http.Client{
		Timeout: time.Second,
	}
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

	return torrents, nil
}

type JellyfinCaffeinater struct {
	url    string
	apiKey string
}

type JellyfinSessionNowPlayingItem struct {
	Name string `json:"Name"`
}

type JellyfinSession struct {
	Client         string                        `json:"Client"`
	NowPlayingItem JellyfinSessionNowPlayingItem `json:"NowPlayingItem"`
}

func (j *JellyfinCaffeinater) shouldCaffeinate() (bool, error) {
	sessions, err := j.getActiveSessions()
	if err != nil {
		return false, err
	}

	for _, session := range sessions {
		if session.NowPlayingItem.Name != "" {
			log.Printf("Not sleeping: jellyfin session %s is actively playing content", session.Client)
			return true, nil
		}
	}

	return false, err
}

func (j *JellyfinCaffeinater) getActiveSessions() ([]JellyfinSession, error) {
	url := j.url + "/Sessions?ActiveWithinSeconds=90"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Add API key header
	req.Header.Add("X-Emby-Token", j.apiKey)

	// Send req using http Client
	client := &http.Client{
		Timeout: time.Second,
	}
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

	var sessions []JellyfinSession

	err = json.Unmarshal(body, &sessions)
	if err != nil {
		return nil, err
	}

	return sessions, nil
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
	tvCaffeinater := TVPingerCaffeinater{ip: config.TVIpAddress}

	caffeinaters := []Caffeinater{&jellyfinCaffeinater, &qBittorrentCaffeinater, &tvCaffeinater}

	circadianRhythm := CircadianRhythm{
		caffeinaters:     caffeinaters,
		lastCaffeinated:  time.Now(),
		threshold:        10 * time.Minute,
		targetMacAddress: config.TargetMacAddress,
		targetIpAddress:  config.TargetIpAddress,
		targetPort:       config.TargetPort,
	}

	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		circadianRhythm.cycle()
	}
}
