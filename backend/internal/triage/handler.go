package triage

import (
	"errors"
	"log"
	"net/http"
	"regexp"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpx"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
)

type Handler struct {
	svc    *Service
	intake intake.Store
}

func NewHandler(svc *Service, intakes intake.Store) *Handler {
	return &Handler{svc: svc, intake: intakes}
}

// Routes adds the staff routes (all need login):
//
//	GET  /intakes/{id}/triage      the saved result (404 if none yet)
//	POST /intakes/{id}/triage/run  run it again (only for completed intakes)
func Routes(mux *http.ServeMux, h *Handler, requireLogin func(http.Handler) http.Handler) {
	mux.Handle("GET /intakes/{id}/triage", requireLogin(http.HandlerFunc(h.Get)))
	mux.Handle("POST /intakes/{id}/triage/run", requireLogin(http.HandlerFunc(h.Run)))
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusNotFound, "triage result not found")
		return
	}
	res, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "triage result not found")
		return
	}
	if err != nil {
		log.Printf("triage: get: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]Result{"triage": res})
}

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusNotFound, "intake not found")
		return
	}
	in, err := h.intake.GetByID(r.Context(), id)
	if errors.Is(err, intake.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "intake not found")
		return
	}
	if err != nil {
		log.Printf("triage: load intake: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if in.Status != intake.StatusCompleted {
		httpx.WriteError(w, http.StatusConflict, "the patient has not finished this intake yet")
		return
	}
	msgs, err := h.intake.Messages(r.Context(), id)
	if err != nil {
		log.Printf("triage: messages: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	res, err := h.svc.Run(r.Context(), id, msgs)
	if err != nil {
		log.Printf("triage: run: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]Result{"triage": res})
}
