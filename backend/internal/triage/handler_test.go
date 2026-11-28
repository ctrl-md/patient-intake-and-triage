package triage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/llm"
)

const (
	secret   = "0123456789abcdef0123456789abcdef"
	intakeID = "33333333-3333-4333-8333-333333333333"
)

// fakeIntakes only implements what the triage handler uses.
type fakeIntakes struct {
	intake.Store
	status string
}

func (f fakeIntakes) GetByID(_ context.Context, id string) (intake.Intake, error) {
	if id != intakeID {
		return intake.Intake{}, intake.ErrNotFound
	}
	return intake.Intake{ID: id, Status: f.status}, nil
}
func (f fakeIntakes) Messages(context.Context, string) ([]intake.Message, error) {
	return chatOf("I have chest pain"), nil
}

func setup(t *testing.T, status string) (http.Handler, string, *memStore) {
	t.Helper()
	st := &memStore{}
	svc := NewService(st, llm.NewFake(llm.Say(goodJSON)))
	mux := http.NewServeMux()
	tm := auth.NewTokenManager(secret, time.Hour)
	Routes(mux, NewHandler(svc, fakeIntakes{status: status}), auth.Middleware(tm))
	tok, _, _ := tm.Issue("11111111-1111-4111-8111-111111111111", "nurse")
	return mux, tok, st
}

func call(h http.Handler, method, path, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRoutesNeedLogin(t *testing.T) {
	h, _, _ := setup(t, intake.StatusCompleted)
	for _, r := range [][2]string{{"GET", "/intakes/" + intakeID + "/triage"}, {"POST", "/intakes/" + intakeID + "/triage/run"}} {
		if rec := call(h, r[0], r[1], ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%v: got %d", r, rec.Code)
		}
	}
}

func TestGetNotFoundThenRunThenGet(t *testing.T) {
	h, tok, _ := setup(t, intake.StatusCompleted)
	p := "/intakes/" + intakeID + "/triage"
	if rec := call(h, "GET", p, tok); rec.Code != http.StatusNotFound {
		t.Fatalf("before run: %d", rec.Code)
	}
	rec := call(h, "POST", p+"/run", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body)
	}
	rec = call(h, "GET", p, tok)
	var body struct {
		Triage Result `json:"triage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Triage.FinalUrgency != Emergency || len(body.Triage.RedFlags) == 0 {
		t.Fatalf("%+v", body.Triage)
	}
}

func TestRunNeedsCompletedIntake(t *testing.T) {
	h, tok, _ := setup(t, intake.StatusInProgress)
	if rec := call(h, "POST", "/intakes/"+intakeID+"/triage/run", tok); rec.Code != http.StatusConflict {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestBadAndUnknownIDs(t *testing.T) {
	h, tok, _ := setup(t, intake.StatusCompleted)
	if rec := call(h, "GET", "/intakes/nope/triage", tok); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d", rec.Code)
	}
	if rec := call(h, "POST", "/intakes/44444444-4444-4444-8444-444444444444/triage/run", tok); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", rec.Code)
	}
}
