package main

import (
	"log"
	"os/exec"
	"time"

	"github.com/go-ping/ping"
)

func pingIP(ip string) (bool, error) {
	pinger, err := ping.NewPinger(ip)
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

func wakeOnLan(macAddr string) error {
	cmd := exec.Command("wakeonlan", macAddr)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func waker(targetHost string, macAddr string, wakeCh <-chan struct{}) {
	for {
		select {
		case <-wakeCh:
			targetHostActive, err := pingIP(targetHost)
			if err != nil || !targetHostActive {
				log.Printf("Waking server on lan")
				go func() {
					err := wakeOnLan(macAddr)
					if err != nil {
						log.Printf("Failed to wake on lan: %s\n", err)
					}
				}()
			} else {
				log.Printf("Target host is already awake. Not sending wakeonlan packet.\n")
			}
		}
	}
}

func ipWatcher(ipAddr string, wakeCh chan struct{}) {
	interval := 3 * time.Second // Set the interval for the ticker
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			res, err := pingIP(ipAddr)
			if err != nil {
				log.Printf("Error while pinging: %s\n", err)
			}

			if res {
				log.Printf("IP address %s is active. Waking target server\n", ipAddr)
				wakeCh <- struct{}{}
			} else {
				log.Printf("IP address %s is not active\n", ipAddr)
			}
		}
	}

}

func main() {
	log.SetFlags(log.LstdFlags)

	// really we should be using DNS...
	ipAddr := "10.0.0.72"
	targetIp := "10.0.0.86"
	macAddr := "6C:4B:90:4B:7B:91"
	wakeCh := make(chan struct{})

	go ipWatcher(ipAddr, wakeCh)
	waker(targetIp, macAddr, wakeCh)
}
