package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"luckydraw/internal/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type brokenStore struct{}

func (b *brokenStore) DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (domain.DrawResult, error) {
	return domain.DrawResult{}, errors.New("dial tcp 10.0.3.14:5432: connection refused")
}

// okStore luôn trả về một giải trúng. replayed quyết định đây là lượt quay
// mới (201) hay bản sao của một lượt đã xử lý (200).
type okStore struct {
	prize    *domain.Prize
	replayed bool
}

func (s *okStore) DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (domain.DrawResult, error) {
	return domain.DrawResult{Prize: s.prize, Replayed: s.replayed}, nil
}

// emptyStore mô phỏng tình huống hết quà.
type emptyStore struct{}

func (s *emptyStore) DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (domain.DrawResult, error) {
	return domain.DrawResult{}, domain.ErrNoPrizeAvailable
}

func TestDraw_SystemError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	body := strings.NewReader(`{"user_id":1,"campaign":"tet2026"}`)
	h := NewHandler(&brokenStore{}, logger)

	req := httptest.NewRequest("POST", "/draws", body)
	rec := httptest.NewRecorder()

	h.Draw(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	if body := rec.Body.String(); strings.Contains(body, "10.0.3.14") {
		t.Errorf("body lộ thông tin nội bộ: %s", body)
	}

	var got errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Error == "" {
		t.Errorf("error rỗng; want message chung cho client")
	}
}

func TestDraw_Success(t *testing.T) {
	body := strings.NewReader(`{"user_id":1,"campaign":"tet2026"}`)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := httptest.NewRequest("POST", "/draws", body)
	rec := httptest.NewRecorder()

	st := &okStore{prize: mustPrize(t, 1, "Item 1", 40, 70)}
	h := NewHandler(st, logger)

	h.Draw(rec, req)

	// 1 status code
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusCreated)
	}

	//2 body - decode ra kiểu thật thay vì chuỗi
	var got drawResponse

	if err := json.NewDecoder(rec.Body).Decode((&got)); err != nil {
		t.Fatalf("decode body: %v; body = %q", err, rec.Body.String())
	}

	if got.PrizeID == 0 {
		t.Errorf("prize_id = 0; want khác 0")
	}
	if got.Name == "" {
		t.Errorf("name rỗng; want tên giải")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

func TestDraw_OutOfStock(t *testing.T) {
	body := strings.NewReader(`{"user_id":1,"campaign":"tet2026"}`)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := httptest.NewRequest("POST", "/draws", body)
	rec := httptest.NewRecorder()
	st := &emptyStore{}

	h := NewHandler(st, logger)

	h.Draw(rec, req)

	var got errorResponse

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusConflict)
	}

	if err := json.NewDecoder(rec.Body).Decode((&got)); err != nil {
		t.Fatalf("decode body: %v; body = %q", err, rec.Body.String())

	}

	if got.Error == "" {
		t.Errorf("error rỗng; want thông báo hết quà")

	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

func TestHealthz(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	h := NewHandler(nil, logger)

	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if body := rec.Body.String(); body != "ok" {
		t.Errorf("body = %q; want %q", body, "ok")
	}

}

func mustPrize(t *testing.T, id int, name string, qty, weight int) *domain.Prize {
	p, err := domain.NewPrize(id, name, qty, weight)
	if err != nil {
		log.Fatalf("NewPrize(%d, %q): %v", id, name, err)
	}
	return p
}
