package main

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudowritecode/BlackArk/internal/config"
)

func frame(stream byte, payload string) []byte {
	b := make([]byte, 8+len(payload))
	b[0] = stream
	binary.BigEndian.PutUint32(b[4:8], uint32(len(payload)))
	copy(b[8:], payload)
	return b
}

func TestDecodeDockerLogs(t *testing.T) {
	framed := append(frame(1, "hello\n"), frame(2, "warning\n")...)
	if got, want := decodeDockerLogs(framed), "hello\nwarning\n"; got != want {
		t.Fatalf("decodeDockerLogs() = %q, want %q", got, want)
	}
}

func TestDecodeDockerLogsPlainText(t *testing.T) {
	if got, want := decodeDockerLogs([]byte("plain\n")), "plain\n"; got != want {
		t.Fatalf("decodeDockerLogs() = %q, want %q", got, want)
	}
}

func TestCreateJoinTokenUsesAdminToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/join-tokens" {
			t.Fatalf("path = %s, want /v1/join-tokens", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer admin-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		var body map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if got, want := body["ttl_seconds"], 600; got != want {
			t.Fatalf("ttl_seconds = %d, want %d", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "join-token"})
	}))
	defer srv.Close()

	a := &agent{cfg: config.Config{ControlURL: srv.URL, APIToken: "admin-token"}, api: srv.Client()}
	got, err := a.createJoinToken()
	if err != nil {
		t.Fatal(err)
	}
	if got != "join-token" {
		t.Fatalf("createJoinToken() = %q, want join-token", got)
	}
}

func TestAgentJoinWithValidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/nodes/join" {
			t.Fatalf("path = %s, want /v1/nodes/join", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if got, want := body["name"], "worker-1"; got != want {
			t.Fatalf("name = %q, want %q", got, want)
		}
		if got, want := body["token"], "join-token-123"; got != want {
			t.Fatalf("token = %q, want %q", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "node-456", "credential": "node-token-789"})
	}))
	defer srv.Close()

	a := &agent{cfg: config.Config{ControlURL: srv.URL, NodeName: "worker-1", JoinToken: "join-token-123"}, api: srv.Client()}
	id, token, err := a.join()
	if err != nil {
		t.Fatal(err)
	}
	if id != "node-456" {
		t.Fatalf("join() id = %q, want node-456", id)
	}
	if token != "node-token-789" {
		t.Fatalf("join() credential = %q, want node-token-789", token)
	}
}

func TestPersistAgentEnvStoresDurableCredentialsOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.env")
	cfg := config.Config{
		ControlURL:   "https://blackark.example.test",
		NodeName:     "worker 1",
		NodeID:       "node-123",
		NodeToken:    "node-token",
		JoinToken:    "join-token",
		APIToken:     "admin-token",
		AgentEnvFile: path,
		DockerSocket: "/var/run/docker.sock",
	}
	if err := persistAgentEnv(path, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"BLACKARK_CONTROL_URL=\"https://blackark.example.test\"",
		"BLACKARK_NODE_NAME=\"worker 1\"",
		"BLACKARK_NODE_ID=\"node-123\"",
		"BLACKARK_NODE_TOKEN=\"node-token\"",
		"BLACKARK_AGENT_ENV_FILE=\"" + path + "\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("persisted env missing %q in:\n%s", want, got)
		}
	}
	for _, secret := range []string{"BLACKARK_JOIN_TOKEN", "BLACKARK_API_TOKEN", "join-token", "admin-token"} {
		if strings.Contains(got, secret) {
			t.Fatalf("persisted env leaked %q in:\n%s", secret, got)
		}
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got, want := info.Mode().Perm(), os.FileMode(0600); got != want {
		t.Fatalf("mode = %v, want %v", got, want)
	}
}
