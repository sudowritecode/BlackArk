package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var authToken = "test-token-abc"

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"service":"blackark-control","status":"ok"}`))
	})
	mux.HandleFunc("GET /v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":"n1","name":"node-1","status":"healthy"}]`))
	})
	mux.HandleFunc("GET /v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":"a1","name":"my-app","image":"nginx:1.27","replicas":2}]`))
	})
	mux.HandleFunc("GET /v1/apps/a1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"a1","name":"my-app","image":"nginx:1.27","replicas":2,"instances":[{"id":"d1","status":"running"}]}`))
	})
	mux.HandleFunc("GET /v1/apps/a1/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("line1\nline2\nline3\n"))
	})
	mux.HandleFunc("PATCH /v1/apps/a1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"a1","name":"my-app","replicas":5}`))
	})
	mux.HandleFunc("POST /v1/apps/a1/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"id":"a1","status":"restarting"}`))
	})
	mux.HandleFunc("DELETE /v1/apps/a1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"a2","name":"new-app","image":"redis:7","replicas":1}`))
	})
	mux.HandleFunc("POST /v1/join-tokens", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"token":"test-join-token-abc","expires_at":"2026-07-11T12:30:00Z"}`))
	})
	mux.HandleFunc("GET /api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
			"cluster": {"url":"http://control:8080","version":"0.1.0","uptime_seconds":3600},
			"nodes": {
				"healthy":2,"unhealthy":0,"pending":1,"total":3,
				"details":[
					{"id":"n1","name":"node-1","status":"healthy","cpus":4,"cpus_used":2,"mem_bytes":17179869184,"mem_used_bytes":8589934592,"app_count":2,"last_seen_at":"2026-07-08T12:00:00Z"},
					{"id":"n2","name":"node-2","status":"pending","cpus":0,"cpus_used":0,"mem_bytes":0,"mem_used_bytes":0,"app_count":0,"last_seen_at":null}
				]
			},
			"apps": {
				"running":2,"stopped":1,"failed":0,"total":3,
				"details":[
					{"id":"a1","name":"api-gateway","image":"nginx:1.27","desired_replicas":3,"ready_replicas":3,"status":"running"},
					{"id":"a2","name":"worker","image":"redis:7","desired_replicas":2,"ready_replicas":0,"status":"stopped"}
				]
			}
		}`))
	})
	return httptest.NewServer(mux)
}

func writeConfig(t *testing.T, url string) {
	t.Helper()
	dir := t.TempDir()
	os.Setenv("BLACKARK_CONFIG", filepath.Join(dir, "config.yaml"))
	t.Cleanup(func() { os.Unsetenv("BLACKARK_CONFIG") })
	c := Config{URL: url, Token: authToken}
	if err := saveConfig(c); err != nil {
		t.Fatal(err)
	}
}

func TestRunHelp(t *testing.T) {
	err := Run([]string{}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := Run([]string{"bogus"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRunHealth(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	t.Setenv("BLACKARK_CONTROL_URL", srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"health"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["status"] != "ok" {
		t.Fatalf("expected ok, got %v", m)
	}
}

func TestRunLogin(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	dir := t.TempDir()
	os.Setenv("BLACKARK_CONFIG", filepath.Join(dir, "config.yaml"))
	t.Cleanup(func() { os.Unsetenv("BLACKARK_CONFIG") })
	out := new(bytes.Buffer)
	err := Run([]string{"login", "--url", srv.URL, "--token", authToken}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunLoginMissingFlags(t *testing.T) {
	err := Run([]string{"login"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing flags")
	}
}

func TestRunNotLoggedIn(t *testing.T) {
	os.Setenv("BLACKARK_CONFIG", "/nonexistent/blackark/config.yaml")
	t.Cleanup(func() { os.Unsetenv("BLACKARK_CONFIG") })
	err := Run([]string{"get", "apps"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestRunGetNodes(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"get", "nodes"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "node-1") || !strings.Contains(got, "healthy") {
		t.Fatalf("expected node info in table output:\n%s", got)
	}
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "STATUS") {
		t.Fatalf("expected table headers:\n%s", got)
	}
}

func TestRunGetApps(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"get", "apps"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "my-app") || !strings.Contains(got, "nginx") {
		t.Fatalf("expected app info in table output:\n%s", got)
	}
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "REPLICAS") {
		t.Fatalf("expected table headers:\n%s", got)
	}
}

func TestRunGetNodesJSON(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"get", "-o", "json", "nodes"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var nodes []map[string]any
	if err := json.Unmarshal(out.Bytes(), &nodes); err != nil {
		t.Fatalf("expected JSON output: %v", err)
	}
	if len(nodes) != 1 || nodes[0]["name"] != "node-1" {
		t.Fatalf("unexpected nodes: %v", nodes)
	}
}

func TestRunGetAppsJSON(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"get", "-o", "json", "apps"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var apps []map[string]any
	if err := json.Unmarshal(out.Bytes(), &apps); err != nil {
		t.Fatalf("expected JSON output: %v", err)
	}
	if len(apps) != 1 || apps[0]["name"] != "my-app" {
		t.Fatalf("unexpected apps: %v", apps)
	}
}

func TestRunGetInvalidResource(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	err := Run([]string{"get", "pods"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid resource")
	}
}

func TestRunGetMissingArg(t *testing.T) {
	err := Run([]string{"get"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
}

func TestRunDescribeApp(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"describe", "app", "a1"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Name:", "my-app", "ID:", "Instances:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected describe output to contain %q:\n%s", want, got)
		}
	}
}

func TestRunDescribeAppBadUsage(t *testing.T) {
	err := Run([]string{"describe", "app"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	err = Run([]string{"describe", "node", "n1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for non-app resource")
	}
}

func TestRunLogs(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"logs", "a1"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "line1\nline2\nline3\n" {
		t.Fatalf("unexpected logs: %q", out.String())
	}
}

func TestRunLogsWithTail(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"logs", "--tail", "100", "a1"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "line1\nline2\nline3\n" {
		t.Fatalf("unexpected logs: %q", out.String())
	}
}

func TestRunLogsBadTail(t *testing.T) {
	err := Run([]string{"logs", "--tail", "99999", "a1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for out-of-range tail")
	}
	err = Run([]string{"logs", "--tail", "-1", "a1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for negative tail")
	}
}

func TestRunLogsMissingApp(t *testing.T) {
	err := Run([]string{"logs"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing app id")
	}
}

func TestRunScale(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"scale", "a1", "5"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var app map[string]any
	if err := json.Unmarshal(out.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app["replicas"] != float64(5) {
		t.Fatalf("expected 5 replicas, got %v", app["replicas"])
	}
}

func TestRunScaleBadUsage(t *testing.T) {
	err := Run([]string{"scale", "a1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing replicas")
	}
	err = Run([]string{"scale", "a1", "foo"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid replicas")
	}
	err = Run([]string{"scale", "a1", "-1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for negative replicas")
	}
}

func TestRunRestart(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"restart", "a1"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var app map[string]any
	if err := json.Unmarshal(out.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app["status"] != "restarting" {
		t.Fatalf("unexpected response: %v", app)
	}
}

func TestRunRestartBadUsage(t *testing.T) {
	err := Run([]string{"restart"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing app id")
	}
}

func TestRunDeleteApp(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"delete", "app", "a1"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "app deleted\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunDeleteAppBadUsage(t *testing.T) {
	err := Run([]string{"delete", "app"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	err = Run([]string{"delete", "node", "n1"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for non-app resource")
	}
}

func validManifest(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "app.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v1
kind: App
metadata:
  name: new-app
spec:
  image: redis:7
  replicas: 1
`), 0644)
	return p
}

func TestRunApplyCreate(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	dir := t.TempDir()
	p := validManifest(t, dir)
	out := new(bytes.Buffer)
	err := Run([]string{"apply", "-f", p}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var app map[string]any
	if err := json.Unmarshal(out.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app["name"] != "new-app" {
		t.Fatalf("unexpected app: %v", app)
	}
}

func TestRunApplyMissingFlag(t *testing.T) {
	err := Run([]string{"apply"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing -f flag")
	}
}

func TestRunApplyInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: v1
kind: Pod
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid manifest")
	}
}

func TestRunApplyMissingName(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v1
kind: App
metadata:
  name: ""
spec:
  image: redis:7
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRunApplyMissingImage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v1
kind: App
metadata:
  name: my-app
spec:
  replicas: 1
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing image")
	}
}

func TestRunApplyBadAPIVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v2
kind: App
metadata:
  name: my-app
spec:
  image: redis:7
  replicas: 1
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for bad apiVersion")
	}
}

func TestRunApplyBadKind(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v1
kind: Deployment
metadata:
  name: my-app
spec:
  image: redis:7
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for bad kind")
	}
}

func TestRunApplyUnknownFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	os.WriteFile(p, []byte(`apiVersion: blackark/v1
kind: App
metadata:
  name: my-app
spec:
  image: redis:7
  ports:
    - 80
`), 0644)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for unknown fields")
	}
}

func TestApplyCreatesViaPost(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.Method == "GET" {
			w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()
	writeConfig(t, srv.URL)
	dir := t.TempDir()
	p := validManifest(t, dir)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if method != "POST" {
		t.Fatalf("expected POST for new app, got %s", method)
	}
}

func TestRunDashboard(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"dashboard"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"BlackArk Cluster", "node-1", "node-2", "api-gateway", "worker", "healthy", "pending", "running", "stopped"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q:\n%s", want, got)
		}
	}
}

func TestRunDashboardAuth(t *testing.T) {
	out := new(bytes.Buffer)
	err := Run([]string{"dashboard"}, nil, out, io.Discard)
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestRenderDashboard(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	c := Config{URL: srv.URL, Token: authToken}
	out := new(bytes.Buffer)
	err := renderDashboard(c, out, false)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"BlackArk Cluster", "node-1", "api-gateway", "healthy", "running"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q:\n%s", want, got)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{{0, "0B"}, {500, "500B"}, {2048, "2.0KB"}, {1048576, "1.0MB"}, {1073741824, "1.0GB"}, {1099511627776, "1.0TB"}} {
		got := formatBytes(tc.in)
		if got != tc.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestApplyUpdatesViaPatch(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.Method == "GET" {
			w.Write([]byte(`[{"id":"existing","name":"new-app","image":"redis:6","replicas":2}]`))
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	writeConfig(t, srv.URL)
	dir := t.TempDir()
	p := validManifest(t, dir)
	err := Run([]string{"apply", "-f", p}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if method != "PATCH" {
		t.Fatalf("expected PATCH for existing app, got %s", method)
	}
}

func TestRunJoinToken(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"join-token"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Token:", "test-join-token-abc", "TTL:", "600s", "Expires:", "docker run", "BLACKARK_JOIN_TOKEN=test-join-token-abc", srv.URL} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q:\n%s", want, got)
		}
	}
}

func TestRunJoinTokenCustomTTL(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"join-token", "--ttl", "120"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "120s") {
		t.Fatalf("expected custom TTL in output:\n%s", got)
	}
}

func TestRunJoinTokenJSON(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	writeConfig(t, srv.URL)
	out := new(bytes.Buffer)
	err := Run([]string{"join-token", "-o", "json"}, nil, out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON output: %v", err)
	}
	if resp["token"] != "test-join-token-abc" {
		t.Fatalf("unexpected token: %v", resp["token"])
	}
}

func TestRunJoinTokenNotLoggedIn(t *testing.T) {
	os.Setenv("BLACKARK_CONFIG", "/nonexistent/blackark/config.yaml")
	t.Cleanup(func() { os.Unsetenv("BLACKARK_CONFIG") })
	err := Run([]string{"join-token"}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}
