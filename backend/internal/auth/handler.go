package auth

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpx"
)

type Handler struct {
	store  StaffStore
	hasher PasswordHasher
	tokens *TokenManager

	// A real hash we compare against when the email is unknown, so a login for
	// a missing account takes about as long as one for a real account.
	dummyHash string
}

func NewHandler(store StaffStore, hasher PasswordHasher, tokens *TokenManager) (*Handler, error) {
	dummy, err := hasher.Hash("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Handler{store: store, hasher: hasher, tokens: tokens, dummyHash: dummy}, nil
}

type staffView struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type authResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Staff     staffView `json:"staff"`
}

func view(s Staff) staffView {
	return staffView{ID: s.ID, Email: s.Email, FullName: s.FullName, Role: s.Role}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// Register creates a staff account. The role is always "nurse" here. A request
// cannot pick its own role. Promote someone to doctor or admin in the database.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	email, err := cleanEmail(req.Email)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Password) < 8 {
		httpx.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 72 { // bcrypt only reads the first 72 bytes
		httpx.WriteError(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}
	name := strings.TrimSpace(req.FullName)
	if name == "" || len(name) > 100 {
		httpx.WriteError(w, http.StatusBadRequest, "full_name is required (up to 100 characters)")
		return
	}

	hash, err := h.hasher.Hash(req.Password)
	if err != nil {
		log.Printf("auth: hash password: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	staff, err := h.store.Create(r.Context(), NewStaff{Email: email, PasswordHash: hash, FullName: name})
	if errors.Is(err, ErrEmailTaken) {
		httpx.WriteError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		log.Printf("auth: create staff: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.respondWithToken(w, http.StatusCreated, staff)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Same message for "no such email" and "wrong password", on purpose.
	const bad = "invalid email or password"

	staff, err := h.store.GetByEmail(r.Context(), strings.TrimSpace(req.Email))
	if errors.Is(err, ErrNotFound) {
		h.hasher.Compare(h.dummyHash, req.Password)
		httpx.WriteError(w, http.StatusUnauthorized, bad)
		return
	}
	if err != nil {
		log.Printf("auth: get staff: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if !h.hasher.Compare(staff.PasswordHash, req.Password) {
		httpx.WriteError(w, http.StatusUnauthorized, bad)
		return
	}

	h.respondWithToken(w, http.StatusOK, staff)
}

// Me returns the logged-in staff member. Mount it behind Middleware.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r.Context())
	if !ok {
		unauthorized(w, "missing token")
		return
	}
	staff, err := h.store.GetByID(r.Context(), claims.Subject)
	if errors.Is(err, ErrNotFound) {
		unauthorized(w, "account no longer exists")
		return
	}
	if err != nil {
		log.Printf("auth: get staff by id: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]staffView{"staff": view(staff)})
}

func (h *Handler) respondWithToken(w http.ResponseWriter, code int, staff Staff) {
	token, exp, err := h.tokens.Issue(staff.ID, staff.Role)
	if err != nil {
		log.Printf("auth: issue token: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, code, authResponse{Token: token, ExpiresAt: exp, Staff: view(staff)})
}

func cleanEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	// ParseAddress also accepts "Name <a@b.com>". We only want the bare address.
	if err != nil || addr.Address != email || len(email) > 254 {
		return "", errors.New("a valid email is required")
	}
	return email, nil
}
