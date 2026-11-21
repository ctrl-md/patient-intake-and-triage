package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(ctx context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name string
		db   fakeDB
		want int
	}{
		{"database up", fakeDB{}, http.StatusOK},
		{"database down", fakeDB{err: errors.New("no connection")}, http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			NewRouter(Deps{DB: tc.db}).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
