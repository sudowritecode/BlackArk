package config

import (
	"fmt"
	"os"
)

type Config struct {
	ListenAddr       string
	DatabaseURL      string
	APIToken         string
	ControlURL       string
	NodeName         string
	NodeID           string
	NodeToken        string
	JoinToken        string
	DockerSocket     string
	DashboardEnabled bool
}

func Load() Config {
	return Config{
		ListenAddr:       value("BLACKARK_LISTEN_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("BLACKARK_DATABASE_URL"),
		APIToken:         os.Getenv("BLACKARK_API_TOKEN"),
		ControlURL:       value("BLACKARK_CONTROL_URL", "http://localhost:8080"),
		NodeName:         value("BLACKARK_NODE_NAME", hostname()),
		NodeID:           os.Getenv("BLACKARK_NODE_ID"),
		NodeToken:        os.Getenv("BLACKARK_NODE_TOKEN"),
		JoinToken:        os.Getenv("BLACKARK_JOIN_TOKEN"),
		DockerSocket:     value("BLACKARK_DOCKER_SOCKET", "/var/run/docker.sock"),
		DashboardEnabled: os.Getenv("BLACKARK_DASHBOARD_ENABLED") == "true",
	}
}

func hostname() string { h, _ := os.Hostname(); return h }

func (c Config) ValidateControl() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("BLACKARK_DATABASE_URL is required")
	}
	if c.APIToken == "" {
		return fmt.Errorf("BLACKARK_API_TOKEN is required")
	}
	return nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
