package api

import (
	"context"
	"errors"
	"log/slog"
	"luckydraw/internal/domain"
	"net/http"
)

type Handler struct {
	store  PrizeDrawer
	logger *slog.Logger
}

type PrizeDrawer interface {
	DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (domain.DrawResult, error)
}

func NewHandler(st PrizeDrawer, lg *slog.Logger) *Handler {
	return &Handler{store: st, logger: lg}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /draws", h.Draw)
	mux.HandleFunc("GET /healthz", h.Healthz)

	return h.logging(h.recoverMiddleware(mux))
}

func (h *Handler) Draw(w http.ResponseWriter, r *http.Request) {

	var req drawRequest
	key := r.Header.Get("Idempotency-Key")
	if err := readJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	res, err := h.store.DrawAndTake(r.Context(), int64(req.UserID), req.Campaign, key)
	if err != nil {

		if errors.Is(err, domain.ErrNoPrizeAvailable) {
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, domain.ErrQuotaExceeded) {
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, domain.ErrKeyConflict) {
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		// Lỗi hệ thống: log đầy đủ ở server, giấu chi tiết với client.
		h.logger.Error("Draw",
			"method", r.Method,
			"path", r.URL.Path,
			"err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "lỗi hệ thống"})
		return
	}
	status := http.StatusCreated
	if res.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, drawResponse{
		PrizeID: res.Prize.ID,
		Name:    res.Prize.Name,
	})

}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
