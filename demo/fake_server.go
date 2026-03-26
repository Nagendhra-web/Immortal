//go:build ignore

// fake_server.go - A simple HTTP server that can be "broken" and "fixed"
// Run: go run demo/fake_server.go
// It serves on :8089 and logs to demo/app.log
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	logFile, _ := os.OpenFile("demo/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	logger := log.New(logFile, "", log.LstdFlags)

	logger.Println("INFO: Server starting on :8089")
	fmt.Println("Demo server running on http://localhost:8089")
	fmt.Println("Endpoints: / (ok), /health (ok), /break (simulate crash)")

	broken := false

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if broken {
			logger.Println("ERROR: Server is broken — returning 500")
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal Server Error")
			return
		}
		logger.Println("INFO: Served request OK")
		fmt.Fprintf(w, "Hello from demo server! Time: %s", time.Now().Format(time.RFC3339))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if broken {
			logger.Println("CRITICAL: Health check failed — server is broken")
			w.WriteHeader(500)
			fmt.Fprintf(w, `{"status": "unhealthy"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy"}`)
	})

	http.HandleFunc("/break", func(w http.ResponseWriter, r *http.Request) {
		broken = true
		logger.Println("FATAL: Server manually broken via /break endpoint")
		fmt.Fprintf(w, "Server is now broken! Immortal should detect this.")
	})

	http.HandleFunc("/fix", func(w http.ResponseWriter, r *http.Request) {
		broken = false
		logger.Println("INFO: Server manually fixed via /fix endpoint")
		fmt.Fprintf(w, "Server is fixed!")
	})

	log.Fatal(http.ListenAndServe(":8089", nil))
}
