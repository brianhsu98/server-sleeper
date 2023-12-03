package main

import (
	"fmt"
	"net"
	"time"
)

func pingIP(ip string) bool {
	timeout := time.Duration(1 * time.Second)
	_, err := net.DialTimeout("tcp", ip+":80", timeout)
	if err != nil {
		return false
	}
	return true
}

func main() {
	ip := "10.0.0.72"

	interval := 1 * time.Second // Set the interval for the ticker
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			res := pingIP(ip)
			fmt.Printf("Ping to %s result: %v\n", ip, res)
		}
	}
}
