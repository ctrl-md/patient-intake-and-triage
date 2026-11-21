package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- fakes ----

type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }
func (fakeHasher) Compare(hash, p string) bool   { return hash == "hashed:"+p }

type fakeStore struct {
	byID map[string]Staff
	n    int
}

func newFakeStore() *fakeStore { return &fakeStore{byID: map[string]Staff{}} }

func (f *fakeStore) Create(_ context.Context, in NewStaff) (Staff, error) {
	for _, s := range f.byID {
		if strings.EqualFold(s.Email, in.Email) {
			return Staff{}, ErrEmailTaken
		}
	}
	f.n++
	s := Staff{
		ID: "id-" + string(rune('0'+f.n)), Email: in.Email, PasswordHash: in.PasswordHash,
		FullName: in.FullName, Role: "nurse", CreatedAt: time.Now(),
	}
	f.byID[s.ID] = s
	return s, nil
}

func (f *fakeStore) GetByEmail(_ context.Context, email string) (Staff, error) {
	for _, s := range f.byID {
		if strings.EqualFold(s.Email, email) {
			return s, nil
		}
	}
	return Staff{}, ErrNotFound
}

func (f *fakeStore) GetByID(_ context.Context, id string) (Staff, error) {
	if s, ok := f.byID[id]; ok {
		return s, nil
	}
	return Staff{}, ErrNotFound
}

// ---- helpers ----

// newTestServer builds the same three routes the real router has.
func newTestServer(t *testing.T) (http.Handler, *fakeStore, *TokenManager) {
	t.Helper()
	store := newFakeStore()
	tm := NewTokenManager(testSecret, time.Hour)
	h, err := NewHandler(store, fakeHasher{}, tm)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.Handle("GET /auth/me", Middleware(tm)(http.HandlerFunc(h.Me)))
	return mux, store, tm
}

func do(t *testing.T, h http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad json %q: %v", rec.Body.String(), err)
	}
	return m
}

const goodRegister = `{"email":"Ada@Clinic.com","password":"supersecret1","full_name":"Ada Obi"}`

// ---- register ----

func TestRegister(t *testing.T) {
	h, _, tm := newTestServer(t)

	rec := do(t, h, "POST", "/auth/register", goodRegister, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)

	staff := body["staff"].(map[string]any)
	if staff["email"] != "ada@clinic.com" {
		t.Errorf("email should be lower case, got %v", staff["email"])
	}
	if staff["role"] != "nurse" {
		t.Errorf("role = %v, want nurse", staff["role"])
	}
	if strings.Contains(rec.Body.String(), "hashed:") || strings.Contains(rec.Body.String(), "password") {
		t.Errorf("response leaks password data: %s", rec.Body)
	}
	if _, err := tm.Parse(body["token"].(string)); err != nil {
		t.Errorf("token from register does not parse: %v", err)
	}
}

func TestRegisterCannotChooseRole(t *testing.T) {
	h, _, _ := newTestServer(t)
	rec := do(t, h, "POST", "/auth/register",
		`{"email":"a@b.com","password":"supersecret1","full_name":"A","role":"admin"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d", rec.Code)
	}
	if role := decode(t, rec)["staff"].(map[string]any)["role"]; role != "nurse" {
		t.Fatalf("role = %v, want nurse", role)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	h, _, _ := newTestServer(t)
	do(t, h, "POST", "/auth/register", goodRegister, "")

	rec := do(t, h, "POST", "/auth/register",
		`{"email":"ADA@clinic.com","password":"anotherpass1","full_name":"Other"}`, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409", rec.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	h, _, _ := newTestServer(t)
	long := strings.Repeat("a", 73)

	tests := map[string]string{
		"not json":       `nope`,
		"no email":       `{"password":"supersecret1","full_name":"A"}`,
		"bad email":      `{"email":"not-an-email","password":"supersecret1","full_name":"A"}`,
		"name in email":  `{"email":"Ada <a@b.com>","password":"supersecret1","full_name":"A"}`,
		"short password": `{"email":"a@b.com","password":"short","full_name":"A"}`,
		"long password":  `{"email":"a@b.com","password":"` + long + `","full_name":"A"}`,
		"no name":        `{"email":"a@b.com","password":"supersecret1","full_name":"  "}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			rec := do(t, h, "POST", "/auth/register", body, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", rec.Code, rec.Body)
			}
		})
	}
}

// ---- login ----

func TestLogin(t *testing.T) {
	h, _, tm := newTestServer(t)
	do(t, h, "POST", "/auth/register", goodRegister, "")

	rec := do(t, h, "POST", "/auth/login", `{"email":"ada@clinic.com","password":"supersecret1"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if _, err := tm.Parse(body["token"].(string)); err != nil {
		t.Fatalf("token does not parse: %v", err)
	}

	// Email in different capitals still works.
	rec = do(t, h, "POST", "/auth/login", `{"email":"ADA@CLINIC.COM","password":"supersecret1"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("upper case email: got %d", rec.Code)
	}
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	h, _, _ := newTestServer(t)
	do(t, h, "POST", "/auth/register", goodRegister, "")

	wrongPass := do(t, h, "POST", "/auth/login", `{"email":"ada@clinic.com","password":"wrongpass123"}`, "")
	noUser := do(t, h, "POST", "/auth/login", `{"email":"nobody@clinic.com","password":"supersecret1"}`, "")

	if wrongPass.Code != http.StatusUnauthorized || noUser.Code != http.StatusUnauthorized {
		t.Fatalf("got %d and %d, want 401 and 401", wrongPass.Code, noUser.Code)
	}
	if wrongPass.Body.String() != noUser.Body.String() {
		t.Fatalf("messages differ, which tells an attacker which emails exist:\n%s\n%s",
			wrongPass.Body, noUser.Body)
	}
}

// ---- me + middleware ----

func TestMe(t *testing.T) {
	h, _, tm := newTestServer(t)
	reg := decode(t, do(t, h, "POST", "/auth/register", goodRegister, ""))
	token := reg["token"].(string)

	rec := do(t, h, "GET", "/auth/me", "", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	if email := decode(t, rec)["staff"].(map[string]any)["email"]; email != "ada@clinic.com" {
		t.Fatalf("email = %v", email)
	}

	// No token, junk token, expired token.
	if rec := do(t, h, "GET", "/auth/me", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/auth/me", "", "junk"); rec.Code != http.StatusUnauthorized {
		t.Errorf("junk token: got %d", rec.Code)
	}
	tm.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if rec := do(t, h, "GET", "/auth/me", "", token); rec.Code != http.StatusUnauthorized {
		t.Errorf("expired token: got %d", rec.Code)
	}
}

func TestMeWhenAccountWasDeleted(t *testing.T) {
	h, store, _ := newTestServer(t)
	reg := decode(t, do(t, h, "POST", "/auth/register", goodRegister, ""))
	token := reg["token"].(string)

	store.byID = map[string]Staff{} // account removed after the token was made

	if rec := do(t, h, "GET", "/auth/me", "", token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestRequireRole(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Hour)
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := Middleware(tm)(RequireRole("doctor", "admin")(ok))

	nurse, _, _ := tm.Issue("s1", "nurse")
	doctor, _, _ := tm.Issue("s2", "doctor")

	tests := []struct {
		name, token string
		want        int
	}{
		{"doctor allowed", doctor, http.StatusOK},
		{"nurse blocked", nurse, http.StatusForbidden},
		{"no token", "", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, "GET", "/x", "", tc.token)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
