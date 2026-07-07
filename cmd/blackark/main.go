package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sudowritecode/BlackArk/internal/config"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "health" && os.Args[1] != "status") {
		fmt.Fprintln(os.Stderr, "usage: blackark <health|status>")
		os.Exit(2)
	}
	cfg := config.Load()
	path := "/healthz"
	auth := false
	if os.Args[1] == "status" {
		path = "/api/v1/status"
		auth = true
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.ControlURL, "/")+path, nil)
	if err != nil {
		fail(err)
	}
	if auth {
		if cfg.APIToken == "" {
			fail(fmt.Errorf("BLACKARK_API_TOKEN is required"))
		}
		req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fail(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Print(string(body))
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "blackark:", err); os.Exit(1) }
