package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

func main() {
	port := flag.Int("port", 8202, "Port to listen on")
	flag.Parse()

	http.HandleFunc("/sleep", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
			return
		}

		cmd := exec.Command("systemctl", "suspend")
		err := cmd.Run()
		if err != nil {
			log.Printf("failed to suspend the server: %v", err)
		}
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
