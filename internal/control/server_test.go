package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedStatus(t *testing.T) {
	s := &Server{token: "secret"}
	h := s.auth(http.HandlerFunc(s.status))
	for _, tc := range []struct {
		name, token string
		want        int
	}{{"missing", "", 401}, {"wrong", "nope", 401}, {"valid", "secret", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

func TestDashboardRouteRegistration(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		mux := New(nil, "token")
		r := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatalf("dashboard route found when disabled, got %d", w.Code)
		}
	})
	t.Run("enabled_web_ui_served", func(t *testing.T) {
		mux := New(nil, "token", true)
		r := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("dashboard UI expected 200, got %d", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Fatalf("expected HTML content type, got %q", ct)
		}
	})
	// "enabled" dashboard API endpoint verified by build + needs DB
}
