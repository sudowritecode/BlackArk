package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}
type Manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Image    string `yaml:"image"`
		Replicas *int   `yaml:"replicas"`
	} `yaml:"spec"`
}
type App struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Image     string         `json:"image"`
	Replicas  int            `json:"replicas"`
	Instances []instanceItem `json:"instances,omitempty"`
}
type nodeTableItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type appTableItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
	Replicas int    `json:"replicas"`
}
type instanceItem struct {
	ID          string `json:"id"`
	NodeID      string `json:"node_id"`
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
}

type dashboardResponse struct {
	Cluster struct {
		URL        string `json:"url"`
		Version    string `json:"version"`
		UptimeSecs int64  `json:"uptime_seconds"`
	} `json:"cluster"`
	Nodes struct {
		Healthy   int `json:"healthy"`
		Unhealthy int `json:"unhealthy"`
		Pending   int `json:"pending"`
		Total     int `json:"total"`
		Details   []struct {
			ID       string     `json:"id"`
			Name     string     `json:"name"`
			Status   string     `json:"status"`
			CPUs     float64    `json:"cpus"`
			CPUsUsed float64    `json:"cpus_used"`
			MemBytes int64      `json:"mem_bytes"`
			MemUsed  int64      `json:"mem_used_bytes"`
			AppCount int        `json:"app_count"`
			LastSeen *time.Time `json:"last_seen_at"`
		} `json:"details"`
	} `json:"nodes"`
	Apps struct {
		Running int `json:"running"`
		Stopped int `json:"stopped"`
		Failed  int `json:"failed"`
		Total   int `json:"total"`
		Details []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Image           string `json:"image"`
			DesiredReplicas int    `json:"desired_replicas"`
			ReadyReplicas   int    `json:"ready_replicas"`
			Status          string `json:"status"`
		} `json:"details"`
	} `json:"apps"`
}

var client = &http.Client{Timeout: 15 * time.Second}

func configPath() string {
	if p := os.Getenv("BLACKARK_CONFIG"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return ".blackark.yaml"
	}
	return filepath.Join(d, "blackark", "config.yaml")
}
func loadConfig() (Config, error) {
	c := Config{URL: os.Getenv("BLACKARK_CONTROL_URL"), Token: os.Getenv("BLACKARK_API_TOKEN")}
	b, err := os.ReadFile(configPath())
	if err == nil {
		if err = yaml.Unmarshal(b, &c); err != nil {
			return c, fmt.Errorf("invalid config %s: %w", configPath(), err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	if v := os.Getenv("BLACKARK_CONTROL_URL"); v != "" {
		c.URL = v
	}
	if v := os.Getenv("BLACKARK_API_TOKEN"); v != "" {
		c.Token = v
	}
	return c, nil
}
func saveConfig(c Config) error {
	if _, err := url.ParseRequestURI(c.URL); err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, _ := yaml.Marshal(c)
	if err := os.WriteFile(p, b, 0600); err != nil {
		return err
	}
	return nil
}

func Run(args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("login", flag.ContinueOnError)
		fs.SetOutput(errOut)
		u := fs.String("url", "", "control-plane URL")
		t := fs.String("token", "", "API token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *u == "" || *t == "" {
			return fmt.Errorf("login requires --url and --token")
		}
		c := Config{URL: strings.TrimRight(*u, "/"), Token: *t}
		if err := saveConfig(c); err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved credentials to %s\n", configPath())
		return nil
	case "health":
		return raw(Config{URL: envOr("BLACKARK_CONTROL_URL", "http://localhost:8080")}, http.MethodGet, "/healthz", nil, out)
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	if c.URL == "" || c.Token == "" {
		return fmt.Errorf("not logged in; run blackark login --url <url> --token <token>")
	}
	switch args[0] {
	case "join-token":
		return joinTokenCmd(c, args[1:], out, errOut)
	case "dashboard":
		return dashboardCmd(c, args[1:], out, errOut)
	case "status":
		return raw(c, http.MethodGet, "/api/v1/status", nil, out)
	case "get":
		fs := flag.NewFlagSet("get", flag.ContinueOnError)
		fs.SetOutput(errOut)
		o := fs.String("o", "table", "output format: table|json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: blackark get [-o table|json] <nodes|apps>")
		}
		res := fs.Arg(0)
		switch res {
		case "nodes":
			if *o == "json" {
				return raw(c, http.MethodGet, "/v1/nodes", nil, out)
			}
			return getNodesCmd(c, out)
		case "apps":
			if *o == "json" {
				return raw(c, http.MethodGet, "/v1/apps", nil, out)
			}
			return getAppsCmd(c, out)
		default:
			return fmt.Errorf("unknown resource %q", res)
		}
	case "describe":
		fs := flag.NewFlagSet("describe", flag.ContinueOnError)
		fs.SetOutput(errOut)
		o := fs.String("o", "table", "output format: table|json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 2 || fs.Arg(0) != "app" {
			return fmt.Errorf("usage: blackark describe [-o table|json] app <id>")
		}
		if *o == "json" {
			return raw(c, http.MethodGet, "/v1/apps/"+url.PathEscape(fs.Arg(1)), nil, out)
		}
		return describeAppCmd(c, fs.Arg(1), out)
	case "apply":
		fs := flag.NewFlagSet("apply", flag.ContinueOnError)
		fs.SetOutput(errOut)
		f := fs.String("f", "", "manifest file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *f == "" {
			return fmt.Errorf("usage: blackark apply -f <manifest.yaml>")
		}
		return apply(c, *f, out)
	case "logs":
		fs := flag.NewFlagSet("logs", flag.ContinueOnError)
		fs.SetOutput(errOut)
		tail := fs.Int("tail", 200, "lines")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 || *tail < 1 || *tail > 5000 {
			return fmt.Errorf("usage: blackark logs [--tail 1..5000] <app-id>")
		}
		return raw(c, http.MethodGet, "/v1/apps/"+url.PathEscape(fs.Arg(0))+"/logs?tail="+strconv.Itoa(*tail), nil, out)
	case "scale":
		if len(args) != 3 {
			return fmt.Errorf("usage: blackark scale <app-id> <replicas>")
		}
		n, e := strconv.Atoi(args[2])
		if e != nil || n < 0 {
			return fmt.Errorf("replicas must be a non-negative integer")
		}
		return raw(c, http.MethodPatch, "/v1/apps/"+url.PathEscape(args[1]), map[string]any{"replicas": n}, out)
	case "restart":
		if len(args) != 2 {
			return fmt.Errorf("usage: blackark restart <app-id>")
		}
		return raw(c, http.MethodPost, "/v1/apps/"+url.PathEscape(args[1])+"/restart", map[string]any{}, out)
	case "delete":
		if len(args) != 3 || args[1] != "app" {
			return fmt.Errorf("usage: blackark delete app <id>")
		}
		if err := raw(c, http.MethodDelete, "/v1/apps/"+url.PathEscape(args[2]), nil, io.Discard); err != nil {
			return err
		}
		fmt.Fprintln(out, "app deleted")
		return nil
	default:
		return usage()
	}
}
func joinTokenCmd(c Config, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("join-token", flag.ContinueOnError)
	fs.SetOutput(errOut)
	ttl := fs.Int("ttl", 600, "token TTL in seconds (1..86400)")
	o := fs.String("o", "table", "output format: table|json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := request(c, http.MethodPost, "/v1/join-tokens", map[string]int{"ttl_seconds": *ttl})
	if err != nil {
		return err
	}
	if *o == "json" {
		_, err = out.Write(b)
		return err
	}
	var resp struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "Token:\t%s\n", resp.Token)
	fmt.Fprintf(tw, "TTL:\t%ds\n", *ttl)
	fmt.Fprintf(tw, "Expires:\t%s\n", resp.ExpiresAt)
	tw.Flush()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "# Copy-paste this command on the worker node to join the cluster:")
	cmd := fmt.Sprintf("docker run -d --name blackark-agent --restart unless-stopped \\\n  -e BLACKARK_CONTROL_URL=%s \\\n  -e BLACKARK_JOIN_TOKEN=%s \\\n  -v /var/run/docker.sock:/var/run/docker.sock \\\n  blackark-agent", c.URL, resp.Token)
	fmt.Fprintln(out, cmd)
	return nil
}

func dashboardCmd(c Config, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(errOut)
	watch := fs.Bool("watch", false, "auto-refresh")
	watchShort := fs.Bool("w", false, "auto-refresh (shorthand)")
	interval := fs.Int("interval", 5, "refresh interval in seconds")
	intervalShort := fs.Int("i", 5, "refresh interval in seconds (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	iv := *interval
	if *intervalShort != 5 {
		iv = *intervalShort
	}
	if iv < 1 {
		iv = 1
	}
	w := *watch || *watchShort

	if w {
		for {
			if err := renderDashboard(c, out, true); err != nil {
				return err
			}
			time.Sleep(time.Duration(iv) * time.Second)
		}
	}
	return renderDashboard(c, out, false)
}

func renderDashboard(c Config, out io.Writer, clear bool) error {
	b, err := request(c, http.MethodGet, "/api/v1/dashboard", nil)
	if err != nil {
		return err
	}
	var d dashboardResponse
	if err := json.Unmarshal(b, &d); err != nil {
		return fmt.Errorf("invalid dashboard response: %w", err)
	}

	if clear {
		fmt.Fprint(out, "\033[2J\033[H")
	}

	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)

	fmt.Fprintf(tw, "BlackArk Cluster\t%s\n", d.Cluster.URL)
	fmt.Fprintf(tw, "Version\t%s\tUptime\t%ds\n", d.Cluster.Version, d.Cluster.UptimeSecs)
	fmt.Fprintln(tw, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(tw, "Nodes:\t%d healthy\t·\t%d unhealthy\t·\t%d pending\n", d.Nodes.Healthy, d.Nodes.Unhealthy, d.Nodes.Pending)
	fmt.Fprintf(tw, "Apps:\t%d running\t·\t%d stopped\t·\t%d failed\n", d.Apps.Running, d.Apps.Stopped, d.Apps.Failed)
	fmt.Fprintln(tw)

	// Nodes table header
	fmt.Fprintln(tw, "NODE\tSTATUS\tCPUS\tMEM\tAPPS")
	for _, n := range d.Nodes.Details {
		memUsed := formatBytes(n.MemUsed)
		memTotal := formatBytes(n.MemBytes)
		fmt.Fprintf(tw, "%s\t%s\t%.0f/%.0f\t%s/%s\t%d\n", n.Name, n.Status, n.CPUsUsed, n.CPUs, memUsed, memTotal, n.AppCount)
	}

	fmt.Fprintln(tw)

	// Apps table header
	fmt.Fprintln(tw, "APP\tIMAGE\tREPLICAS\tSTATUS")
	for _, a := range d.Apps.Details {
		fmt.Fprintf(tw, "%s\t%s\t%d/%d\t%s\n", a.Name, a.Image, a.ReadyReplicas, a.DesiredReplicas, a.Status)
	}

	return tw.Flush()
}

func getNodesCmd(c Config, out io.Writer) error {
	b, err := request(c, http.MethodGet, "/v1/nodes", nil)
	if err != nil {
		return err
	}
	var nodes []nodeTableItem
	if err := json.Unmarshal(b, &nodes); err != nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS")
	for _, n := range nodes {
		fmt.Fprintf(tw, "%s\t%s\n", n.Name, n.Status)
	}
	return tw.Flush()
}

func getAppsCmd(c Config, out io.Writer) error {
	b, err := request(c, http.MethodGet, "/v1/apps", nil)
	if err != nil {
		return err
	}
	var apps []appTableItem
	if err := json.Unmarshal(b, &apps); err != nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tIMAGE\tREPLICAS")
	for _, a := range apps {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", a.Name, a.Image, a.Replicas)
	}
	return tw.Flush()
}

func describeAppCmd(c Config, id string, out io.Writer) error {
	b, err := request(c, http.MethodGet, "/v1/apps/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	var a App
	if err := json.Unmarshal(b, &a); err != nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "Name:\t%s\n", a.Name)
	fmt.Fprintf(tw, "ID:\t%s\n", a.ID)
	fmt.Fprintf(tw, "Image:\t%s\n", a.Image)
	fmt.Fprintf(tw, "Replicas:\t%d\n", a.Replicas)
	if len(a.Instances) > 0 {
		fmt.Fprintln(tw)
		fmt.Fprintln(tw, "Instances:")
		fmt.Fprintln(tw, "  ID\tNode\tContainer\tStatus")
		for _, x := range a.Instances {
			cid := x.ContainerID
			if len(cid) > 12 {
				cid = cid[:12]
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", x.ID, x.NodeID, cid, x.Status)
		}
	}
	return tw.Flush()
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "0B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	v := float64(b)
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%dB", b)
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

func usage() error {
	return fmt.Errorf("usage: blackark <login|health|status|get|apply|describe|logs|scale|restart|delete|join-token|dashboard>")
}
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func request(c Config, method, path string, body any) ([]byte, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, strings.TrimRight(c.URL, "/")+path, r)
	if e != nil {
		return nil, e
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("request failed: %w", e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode >= 300 {
		var x struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(b, &x)
		if x.Error == "" {
			x.Error = strings.TrimSpace(string(b))
		}
		return nil, fmt.Errorf("server returned %s: %s", resp.Status, x.Error)
	}
	return b, nil
}
func raw(c Config, m, p string, v any, out io.Writer) error {
	b, e := request(c, m, p, v)
	if e != nil {
		return e
	}
	if len(b) > 0 {
		_, e = out.Write(b)
	}
	return e
}
func apply(c Config, path string, out io.Writer) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return fmt.Errorf("read manifest: %w", e)
	}
	var m Manifest
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if e = d.Decode(&m); e != nil {
		return fmt.Errorf("invalid manifest: %w", e)
	}
	if m.APIVersion != "blackark/v1" {
		return fmt.Errorf("invalid manifest: apiVersion must be blackark/v1")
	}
	if m.Kind != "App" {
		return fmt.Errorf("invalid manifest: kind must be App")
	}
	if m.Metadata.Name == "" {
		return fmt.Errorf("invalid manifest: metadata.name is required")
	}
	if m.Spec.Image == "" {
		return fmt.Errorf("invalid manifest: spec.image is required")
	}
	n := 1
	if m.Spec.Replicas != nil {
		n = *m.Spec.Replicas
	}
	if n < 0 {
		return fmt.Errorf("invalid manifest: spec.replicas must be non-negative")
	}
	b, e = request(c, http.MethodGet, "/v1/apps", nil)
	if e != nil {
		return e
	}
	var apps []App
	if e = json.Unmarshal(b, &apps); e != nil {
		return e
	}
	for _, a := range apps {
		if a.Name == m.Metadata.Name {
			return raw(c, http.MethodPatch, "/v1/apps/"+url.PathEscape(a.ID), map[string]any{"image": m.Spec.Image, "replicas": n}, out)
		}
	}
	return raw(c, http.MethodPost, "/v1/apps", map[string]any{"name": m.Metadata.Name, "image": m.Spec.Image, "replicas": n}, out)
}
