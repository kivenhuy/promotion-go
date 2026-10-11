package store

import (
	"errors"
	"luckydraw/internal/domain"
	"runtime"
	"time"

	"context"
	"sync"
	"testing"
)

func mustPrize(t *testing.T, id int, name string, qty, weight int) *domain.Prize {
	t.Helper()
	p, err := domain.NewPrize(id, name, qty, weight)
	if err != nil {
		t.Fatalf("NewPrize(%d, %q, %d, %d) lỗi: %v", id, name, qty, weight, err)
	}
	return p
}

func TestDrawAndTake_Concurrent(t *testing.T) {
	const (
		rounds = 1000
	)

	prizes := []*domain.Prize{
		mustPrize(t, 1, "iPhone", 40, 70),
		mustPrize(t, 2, "Voucher", 40, 20),
		mustPrize(t, 3, "Sticker", 40, 10),
	}

	st := NewPrizeStore(prizes)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0

	for range rounds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.DrawAndTake(context.Background(), 1, "", ""); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()

			}
		}()

	}

	wg.Wait()
	if ok != 120 {
		t.Errorf("phát ra %d giải; want 120", ok)
	}

	totalQuantity := 0

	for _, p := range prizes {
		totalQuantity += p.Quantity
	}

	if totalQuantity != 0 {
		t.Errorf("kho còn lại %d; want 0", totalQuantity)
	}

	t.Logf("Tổng số là: %d ", totalQuantity)

}

func TestDrawMany(t *testing.T) {
	const (
		rounds = 1000
	)

	prizes := []*domain.Prize{
		mustPrize(t, 1, "iPhone", 40, 70),
		mustPrize(t, 2, "Voucher", 40, 20),
		mustPrize(t, 3, "Sticker", 40, 10),
	}

	st := NewPrizeStore(prizes)

	won := DrawMany(st, "", rounds)

	if len(won) != 120 {
		t.Errorf("phát ra %d giải; want 120", len(won))
	}

	totalQuantity := 0

	for _, p := range prizes {
		totalQuantity += p.Quantity
	}

	if totalQuantity != 0 {
		t.Errorf("kho còn lại %d; want 0", totalQuantity)
	}

	t.Logf("Tổng số là: %d ", totalQuantity)
}

// func TestDrawManyContext_Cancelled(t *testing.T) {
// 	checkLeak(t)
// 	const (
// 		rounds = 1000
// 	)

// 	prizes := []*domain.Prize{
// 		mustPrize(t, 1, "iPhone", 40, 70),
// 		mustPrize(t, 2, "Voucher", 40, 20),
// 		mustPrize(t, 3, "Sticker", 40, 10),
// 	}

// 	ctx, cancel := context.WithCancel(context.Background())
// 	cancel()

// 	st := NewPrizeStore(prizes)

// 	won, err := DrawManyContext(ctx, st, rounds)

// 	if len(won) != 0 {
// 		t.Errorf("phát ra %d giải;", len(won))
// 	}

// 	totalQuantity := 0

// 	for _, p := range prizes {
// 		totalQuantity += p.Quantity
// 	}

// 	if totalQuantity != 120 {
// 		t.Errorf("kho còn lại %d; want 0", totalQuantity)
// 	}

// 	if errors.Is(err, context.Canceled) != true {
// 		t.Errorf("bug")
// 	}
// 	t.Logf("Tổng số là: %d ", totalQuantity)
// }

func TestDrawManyPool(t *testing.T) {
	checkLeak(t)

	prizes := []*domain.Prize{
		mustPrize(t, 1, "iPhone", 40, 70),
		mustPrize(t, 2, "Voucher", 40, 20),
		mustPrize(t, 3, "Sticker", 40, 10),
	}

	st := NewPrizeStore(prizes)

	won, err := DrawManyPool(context.Background(), st, "", 1000, 10)

	if err != nil {
		t.Errorf("bug")
	}

	if len(won) != 120 {
		t.Errorf("phát ra %d giải;", len(won))
	}

	totalQuantity := 0

	for _, p := range prizes {
		totalQuantity += p.Quantity
	}

	if totalQuantity != 0 {
		t.Errorf("kho còn lại %d; want 0", totalQuantity)
	}
}

func TestDrawManyPoolContext_Cancelled(t *testing.T) {
	checkLeak(t)
	const (
		rounds = 1000
	)

	prizes := []*domain.Prize{
		mustPrize(t, 1, "iPhone", 40, 70),
		mustPrize(t, 2, "Voucher", 40, 20),
		mustPrize(t, 3, "Sticker", 40, 10),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	st := NewPrizeStore(prizes)

	won, err := DrawManyPool(ctx, st, "", rounds, 10)

	if len(won) != 0 {
		t.Errorf("phát ra %d giải;", len(won))
	}

	totalQuantity := 0

	for _, p := range prizes {
		totalQuantity += p.Quantity
	}

	if totalQuantity != 120 {
		t.Errorf("kho còn lại %d; want 0", totalQuantity)
	}

	if errors.Is(err, context.Canceled) != true {
		t.Errorf("bug")
	}
	t.Logf("Tổng số là: %d ", totalQuantity)
}

func checkLeak(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()

	t.Cleanup(func() {
		// chờ một nhịp cho goroutine kịp thoát
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		if after > before {
			t.Errorf("leak %d goroutine (trước %d, sau %d)", after-before, before, after)
		}
	})
}
