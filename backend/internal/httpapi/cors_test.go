package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func corsServer() http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return CORS([]string{"http://localhost:3000"})(inner)
}

func TestCORSAllowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	corsServer().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("Vary = %q, want Origin", rec.Header().Get("Vary"))
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	rec := httptest.NewRecorder()
	corsServer().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	h := rec.Header()
	if h.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q", h.Get("Access-Control-Allow-Origin"))
	}
	if h.Get("Access-Control-Allow-Headers") != "Authorization, Content-Type" {
		t.Errorf("Allow-Headers = %q", h.Get("Access-Control-Allow-Headers"))
	}
	if h.Get("Access-Control-Allow-Methods") == "" {
		t.Error("Allow-Methods is empty")
	}
}

func TestCORSOtherOriginGetsNothing(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		req := httptest.NewRequest(method, "/x", nil)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		corsServer().ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s: Allow-Origin = %q, want empty", method, got)
		}
		if rec.Code == http.StatusNoContent {
			t.Errorf("%s: other origins must not get a preflight approval", method)
		}
	}
}

func TestCORSIgnoresRequestsWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	corsServer().ServeHTTP(rec, req)

	if len(rec.Header()) != 0 {
		t.Fatalf("unexpected headers: %v", rec.Header())
	}
}

func TestRouterAppliesCORS(t *testing.T) {
	r := NewRouter(Deps{DB: fakeDB{}, CORSOrigins: []string{"http://localhost:3000"}})

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
}
