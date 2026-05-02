package ngrok

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"telegram-bot/internal/config"
)

// StartNgrok ensures ngrok is running and returns its public URL.
func StartNgrok() (string, error) {
	// 1. Try to fetch existing tunnel URL from local ngrok API
	url, err := fetchNgrokURL()
	if err == nil {
		fmt.Printf("Using existing ngrok tunnel: %s\n", url)
		return url, nil
	}

	cfg := config.LoadConfig()

	// 2. Start ngrok in background if not already running
	fmt.Println("Starting new ngrok tunnel on port 8080...")

	args := []string{"http", "8080"}
	if cfg.NgrokAuthToken != "" {
		args = append(args, "--authtoken", cfg.NgrokAuthToken)
	}

	cmd := exec.Command("ngrok", args...)

	// Pipe stderr to bot's stdout so errors appear in systemd logs (journalctl)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start ngrok binary: %v (make sure it's in your PATH)", err)
	}

	// 3. Poll the local API until the tunnel is ready
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		url, err := fetchNgrokURL()
		if err == nil {
			fmt.Printf("Ngrok started successfully: %s\n", url)
			return url, nil
		}
	}

	return "", fmt.Errorf("timeout waiting for ngrok to initialize. Check logs for ngrok stderr output")
}

func fetchNgrokURL() (string, error) {
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
			Proto     string `json:"proto"`
		} `json:"tunnels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	// Prefer HTTPS tunnel
	for _, t := range data.Tunnels {
		if t.Proto == "https" {
			return t.PublicURL, nil
		}
	}

	// Fallback to first available tunnel
	if len(data.Tunnels) > 0 {
		return data.Tunnels[0].PublicURL, nil
	}

	return "", fmt.Errorf("no active tunnels found")
}
