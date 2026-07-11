package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sudowritecode/BlackArk/internal/config"
)

type action struct {
	DeploymentID string `json:"deployment_id"`
	Image        string `json:"image"`
	Operation    string `json:"operation"`
}
type reported struct {
	ID          string  `json:"id"`
	ContainerID *string `json:"container_id"`
	Status      string  `json:"status"`
	Logs        string  `json:"logs,omitempty"`
}
type agent struct {
	cfg         config.Config
	api, docker *http.Client
	instances   map[string]reported
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("blackark-agent dev")
		return
	}
	cfg := config.Load()
	a := &agent{cfg: cfg, api: &http.Client{Timeout: 15 * time.Second}, docker: dockerClient(cfg.DockerSocket), instances: map[string]reported{}}
	if cfg.NodeID == "" || cfg.NodeToken == "" {
		if cfg.JoinToken == "" {
			if cfg.APIToken == "" {
				log.Fatal("BLACKARK_NODE_ID/BLACKARK_NODE_TOKEN, BLACKARK_JOIN_TOKEN, or BLACKARK_API_TOKEN is required")
			}
			t, err := a.createJoinToken()
			if err != nil {
				log.Fatal(err)
			}
			cfg.JoinToken = t
			a.cfg = cfg
		}
		id, t, err := a.join()
		if err != nil {
			log.Fatal(err)
		}
		cfg.NodeID = id
		cfg.NodeToken = t
		cfg.JoinToken = ""
		cfg.APIToken = ""
		a.cfg = cfg
		if cfg.AgentEnvFile != "" {
			if err := persistAgentEnv(cfg.AgentEnvFile, cfg); err != nil {
				log.Fatalf("persist joined credentials: %v", err)
			}
			log.Printf("joined as %s; persisted BLACKARK_NODE_ID=%s credentials to %s", cfg.NodeName, id, cfg.AgentEnvFile)
		} else {
			log.Printf("joined as %s; persist BLACKARK_NODE_ID=%s and BLACKARK_NODE_TOKEN securely", cfg.NodeName, id)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := a.tick(ctx); err != nil {
			log.Printf("reconcile: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func dockerClient(sock string) *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", sock)
	}}}
}
func (a *agent) request(ctx context.Context, method, path, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.cfg.ControlURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control returned %s: %s", resp.Status, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
func (a *agent) join() (string, string, error) {
	var out struct{ ID, Credential string }
	err := a.request(context.Background(), "POST", "/v1/nodes/join", "", map[string]string{"name": a.cfg.NodeName, "token": a.cfg.JoinToken}, &out)
	return out.ID, out.Credential, err
}
func (a *agent) createJoinToken() (string, error) {
	var out struct{ Token string }
	err := a.request(context.Background(), "POST", "/v1/join-tokens", a.cfg.APIToken, map[string]int{"ttl_seconds": 600}, &out)
	return out.Token, err
}
func persistAgentEnv(path string, cfg config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent.env.")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	write := func(key, value string) error {
		_, err := fmt.Fprintf(tmp, "%s=%s\n", key, quoteEnv(value))
		return err
	}
	for _, kv := range []struct{ key, value string }{
		{"BLACKARK_CONTROL_URL", cfg.ControlURL},
		{"BLACKARK_NODE_NAME", cfg.NodeName},
		{"BLACKARK_DOCKER_SOCKET", cfg.DockerSocket},
		{"BLACKARK_NODE_ID", cfg.NodeID},
		{"BLACKARK_NODE_TOKEN", cfg.NodeToken},
		{"BLACKARK_AGENT_ENV_FILE", path},
	} {
		if err := write(kv.key, kv.value); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
func quoteEnv(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
	)
	return "\"" + replacer.Replace(value) + "\""
}
func (a *agent) tick(ctx context.Context) error {
	a.refresh(ctx)
	payload := map[string]any{"capacity": map[string]any{"cpus": runtime.NumCPU()}, "resources": map[string]any{}, "instances": values(a.instances)}
	var out struct {
		Actions []action `json:"actions"`
	}
	if err := a.request(ctx, "POST", "/v1/nodes/"+a.cfg.NodeID+"/heartbeat", a.cfg.NodeToken, payload, &out); err != nil {
		return err
	}
	for _, x := range out.Actions {
		var err error
		if x.Operation == "delete" {
			err = a.remove(ctx, x)
		} else {
			err = a.run(ctx, x)
		}
		if err != nil {
			log.Printf("deployment %s: %v", x.DeploymentID, err)
		}
	}
	return nil
}

func (a *agent) refresh(ctx context.Context) {
	for id, v := range a.instances {
		if v.ContainerID == nil {
			continue
		}
		body, err := a.dockerReq(ctx, "GET", "/containers/"+*v.ContainerID+"/json", nil)
		if err != nil {
			v.Status = "failed"
			a.instances[id] = v
			continue
		}
		var state struct{ State struct{ Running bool } }
		if json.Unmarshal(body, &state) != nil {
			continue
		}
		if !state.State.Running {
			if _, err = a.dockerReq(ctx, "POST", "/containers/"+*v.ContainerID+"/start", nil); err != nil {
				v.Status = "failed"
			} else {
				v.Status = "running"
			}
		}
		if logs, e := a.dockerReq(ctx, "GET", "/containers/"+*v.ContainerID+"/logs?stdout=1&stderr=1&tail=200", nil); e == nil {
			v.Logs = decodeDockerLogs(logs)
		}
		a.instances[id] = v
	}
}

// Docker frames stdout/stderr with an eight-byte header for non-TTY containers.
// Persist only the text payload: the framing contains NUL bytes that PostgreSQL
// text columns reject.
func decodeDockerLogs(src []byte) string {
	var out bytes.Buffer
	for len(src) >= 8 && src[0] <= 2 {
		size := int(binary.BigEndian.Uint32(src[4:8]))
		if size > len(src)-8 {
			return strings.ToValidUTF8(string(src), "�")
		}
		out.Write(src[8 : 8+size])
		src = src[8+size:]
	}
	if out.Len() == 0 || len(src) != 0 {
		return strings.ToValidUTF8(string(src), "�")
	}
	return strings.ToValidUTF8(out.String(), "�")
}
func values(m map[string]reported) []reported {
	out := make([]reported, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
func (a *agent) dockerReq(ctx context.Context, method, path string, in any) ([]byte, error) {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, "http://docker"+path, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.docker.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker returned %s: %s", resp.Status, string(b))
	}
	return b, nil
}
func (a *agent) run(ctx context.Context, x action) error {
	if v, ok := a.instances[x.DeploymentID]; ok && v.Status == "running" {
		return nil
	}
	if _, err := a.dockerReq(ctx, "POST", "/images/create?fromImage="+url.QueryEscape(x.Image), nil); err != nil {
		return err
	}
	name := "blackark-" + x.DeploymentID
	body, err := a.dockerReq(ctx, "POST", "/containers/create?name="+name, map[string]any{"Image": x.Image, "Labels": map[string]string{"blackark.deployment": x.DeploymentID}, "HostConfig": map[string]any{"RestartPolicy": map[string]string{"Name": "unless-stopped"}}})
	if err != nil {
		return err
	}
	var c struct{ ID string }
	if err = json.Unmarshal(body, &c); err != nil {
		return err
	}
	if _, err = a.dockerReq(ctx, "POST", "/containers/"+c.ID+"/start", nil); err != nil {
		return err
	}
	a.instances[x.DeploymentID] = reported{ID: x.DeploymentID, ContainerID: &c.ID, Status: "running"}
	return nil
}
func (a *agent) remove(ctx context.Context, x action) error {
	v, ok := a.instances[x.DeploymentID]
	if !ok {
		return nil
	}
	if v.ContainerID != nil {
		_, err := a.dockerReq(ctx, "DELETE", "/containers/"+*v.ContainerID+"?force=true", nil)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	delete(a.instances, x.DeploymentID)
	return nil
}
