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
