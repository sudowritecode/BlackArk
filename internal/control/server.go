package control

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed webui
var dashboardFS embed.FS

// Version is the build version, set via ldflags at build time.
var Version = "dev"

type Server struct {
	db               *pgxpool.Pool
	token            string
	dashboardEnabled bool
	startedAt        time.Time
	events           *Ring
}
type instance struct {
	ID          string  `json:"id"`
	NodeID      *string `json:"node_id"`
	ContainerID *string `json:"container_id"`
	Status      string  `json:"status"`
	Logs        string  `json:"logs,omitempty"`
}
type app struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Image     string     `json:"image"`
	Replicas  int        `json:"replicas"`
	Instances []instance `json:"instances,omitempty"`
}

func New(db *pgxpool.Pool, token string, opts ...bool) http.Handler {
	dash := len(opts) > 0 && opts[0]
	s := &Server{db: db, token: token, dashboardEnabled: dash, startedAt: time.Now(), events: NewRing(256)}
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", s.health)
	m.Handle("GET /api/v1/status", s.auth(http.HandlerFunc(s.status)))
	m.Handle("GET /api/v1/activity", s.auth(http.HandlerFunc(s.activity)))
	m.Handle("POST /v1/join-tokens", s.auth(http.HandlerFunc(s.createJoinToken)))
	m.HandleFunc("POST /v1/nodes/join", s.join)
	m.HandleFunc("POST /v1/nodes/{id}/heartbeat", s.heartbeat)
	m.Handle("GET /v1/nodes", s.auth(http.HandlerFunc(s.nodes)))
	m.Handle("POST /v1/apps", s.auth(http.HandlerFunc(s.createApp)))
	m.Handle("GET /v1/apps", s.auth(http.HandlerFunc(s.apps)))
	m.Handle("GET /v1/apps/{id}", s.auth(http.HandlerFunc(s.getApp)))
	m.Handle("PATCH /v1/apps/{id}", s.auth(http.HandlerFunc(s.scaleApp)))
	m.Handle("POST /v1/apps/{id}/deploy", s.auth(http.HandlerFunc(s.deployApp)))
	m.Handle("POST /v1/apps/{id}/restart", s.auth(http.HandlerFunc(s.deployApp)))
	m.Handle("GET /v1/apps/{id}/logs", s.auth(http.HandlerFunc(s.logs)))
	m.Handle("DELETE /v1/apps/{id}", s.auth(http.HandlerFunc(s.deleteApp)))
	if s.dashboardEnabled {
		m.HandleFunc("GET /api/v1/dashboard", s.dashboard)
		dashUI, _ := fs.Sub(dashboardFS, "webui")
		m.Handle("GET /dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.FS(dashUI))))
		m.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard/", 301)
		})
	}
	return m
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "blackark-control", "status": "ok"})
}
func (s *Server) auth(n http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		n.ServeHTTP(w, r)
	})
}
func hash(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
func secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

func (s *Server) createJoinToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	_ = decode(r, &in)
	if in.TTLSeconds == 0 {
		in.TTLSeconds = 600
	}
	if in.TTLSeconds < 1 || in.TTLSeconds > 86400 {
		writeError(w, 400, "ttl_seconds must be 1..86400")
		return
	}
	t := secret()
	exp := time.Now().Add(time.Duration(in.TTLSeconds) * time.Second)
	if _, err := s.db.Exec(r.Context(), `INSERT INTO join_tokens(token_hash,expires_at) VALUES($1,$2)`, hash(t), exp); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"token": t, "expires_at": exp})
}
func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Token string }
	if decode(r, &in) != nil || in.Name == "" || in.Token == "" {
		writeError(w, 400, "name and token are required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	var ok bool
	err = tx.QueryRow(r.Context(), `UPDATE join_tokens SET used_at=now() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>now() RETURNING true`, hash(in.Token)).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 401, "invalid or expired join token")
		return
	}
	cred := secret()
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO nodes(name,status,agent_token_hash,last_seen_at) VALUES($1,'healthy',$2,now()) RETURNING id`, in.Name, hash(cred)).Scan(&id)
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	s.events.Push(Event{
		Type:     "node_joined",
		Message:  "Node " + in.Name + " joined the cluster",
		NodeID:   id,
		NodeName: in.Name,
	})
	writeJSON(w, 201, map[string]string{"id": id, "credential": cred})
}
func (s *Server) agent(r *http.Request, id string) bool {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var want []byte
	return got != "" && s.db.QueryRow(r.Context(), `SELECT agent_token_hash FROM nodes WHERE id=$1`, id).Scan(&want) == nil && subtle.ConstantTimeCompare(hash(got), want) == 1
}
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.agent(r, id) {
		writeError(w, 401, "unauthorized")
		return
	}
	var in struct {
		Capacity, Resources map[string]any
		Instances           []instance `json:"instances"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	capJSON, _ := json.Marshal(in.Capacity)
	resJSON, _ := json.Marshal(in.Resources)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `UPDATE nodes SET status='healthy',last_seen_at=now(),capacity=$2,resources=$3,updated_at=now() WHERE id=$1`, id, capJSON, resJSON)
	for _, x := range in.Instances {
		if err == nil {
			_, err = tx.Exec(r.Context(), `UPDATE deployments SET container_id=$2,status=$3,logs=$5,updated_at=now() WHERE id=$1 AND node_id=$4`, x.ID, x.ContainerID, x.Status, id, x.Logs)
		}
	}
	if err == nil {
		err = s.reconcile(r.Context(), tx)
	}
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT d.id,a.image,a.name,d.status FROM deployments d JOIN apps a ON a.id=d.app_id WHERE d.node_id=$1 AND d.status IN ('pending','delete') ORDER BY d.created_at`, id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	actions := []map[string]string{}
	for rows.Next() {
		var did, img, appName, status string
		_ = rows.Scan(&did, &img, &appName, &status)
		op := "run"
		if status == "delete" {
			op = "delete"
		}
		actions = append(actions, map[string]string{"deployment_id": did, "image": img, "operation": op})
		s.events.Push(Event{
			Type:    "deployment_" + op,
			Message: "Instance " + did[:8] + " of " + appName + " " + op + " on node " + id[:8],
			AppName: appName,
			NodeID:  id,
		})
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"actions": actions})
}

func (s *Server) reconcile(ctx interface{ Done() <-chan struct{} }, tx pgx.Tx) error { // pgx accepts context.Context; adapter keeps signature concise
	c := ctx.(interface {
		Done() <-chan struct{}
		Err() error
		Value(any) any
		Deadline() (time.Time, bool)
	})
	rows, err := tx.Query(c, `SELECT id,name FROM nodes WHERE last_seen_at < now()-interval '30 seconds' AND status='healthy'`)
	if err != nil {
		return err
	}
	var unhealthies []Event
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		unhealthies = append(unhealthies, Event{
			Type:     "node_unhealthy",
			Message:  "Node " + name + " marked unhealthy (30s timeout)",
			NodeID:   id,
			NodeName: name,
		})
	}
	rows.Close()
	for _, e := range unhealthies {
		s.events.Push(e)
	}
	_, err = tx.Exec(c, `UPDATE nodes SET status='unhealthy',updated_at=now() WHERE last_seen_at < now()-interval '30 seconds' AND status='healthy'`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(c, `UPDATE deployments SET status='pending',node_id=NULL,container_id=NULL,updated_at=now() WHERE status IN ('running','pending') AND node_id IN (SELECT id FROM nodes WHERE status!='healthy')`)
	if err != nil {
		return err
	}
	rows, err = tx.Query(c, `SELECT id,desired_replicas FROM apps`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type ar struct {
		id string
		n  int
	}
	var aa []ar
	for rows.Next() {
		var a ar
		if err = rows.Scan(&a.id, &a.n); err != nil {
			return err
		}
		aa = append(aa, a)
	}
	rows.Close()
	for _, a := range aa {
		unassigned, qerr := tx.Query(c, `SELECT id FROM deployments WHERE app_id=$1 AND status='pending' AND node_id IS NULL ORDER BY created_at`, a.id)
		if qerr != nil {
			return qerr
		}
		var pending []string
		for unassigned.Next() {
			var id string
			if qerr = unassigned.Scan(&id); qerr != nil {
				unassigned.Close()
				return qerr
			}
			pending = append(pending, id)
		}
		unassigned.Close()
		for _, deploymentID := range pending {
			var node string
			qerr = tx.QueryRow(c, `SELECT n.id FROM nodes n LEFT JOIN deployments x ON x.node_id=n.id AND x.status!='delete' WHERE n.status='healthy' GROUP BY n.id ORDER BY count(x.id),n.last_seen_at DESC LIMIT 1`).Scan(&node)
			if errors.Is(qerr, pgx.ErrNoRows) {
				break
			}
			if qerr != nil {
				return qerr
			}
			if _, qerr = tx.Exec(c, `UPDATE deployments SET node_id=$2,updated_at=now() WHERE id=$1`, deploymentID, node); qerr != nil {
				return qerr
			}
		}
		var count int
		if err = tx.QueryRow(c, `SELECT count(*) FROM deployments WHERE app_id=$1 AND status!='delete'`, a.id).Scan(&count); err != nil {
			return err
		}
		for count < a.n {
			var node string
			err = tx.QueryRow(c, `SELECT n.id FROM nodes n LEFT JOIN deployments d ON d.node_id=n.id AND d.status!='delete' WHERE n.status='healthy' GROUP BY n.id ORDER BY count(d.id),n.last_seen_at DESC LIMIT 1`).Scan(&node)
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			if _, err = tx.Exec(c, `INSERT INTO deployments(app_id,node_id,status) VALUES($1,$2,'pending')`, a.id, node); err != nil {
				return err
			}
			count++
		}
		if count > a.n {
			_, err = tx.Exec(c, `UPDATE deployments SET status='delete',updated_at=now() WHERE id IN (SELECT id FROM deployments WHERE app_id=$1 AND status!='delete' ORDER BY created_at DESC LIMIT $2)`, a.id, count-a.n)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT id,name,status,capacity,resources,last_seen_at FROM nodes ORDER BY name`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, n, st string
		var c, res []byte
		var seen *time.Time
		_ = rows.Scan(&id, &n, &st, &c, &res, &seen)
		out = append(out, map[string]any{"id": id, "name": n, "status": st, "capacity": json.RawMessage(c), "resources": json.RawMessage(res), "last_seen_at": seen})
	}
	writeJSON(w, 200, out)
}
func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name, Image string
		Replicas    int
	}
	if err := decode(r, &in); err != nil || in.Name == "" || in.Image == "" || in.Replicas < 0 {
		writeError(w, 400, "name, image, and non-negative replicas required")
		return
	}
	var a app
	err := s.db.QueryRow(r.Context(), `INSERT INTO apps(name,image,desired_replicas) VALUES($1,$2,$3) RETURNING id,name,image,desired_replicas`, in.Name, in.Image, in.Replicas).Scan(&a.ID, &a.Name, &a.Image, &a.Replicas)
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	s.events.Push(Event{
		Type:    "app_created",
		Message: "App " + a.Name + " created with " + strconv.Itoa(a.Replicas) + " replicas",
		AppID:   a.ID,
		AppName: a.Name,
	})
	writeJSON(w, 201, a)
}
func (s *Server) apps(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT id,name,image,desired_replicas FROM apps ORDER BY name`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []app{}
	for rows.Next() {
		var a app
		_ = rows.Scan(&a.ID, &a.Name, &a.Image, &a.Replicas)
		out = append(out, a)
	}
	writeJSON(w, 200, out)
}

type dashboardResponse struct {
	Cluster clusterInfo `json:"cluster"`
	Nodes   nodeSection `json:"nodes"`
	Apps    appSection  `json:"apps"`
}
type clusterInfo struct {
	URL        string `json:"url"`
	Version    string `json:"version"`
	UptimeSecs int64  `json:"uptime_seconds"`
}
type nodeSection struct {
	Healthy   int          `json:"healthy"`
	Unhealthy int          `json:"unhealthy"`
	Pending   int          `json:"pending"`
	Total     int          `json:"total"`
	Details   []nodeDetail `json:"details"`
}
type nodeDetail struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Status   string     `json:"status"`
	CPUs     float64    `json:"cpus"`
	CPUsUsed float64    `json:"cpus_used"`
	MemBytes int64      `json:"mem_bytes"`
	MemUsed  int64      `json:"mem_used_bytes"`
	AppCount int        `json:"app_count"`
	LastSeen *time.Time `json:"last_seen_at"`
}
type appSection struct {
	Running int         `json:"running"`
	Stopped int         `json:"stopped"`
	Failed  int         `json:"failed"`
	Total   int         `json:"total"`
	Details []appDetail `json:"details"`
}
type appDetail struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Image           string `json:"image"`
	DesiredReplicas int    `json:"desired_replicas"`
	ReadyReplicas   int    `json:"ready_replicas"`
	Status          string `json:"status"`
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	var d dashboardResponse
	d.Cluster = clusterInfo{
		URL:        r.Host,
		Version:    Version,
		UptimeSecs: int64(time.Since(s.startedAt).Seconds()),
	}

	// Node counts
	rows, err := s.db.Query(r.Context(), `SELECT status,count(*) FROM nodes GROUP BY status`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		_ = rows.Scan(&st, &n)
		d.Nodes.Total += n
		switch st {
		case "healthy":
			d.Nodes.Healthy = n
		case "unhealthy":
			d.Nodes.Unhealthy = n
		default:
			d.Nodes.Pending += n
		}
	}
	rows.Close()

	// Node details
	nrows, err := s.db.Query(r.Context(), `
		SELECT n.id,n.name,n.status,n.capacity,n.resources,n.last_seen_at,
			COALESCE((SELECT count(*) FROM deployments x WHERE x.node_id=n.id AND x.status!='delete'),0)
		FROM nodes n ORDER BY n.name`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer nrows.Close()
	for nrows.Next() {
		var nd nodeDetail
		var capJSON, resJSON []byte
		var seen *time.Time
		if err := nrows.Scan(&nd.ID, &nd.Name, &nd.Status, &capJSON, &resJSON, &seen, &nd.AppCount); err != nil {
			continue
		}
		nd.LastSeen = seen
		if capJSON != nil {
			var cap map[string]any
			if json.Unmarshal(capJSON, &cap) == nil {
				if v, ok := cap["cpus"].(float64); ok {
					nd.CPUs = v
				}
				if v, ok := cap["memory_bytes"].(float64); ok {
					nd.MemBytes = int64(v)
				}
			}
		}
		if resJSON != nil {
			var res map[string]any
			if json.Unmarshal(resJSON, &res) == nil {
				if v, ok := res["cpus"].(float64); ok {
					nd.CPUsUsed = v
				}
				if v, ok := res["memory_bytes"].(float64); ok {
					nd.MemUsed = int64(v)
				}
			}
		}
		d.Nodes.Details = append(d.Nodes.Details, nd)
	}
	nrows.Close()

	// App details with ready replica count
	arows, err := s.db.Query(r.Context(), `
		SELECT a.id,a.name,a.image,a.desired_replicas,
			COALESCE((SELECT count(*) FROM deployments x WHERE x.app_id=a.id AND x.status='running'),0)
		FROM apps a ORDER BY a.name`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer arows.Close()
	for arows.Next() {
		var ad appDetail
		if err := arows.Scan(&ad.ID, &ad.Name, &ad.Image, &ad.DesiredReplicas, &ad.ReadyReplicas); err != nil {
			continue
		}
		switch {
		case ad.ReadyReplicas > 0 && ad.ReadyReplicas >= ad.DesiredReplicas:
			ad.Status = "running"
		case ad.ReadyReplicas > 0:
			ad.Status = "degraded"
		case ad.DesiredReplicas == 0:
			ad.Status = "stopped"
		default:
			ad.Status = "pending"
		}
		d.Apps.Details = append(d.Apps.Details, ad)
	}
	arows.Close()

	d.Apps.Total = len(d.Apps.Details)
	for _, ad := range d.Apps.Details {
		switch ad.Status {
		case "running":
			d.Apps.Running++
		case "stopped":
			d.Apps.Stopped++
		case "degraded", "pending":
			d.Apps.Failed++
		}
	}
	writeJSON(w, 200, d)
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	n := 50
	if m, _ := strconv.Atoi(r.URL.Query().Get("limit")); m > 0 && m <= 500 {
		n = m
	}
	writeJSON(w, 200, s.events.Recent(n))
}

func (s *Server) loadApp(r *http.Request) (app, error) {
	var a app
	err := s.db.QueryRow(r.Context(), `SELECT id,name,image,desired_replicas FROM apps WHERE id=$1`, r.PathValue("id")).Scan(&a.ID, &a.Name, &a.Image, &a.Replicas)
	if err != nil {
		return a, err
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,node_id,container_id,status FROM deployments WHERE app_id=$1 ORDER BY created_at`, a.ID)
	if err != nil {
		return a, err
	}
	defer rows.Close()
	for rows.Next() {
		var x instance
		if err = rows.Scan(&x.ID, &x.NodeID, &x.ContainerID, &x.Status); err != nil {
			return a, err
		}
		a.Instances = append(a.Instances, x)
	}
	return a, nil
}
func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	a, err := s.loadApp(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "app not found")
		return
	}
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, a)
}
func (s *Server) scaleApp(w http.ResponseWriter, r *http.Request) {
	var in struct{ Replicas int }
	if err := decode(r, &in); err != nil || in.Replicas < 0 {
		writeError(w, 400, "non-negative replicas required")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE apps SET desired_replicas=$2,updated_at=now() WHERE id=$1`, r.PathValue("id"), in.Replicas)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "app not found")
		return
	}
	var name string
	_ = s.db.QueryRow(r.Context(), `SELECT name FROM apps WHERE id=$1`, r.PathValue("id")).Scan(&name)
	s.events.Push(Event{
		Type:    "app_scaled",
		Message: "App " + name + " scaled to " + strconv.Itoa(in.Replicas) + " replicas",
		AppID:   r.PathValue("id"),
		AppName: name,
	})
	s.deployApp(w, r)
}
func (s *Server) deployApp(w http.ResponseWriter, r *http.Request) {
	tx, err := s.db.Begin(r.Context())
	if err == nil {
		err = s.reconcile(r.Context(), tx)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	} else if tx != nil {
		_ = tx.Rollback(r.Context())
	}
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	a, err := s.loadApp(r)
	if err != nil {
		writeError(w, 404, "app not found")
		return
	}
	s.events.Push(Event{
		Type:    "app_deployed",
		Message: "App " + a.Name + " deployed",
		AppID:   a.ID,
		AppName: a.Name,
	})
	writeJSON(w, 202, a)
}
func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	var name string
	_ = s.db.QueryRow(r.Context(), `SELECT name FROM apps WHERE id=$1`, r.PathValue("id")).Scan(&name)
	tag, err := s.db.Exec(r.Context(), `UPDATE deployments SET status='delete' WHERE app_id=$1`, r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "app not found")
		return
	}
	_, _ = s.db.Exec(r.Context(), `UPDATE apps SET desired_replicas=0 WHERE id=$1`, r.PathValue("id"))
	s.events.Push(Event{
		Type:    "app_deleted",
		Message: "App " + name + " deleted",
		AppID:   r.PathValue("id"),
		AppName: name,
	})
	w.WriteHeader(204)
}
func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if n, _ := strconv.Atoi(r.URL.Query().Get("tail")); n > 0 && n <= 5000 {
		limit = n
	}
	rows, err := s.db.Query(r.Context(), `SELECT logs FROM deployments WHERE app_id=$1 ORDER BY created_at`, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var x string
		_ = rows.Scan(&x)
		b.WriteString(x)
	}
	lines := strings.Split(b.String(), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprintln(w, strings.Join(lines, "\n"))
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
