package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
)

const (
	testSecret = "0123456789abcdef0123456789abcdef"
	staffID    = "11111111-1111-4111-8111-111111111111"
)

// ---------- fake store ----------

type fakeStore struct {
	intakes  map[string]*Intake
	hashes   map[string]string // token hash -> intake id
	messages map[string][]Message
	created  []NewIntake // what was passed to CreateIntake
	n        int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		intakes:  map[string]*Intake{},
		hashes:   map[string]string{},
		messages: map[string][]Message{},
	}
}

func (f *fakeStore) nextID() string {
	f.n++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.n)
}

func (f *fakeStore) CreateIntake(_ context.Context, in NewIntake) (Intake, error) {
	f.created = append(f.created, in)
	it := &Intake{ID: f.nextID(), Status: StatusPending, ExpiresAt: in.ExpiresAt, CreatedAt: time.Now()}
	if in.Patient != nil {
		name := in.Patient.FullName
		it.PatientName = &name
	}
	f.intakes[it.ID] = it
	f.hashes[in.TokenHash] = it.ID
	return *it, nil
}

func (f *fakeStore) get(id string) (Intake, error) {
	it, ok := f.intakes[id]
	if !ok {
		return Intake{}, ErrNotFound
	}
	out := *it
	out.MessageCount = len(f.messages[id])
	return out, nil
}

func (f *fakeStore) GetByTokenHash(_ context.Context, hash string) (Intake, error) {
	id, ok := f.hashes[hash]
	if !ok {
		return Intake{}, ErrNotFound
	}
	return f.get(id)
}

func (f *fakeStore) GetByID(_ context.Context, id string) (Intake, error) { return f.get(id) }

func (f *fakeStore) List(_ context.Context, status string, limit int) ([]Intake, error) {
	var out []Intake
	for id := range f.intakes {
		it, _ := f.get(id)
		if status == "" || it.Status == status {
			out = append(out, it)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) Messages(_ context.Context, id string) ([]Message, error) {
	return f.messages[id], nil
}

func (f *fakeStore) AddMessage(_ context.Context, id, role, content string, max int) (Message, error) {
	if len(f.messages[id]) >= max {
		return Message{}, ErrTooManyMessages
	}
	m := Message{ID: f.nextID(), Role: role, Content: content, CreatedAt: time.Now()}
	f.messages[id] = append(f.messages[id], m)
	return m, nil
}

func (f *fakeStore) Start(_ context.Context, id string) error {
	if it := f.intakes[id]; it.Status == StatusPending {
		it.Status = StatusInProgress
	}
	return nil
}

func (f *fakeStore) Complete(_ context.Context, id string) error {
	f.intakes[id].Status = StatusCompleted
	return nil
}

// ---------- test setup ----------

type env struct {
	mux   http.Handler
	store *fakeStore
	h     *Handler
	staff string // staff login token
}

// newEnv uses the real auth middleware and the real Routes function,
// so these tests also check which routes need a login.
func newEnv(t *testing.T) *env {
	t.Helper()
	store := newFakeStore()
	h := NewHandler(store, "https://triagely.test/", 24*time.Hour)

	tm := auth.NewTokenManager(testSecret, time.Hour)
	staff, _, err := tm.Issue(staffID, "nurse")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	Routes(mux, h, auth.Middleware(tm))
	return &env{mux: mux, store: store, h: h, staff: staff}
}

func (e *env) do(method, path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *env) staffDo(method, path, body string) *httptest.ResponseRecorder {
	return e.do(method, path, body, e.staff)
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad json %q: %v", rec.Body.String(), err)
	}
	return m
}

// newIntake creates an intake as staff and returns its id and link token.
func (e *env) newIntake(t *testing.T, body string) (id, token string) {
	t.Helper()
	rec := e.staffDo("POST", "/intakes", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	return m["intake"].(map[string]any)["id"].(string), m["token"].(string)
}

func publicPath(token, suffix string) string { return "/public/intakes/" + token + suffix }

// ---------- staff: create ----------

func TestCreateIntake(t *testing.T) {
	e := newEnv(t)

	rec := e.staffDo("POST", "/intakes", "") // no body at all
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	token := m["token"].(string)

	if len(token) != tokenLength {
		t.Errorf("token length = %d", len(token))
	}
	if link := m["link"].(string); link != "https://triagely.test/intake/"+token {
		t.Errorf("link = %q", link)
	}
	if st := m["intake"].(map[string]any)["status"]; st != "pending" {
		t.Errorf("status = %v", st)
	}

	// Only the hash is saved, never the token.
	saved := e.store.created[0]
	if saved.TokenHash != HashToken(token) || saved.TokenHash == token {
		t.Errorf("stored token_hash is wrong: %q", saved.TokenHash)
	}
	if saved.CreatedBy != staffID {
		t.Errorf("created_by = %q, want the logged-in staff id", saved.CreatedBy)
	}
	if saved.Patient != nil {
		t.Error("no patient was given, none should be saved")
	}
	if d := time.Until(saved.ExpiresAt); d < 23*time.Hour || d > 25*time.Hour {
		t.Errorf("expires in %v, want about 24h", d)
	}
}

func TestCreateIntakeWithPatient(t *testing.T) {
	e := newEnv(t)
	body := `{"patient":{"full_name":" Emeka Eze ","phone":"08012345678","date_of_birth":"1990-05-17","sex":"Male"}}`

	id, _ := e.newIntake(t, body)

	p := e.store.created[0].Patient
	if p == nil || p.FullName != "Emeka Eze" || *p.Phone != "08012345678" || *p.Sex != "male" {
		t.Fatalf("patient not saved right: %+v", p)
	}
	if p.DateOfBirth.Format("2006-01-02") != "1990-05-17" {
		t.Errorf("dob = %v", p.DateOfBirth)
	}

	rec := e.staffDo("GET", "/intakes/"+id, "")
	if name := jsonBody(t, rec)["intake"].(map[string]any)["patient_name"]; name != "Emeka Eze" {
		t.Errorf("patient_name = %v", name)
	}
}

func TestCreateIntakeValidation(t *testing.T) {
	e := newEnv(t)
	tests := map[string]string{
		"not json":     `nope`,
		"empty name":   `{"patient":{"full_name":"  "}}`,
		"bad dob":      `{"patient":{"full_name":"A","date_of_birth":"17/05/1990"}}`,
		"future dob":   `{"patient":{"full_name":"A","date_of_birth":"2999-01-01"}}`,
		"ancient dob":  `{"patient":{"full_name":"A","date_of_birth":"1700-01-01"}}`,
		"bad sex":      `{"patient":{"full_name":"A","sex":"robot"}}`,
		"long phone":   `{"patient":{"full_name":"A","phone":"` + strings.Repeat("1", 31) + `"}}`,
		"missing name": `{"patient":{"phone":"123"}}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if rec := e.staffDo("POST", "/intakes", body); rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", rec.Code, rec.Body)
			}
		})
	}
	if len(e.store.created) != 0 {
		t.Fatal("nothing should be saved when validation fails")
	}
}

func TestStaffRoutesNeedLogin(t *testing.T) {
	e := newEnv(t)
	id, _ := e.newIntake(t, "")

	for _, route := range []string{"POST /intakes", "GET /intakes", "GET /intakes/" + id} {
		method, path, _ := strings.Cut(route, " ")
		if rec := e.do(method, path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without login: got %d, want 401", route, rec.Code)
		}
		if rec := e.do(method, path, "", "junk"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s with junk token: got %d, want 401", route, rec.Code)
		}
	}
}

// ---------- patient flow ----------

func TestPatientFlow(t *testing.T) {
	e := newEnv(t)
	id, token := e.newIntake(t, "")

	// Link works with no login, and starts empty.
	rec := e.do("GET", publicPath(token, ""), "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	if m["status"] != "pending" || len(m["messages"].([]any)) != 0 {
		t.Fatalf("unexpected start state: %v", m)
	}

	// First message starts the intake.
	rec = e.do("POST", publicPath(token, "/messages"), `{"content":"  I have chest pain  "}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	msg := jsonBody(t, rec)["message"].(map[string]any)
	if msg["role"] != "patient" || msg["content"] != "I have chest pain" {
		t.Fatalf("message = %v", msg)
	}
	if e.store.intakes[id].Status != StatusInProgress {
		t.Fatalf("status = %q, want in_progress", e.store.intakes[id].Status)
	}

	// Refreshing the page shows the message.
	m = jsonBody(t, e.do("GET", publicPath(token, ""), "", ""))
	if m["status"] != "in_progress" || len(m["messages"].([]any)) != 1 {
		t.Fatalf("after send: %v", m)
	}

	// Finish. Calling it twice is fine.
	for i := 0; i < 2; i++ {
		rec = e.do("POST", publicPath(token, "/complete"), "", "")
		if rec.Code != http.StatusOK || jsonBody(t, rec)["status"] != "completed" {
			t.Fatalf("complete #%d: %d %s", i+1, rec.Code, rec.Body)
		}
	}

	// After finishing, no more messages.
	rec = e.do("POST", publicPath(token, "/messages"), `{"content":"one more thing"}`, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("send after complete: got %d, want 409", rec.Code)
	}
}

func TestPublicUnknownLinks(t *testing.T) {
	e := newEnv(t)
	_, real := e.newIntake(t, "")

	wrongLength := "short"
	unknown := strings.Repeat("a", tokenLength)
	if unknown == real {
		unknown = strings.Repeat("b", tokenLength)
	}

	for _, token := range []string{wrongLength, unknown} {
		for _, call := range [][2]string{{"GET", ""}, {"POST", "/messages"}, {"POST", "/complete"}} {
			rec := e.do(call[0], publicPath(token, call[1]), `{"content":"hi"}`, "")
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %q: got %d, want 404", call[0], token, rec.Code)
			}
		}
	}
}

func TestExpiredLink(t *testing.T) {
	e := newEnv(t)
	_, token := e.newIntake(t, "")

	e.h.now = func() time.Time { return time.Now().Add(25 * time.Hour) }

	for _, call := range [][2]string{{"GET", ""}, {"POST", "/messages"}, {"POST", "/complete"}} {
		rec := e.do(call[0], publicPath(token, call[1]), `{"content":"hi"}`, "")
		if rec.Code != http.StatusGone {
			t.Errorf("%s: got %d, want 410", call[0], rec.Code)
		}
	}
}

func TestExpiredStatusShownToStaff(t *testing.T) {
	e := newEnv(t)
	id, _ := e.newIntake(t, "")
	e.h.now = func() time.Time { return time.Now().Add(25 * time.Hour) }

	rec := e.staffDo("GET", "/intakes/"+id, "")
	if st := jsonBody(t, rec)["intake"].(map[string]any)["status"]; st != "expired" {
		t.Fatalf("status = %v, want expired", st)
	}
}

func TestSendValidation(t *testing.T) {
	e := newEnv(t)
	_, token := e.newIntake(t, "")

	tests := map[string]string{
		"not json":   `nope`,
		"empty":      `{"content":""}`,
		"only space": `{"content":"   "}`,
		"too long":   `{"content":"` + strings.Repeat("x", maxMessageChars+1) + `"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if rec := e.do("POST", publicPath(token, "/messages"), body, ""); rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", rec.Code)
			}
		})
	}

	// Exactly at the limit is fine, and counts characters, not bytes.
	ok := `{"content":"` + strings.Repeat("é", maxMessageChars) + `"}`
	if rec := e.do("POST", publicPath(token, "/messages"), ok, ""); rec.Code != http.StatusCreated {
		t.Fatalf("at the limit: got %d, want 201", rec.Code)
	}
}

func TestMessageLimit(t *testing.T) {
	e := newEnv(t)
	e.h.maxMessages = 3
	_, token := e.newIntake(t, "")

	for i := 0; i < 3; i++ {
		if rec := e.do("POST", publicPath(token, "/messages"), `{"content":"hi"}`, ""); rec.Code != http.StatusCreated {
			t.Fatalf("message %d: got %d", i+1, rec.Code)
		}
	}
	if rec := e.do("POST", publicPath(token, "/messages"), `{"content":"hi"}`, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: got %d, want 429", rec.Code)
	}
}

func TestCompleteNeedsAMessage(t *testing.T) {
	e := newEnv(t)
	_, token := e.newIntake(t, "")

	if rec := e.do("POST", publicPath(token, "/complete"), "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

// ---------- staff: read ----------

func TestStaffSeesSystemMessagesPatientDoesNot(t *testing.T) {
	e := newEnv(t)
	id, token := e.newIntake(t, "")
	e.store.AddMessage(context.Background(), id, RolePatient, "my head hurts", 100)
	e.store.AddMessage(context.Background(), id, RoleSystem, "internal note", 100)
	e.store.AddMessage(context.Background(), id, RoleAssistant, "how long?", 100)

	staff := jsonBody(t, e.staffDo("GET", "/intakes/"+id, ""))["messages"].([]any)
	if len(staff) != 3 {
		t.Errorf("staff sees %d messages, want 3", len(staff))
	}

	patient := jsonBody(t, e.do("GET", publicPath(token, ""), "", ""))["messages"].([]any)
	if len(patient) != 2 {
		t.Errorf("patient sees %d messages, want 2", len(patient))
	}
	for _, raw := range patient {
		if raw.(map[string]any)["role"] == "system" {
			t.Error("patient can see a system message")
		}
	}
}

func TestDetailNotFound(t *testing.T) {
	e := newEnv(t)
	for _, id := range []string{"not-a-uuid", "99999999-9999-4999-8999-999999999999"} {
		if rec := e.staffDo("GET", "/intakes/"+id, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%q: got %d, want 404", id, rec.Code)
		}
	}
}

func TestList(t *testing.T) {
	e := newEnv(t)
	_, tokenA := e.newIntake(t, "")
	e.newIntake(t, "")
	e.do("POST", publicPath(tokenA, "/messages"), `{"content":"hi"}`, "") // A becomes in_progress

	count := func(query string) int {
		rec := e.staffDo("GET", "/intakes"+query, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("list%s: got %d: %s", query, rec.Code, rec.Body)
		}
		return len(jsonBody(t, rec)["intakes"].([]any))
	}

	if n := count(""); n != 2 {
		t.Errorf("all = %d, want 2", n)
	}
	if n := count("?status=in_progress"); n != 1 {
		t.Errorf("in_progress = %d, want 1", n)
	}
	if n := count("?status=pending"); n != 1 {
		t.Errorf("pending = %d, want 1", n)
	}
	if n := count("?limit=1"); n != 1 {
		t.Errorf("limit=1 gave %d", n)
	}

	for _, q := range []string{"?status=bogus", "?limit=0", "?limit=201", "?limit=abc"} {
		if rec := e.staffDo("GET", "/intakes"+q, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", q, rec.Code)
		}
	}
}

func TestEmptyListIsAnArrayNotNull(t *testing.T) {
	e := newEnv(t)
	rec := e.staffDo("GET", "/intakes", "")
	if !strings.Contains(rec.Body.String(), `"intakes":[]`) {
		t.Fatalf("want an empty array, got %s", rec.Body)
	}
}
