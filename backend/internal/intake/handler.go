package intake

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpx"
)

const (
	maxMessageChars = 2000
	defaultMaxMsgs  = 100 // patient and assistant messages together, per intake
)

type Handler struct {
	store       Store
	frontendURL string
	ttl         time.Duration
	maxMessages int
	now         func() time.Time
}

// NewHandler makes the intake handler. frontendURL is where the patient chat
// page lives. ttl is how long a new link works.
func NewHandler(store Store, frontendURL string, ttl time.Duration) *Handler {
	return &Handler{
		store:       store,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		ttl:         ttl,
		maxMessages: defaultMaxMsgs,
		now:         time.Now,
	}
}

// Routes adds the intake routes to mux.
//
//	Staff (need login):  POST /intakes, GET /intakes, GET /intakes/{id}
//	Patient (no login):  /public/intakes/{token}...  The token in the link is the key.
func Routes(mux *http.ServeMux, h *Handler, requireLogin func(http.Handler) http.Handler) {
	mux.Handle("POST /intakes", requireLogin(http.HandlerFunc(h.Create)))
	mux.Handle("GET /intakes", requireLogin(http.HandlerFunc(h.List)))
	mux.Handle("GET /intakes/{id}", requireLogin(http.HandlerFunc(h.Detail)))

	mux.HandleFunc("GET /public/intakes/{token}", h.PublicGet)
	mux.HandleFunc("POST /public/intakes/{token}/messages", h.PublicSend)
	mux.HandleFunc("POST /public/intakes/{token}/complete", h.PublicComplete)
}

// ---------- JSON shapes ----------

type intakeView struct {
	ID           string     `json:"id"`
	PatientID    *string    `json:"patient_id"`
	PatientName  *string    `json:"patient_name"`
	Status       string     `json:"status"`
	ExpiresAt    time.Time  `json:"expires_at"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	MessageCount int        `json:"message_count"`
}

type messageView struct {
	ID        string    `json:"id,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) view(i Intake) intakeView {
	return intakeView{
		ID: i.ID, PatientID: i.PatientID, PatientName: i.PatientName,
		Status: effectiveStatus(i, h.now()), ExpiresAt: i.ExpiresAt,
		StartedAt: i.StartedAt, CompletedAt: i.CompletedAt,
		CreatedAt: i.CreatedAt, MessageCount: i.MessageCount,
	}
}

func messageViews(msgs []Message, includeSystem bool) []messageView {
	out := make([]messageView, 0, len(msgs)) // [] and not null in JSON
	for _, m := range msgs {
		if m.Role == RoleSystem && !includeSystem {
			continue
		}
		out = append(out, messageView{ID: m.ID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt})
	}
	return out
}

// ---------- staff routes ----------

type createRequest struct {
	Patient *struct {
		FullName    string `json:"full_name"`
		Phone       string `json:"phone"`
		DateOfBirth string `json:"date_of_birth"` // 2006-01-02
		Sex         string `json:"sex"`           // female, male, other
	} `json:"patient"`
}

// Create makes a new intake and its one-time link. The body is optional.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	in := NewIntake{CreatedBy: claims.Subject, ExpiresAt: h.now().Add(h.ttl)}

	if p := req.Patient; p != nil {
		patient, err := parsePatient(p.FullName, p.Phone, p.DateOfBirth, p.Sex, h.now())
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		in.Patient = patient
	}

	token, hash, err := NewToken()
	if err != nil {
		log.Printf("intake: new token: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	in.TokenHash = hash

	created, err := h.store.CreateIntake(r.Context(), in)
	if err != nil {
		log.Printf("intake: create: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"intake": h.view(created),
		"token":  token, // shown once. Only the hash is saved.
		"link":   fmt.Sprintf("%s/intake/%s", h.frontendURL, token),
	})
}

// List returns recent intakes, newest first. Optional ?status= and ?limit=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	switch status {
	case "", StatusPending, StatusInProgress, StatusCompleted:
	default:
		httpx.WriteError(w, http.StatusBadRequest, "status must be pending, in_progress or completed")
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			httpx.WriteError(w, http.StatusBadRequest, "limit must be a number from 1 to 200")
			return
		}
		limit = n
	}

	items, err := h.store.List(r.Context(), status, limit)
	if err != nil {
		log.Printf("intake: list: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	views := make([]intakeView, 0, len(items))
	for _, i := range items {
		views = append(views, h.view(i))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"intakes": views})
}

// Detail returns one intake with all its messages (staff also see system messages).
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		httpx.WriteError(w, http.StatusNotFound, "intake not found")
		return
	}

	in, err := h.store.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "intake not found")
		return
	}
	if err != nil {
		log.Printf("intake: get: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	msgs, err := h.store.Messages(r.Context(), in.ID)
	if err != nil {
		log.Printf("intake: messages: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"intake":   h.view(in),
		"messages": messageViews(msgs, true),
	})
}

// ---------- patient routes (no login) ----------

// loadByToken finds the intake for the link token in the URL.
// It writes the error response itself and returns false when it fails.
func (h *Handler) loadByToken(w http.ResponseWriter, r *http.Request) (Intake, bool) {
	token := r.PathValue("token")
	if len(token) != tokenLength {
		httpx.WriteError(w, http.StatusNotFound, "link not found")
		return Intake{}, false
	}

	in, err := h.store.GetByTokenHash(r.Context(), HashToken(token))
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "link not found")
		return Intake{}, false
	}
	if err != nil {
		log.Printf("intake: get by token: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return Intake{}, false
	}

	if effectiveStatus(in, h.now()) == StatusExpired {
		httpx.WriteError(w, http.StatusGone, "this link has expired, ask the clinic for a new one")
		return Intake{}, false
	}
	return in, true
}

// PublicGet shows the patient their conversation so far (so a page refresh keeps it).
func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) {
	in, ok := h.loadByToken(w, r)
	if !ok {
		return
	}
	msgs, err := h.store.Messages(r.Context(), in.ID)
	if err != nil {
		log.Printf("intake: messages: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     effectiveStatus(in, h.now()),
		"expires_at": in.ExpiresAt,
		"messages":   messageViews(msgs, false),
	})
}

type sendRequest struct {
	Content string `json:"content"`
}

// PublicSend saves a message from the patient. (The AI reply is added on Day 8.)
func (h *Handler) PublicSend(w http.ResponseWriter, r *http.Request) {
	in, ok := h.loadByToken(w, r)
	if !ok {
		return
	}
	if in.Status == StatusCompleted {
		httpx.WriteError(w, http.StatusConflict, "this intake is already finished")
		return
	}

	var req sendRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		httpx.WriteError(w, http.StatusBadRequest, "content is required")
		return
	}
	if utf8.RuneCountInString(content) > maxMessageChars {
		httpx.WriteError(w, http.StatusBadRequest,
			fmt.Sprintf("content must be at most %d characters", maxMessageChars))
		return
	}

	msg, err := h.store.AddMessage(r.Context(), in.ID, RolePatient, content, h.maxMessages)
	if errors.Is(err, ErrTooManyMessages) {
		httpx.WriteError(w, http.StatusTooManyRequests, "message limit reached for this intake")
		return
	}
	if err != nil {
		log.Printf("intake: add message: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	if in.Status == StatusPending {
		if err := h.store.Start(r.Context(), in.ID); err != nil {
			log.Printf("intake: start: %v", err) // the message is saved, so carry on
		}
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]messageView{
		"message": {ID: msg.ID, Role: msg.Role, Content: msg.Content, CreatedAt: msg.CreatedAt},
	})
}

// PublicComplete is the patient saying "I'm done". Safe to call twice.
func (h *Handler) PublicComplete(w http.ResponseWriter, r *http.Request) {
	in, ok := h.loadByToken(w, r)
	if !ok {
		return
	}
	if in.Status == StatusCompleted {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": StatusCompleted})
		return
	}
	if in.MessageCount == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "send at least one message first")
		return
	}

	if err := h.store.Complete(r.Context(), in.ID); err != nil {
		log.Printf("intake: complete: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": StatusCompleted})
}

// ---------- helpers ----------

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidPattern.MatchString(s) }

func parsePatient(name, phone, dob, sex string, now time.Time) (*NewPatient, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, errors.New("patient.full_name is required (up to 100 characters)")
	}
	p := &NewPatient{FullName: name}

	if phone = strings.TrimSpace(phone); phone != "" {
		if utf8.RuneCountInString(phone) > 30 {
			return nil, errors.New("patient.phone is too long")
		}
		p.Phone = &phone
	}

	if dob = strings.TrimSpace(dob); dob != "" {
		t, err := time.Parse("2006-01-02", dob)
		if err != nil {
			return nil, errors.New("patient.date_of_birth must look like 2006-01-02")
		}
		if t.After(now) || t.Before(now.AddDate(-130, 0, 0)) {
			return nil, errors.New("patient.date_of_birth is not realistic")
		}
		p.DateOfBirth = &t
	}

	if sex = strings.ToLower(strings.TrimSpace(sex)); sex != "" {
		if sex != "female" && sex != "male" && sex != "other" {
			return nil, errors.New("patient.sex must be female, male or other")
		}
		p.Sex = &sex
	}
	return p, nil
}
