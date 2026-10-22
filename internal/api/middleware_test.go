package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestLimitInflight(t *testing.T) {
	release := make(chan struct{})    // close() để thả hai handler đang treo
	entered := make(chan struct{}, 2) // handler báo "tôi đã vào"
	codes := make(chan int, 2)        // hai request treo gửi status về đây

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	})
	h := limitInflight(2, slow)

	do := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/draws", nil))
		return rec.Code
	}

	// Hồi 1: làm đầy
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- do() }()
	}
	<-entered
	<-entered

	// Hồi 2: đầy thì từ chối ngay
	start := time.Now()
	if code := do(); code != http.StatusServiceUnavailable {
		t.Errorf("status = %d; want 503", code)
	}
	if d := time.Since(start); d >= 50*time.Millisecond {
		t.Errorf("503 mất %v; want < 50ms", d)
	}

	// Hồi 3: thả, hai request treo phải xong đúng, slot phải được trả
	close(release)
	wg.Wait()
	for range 2 {
		if c := <-codes; c != http.StatusOK {
			t.Errorf("request treo trả %d; want 200", c)
		}
	}
	if c := do(); c != http.StatusOK {
		t.Errorf("sau khi thả: status = %d; want 200", c)
	}
}

func TestMetric(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	body := strings.NewReader(`{"user_id":1,"campaign":"tet2026"}`)
	h := NewHandler(&brokenStore{}, logger)
	c := reqTotal.WithLabelValues("POST /draws", "500")
	before := testutil.ToFloat64(c)
	req := httptest.NewRequest("POST", "/draws", body)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)
	if got := testutil.ToFloat64(c) - before; got != 1 {
		t.Errorf("http_requests_total{POST /draws,500} tăng %v; want 1", got)
	}

}

func TestMetric_Shed(t *testing.T) {
	before := testutil.ToFloat64(shedTotal)
	h := limitInflight(0, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next không được gọi khi limiter từ chối")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/draws", nil))
	if rec.Code != 503 {
		t.Errorf("status = %d; want 503", rec.Code)
	}
	if got := testutil.ToFloat64(shedTotal) - before; got != 1 {
		t.Errorf("http_requests_shed_total tăng %v; want 1", got)
	}
}
