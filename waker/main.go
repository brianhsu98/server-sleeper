package main

import (
	"fmt"
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
	err = pinger.Run() // Blocks until finished.

	if err != nil {
		return false, err
	}

	stats := pinger.Statistics() // get send/receive/duplicate/rtt stats

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

func waker(macAddr string, wakeCh <-chan struct{}) {
	for {
		select {
		case <-wakeCh:
			go func() {
				err := wakeOnLan(macAddr)
				if err != nil {
					fmt.Errorf("%s\n", err)
				}
			}()
		}
	}
}

func ipWatcher(ipAddr string, wakeCh chan struct{}) {
	interval := 1 * time.Second // Set the interval for the ticker
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

	ipAddr := "10.0.0.72"
	macAddr := "6C:4B:90:4B:7B:91"
	wakeCh := make(chan struct{})

	go ipWatcher(ipAddr, wakeCh)
	waker(macAddr, wakeCh)
}
