package idem

import (
	"context"
	"errors"
	"log/slog"
	"luckydraw/internal/domain"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeNext struct {
	calls atomic.Int64      // đếm
	res   domain.DrawResult // trả gì khi được gọi
	err   error
}

var testRedis *redis.Client

func (f *fakeNext) DrawAndTake(ctx context.Context, userID int64, programID, idemKey string) (domain.DrawResult, error) {
	f.calls.Add(1)
	return f.res, f.err
}

func TestGuard_CungKeyGoiNextMotLan(t *testing.T) {
	rdb := requireRedis(t)
	fake := &fakeNext{res: domain.DrawResult{Prize: &domain.Prize{ID: 3, Name: "Sticker"}}}
	g := New(fake, rdb, slog.Default())
	ctx := context.Background()
	const userID int64 = 1
	key := t.Name()
	t.Cleanup(func() { rdb.Del(context.Background(), "idem:"+key) })

	lan1, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	if lan1.Prize == nil {
		t.Fatal("lần 1: Prize = nil; want một giải")
	}

	if lan1.Replayed {
		t.Error("lần 1: Replayed = true; want false")
	}

	lan2, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
	if err != nil {
		t.Fatalf("lần 2: %v", err)
	}

	if !lan2.Replayed {
		t.Error("lần 2: Replayed = false; want true")
	}
	if lan2.Prize.ID != lan1.Prize.ID {
		t.Errorf("lần 2 trả prize %d; want %d (phải giống lần 1)", lan2.Prize.ID, lan1.Prize.ID)
	}
	if fake.calls.Load() != 1 {
		t.Errorf("next bị gọi %d lần; want 1", fake.calls.Load())

	}
}

func TestMain(m *testing.M) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rdb.Ping(ctx).Err(); err == nil {
		testRedis = rdb
	}
	cancel()

	code := m.Run()
	if testRedis != nil {
		testRedis.Close()
	}
	os.Exit(code)
}

func requireRedis(t *testing.T) *redis.Client {
	t.Helper()
	if testRedis == nil {
		if os.Getenv("CI") != "" {
			t.Fatal("running on CI but Redis is unreachable")
		}
		t.Skip("không kết nối được Redis — bỏ qua integration test")
	}
	return testRedis
}

func TestGuard_RedisChet_FailOpen(t *testing.T) {
	redisAddr := "localhost:1"
	down := redis.NewClient(&redis.Options{Addr: redisAddr})
	fake := &fakeNext{res: domain.DrawResult{Prize: &domain.Prize{ID: 3, Name: "Sticker"}}}
	g := New(fake, down, slog.Default())
	start := time.Now()
	ctx := context.Background()
	const userID int64 = 1
	key := t.Name()

	lan1, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	if lan1.Prize == nil {
		t.Fatal("lần 1: Prize = nil; want một giải")
	}
	if lan1.Replayed {
		t.Error("lần 1: Replayed = true; want false")
	}

	lan2, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
	if err != nil {
		t.Fatalf("lần 2: %v", err)
	}
	if lan2.Replayed {
		t.Error("lần 2: Replayed = true; want false")
	}

	if lan2.Prize.ID != lan1.Prize.ID {
		t.Errorf("lần 2 trả prize %d; want %d (phải giống lần 1)", lan2.Prize.ID, lan1.Prize.ID)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("fail-open mất %v; want < 500ms (Redis chết không được làm chậm request)", d)
	}
	if fake.calls.Load() != 2 {
		t.Errorf("next bị gọi %d lần; want 2", fake.calls.Load())
	}
}

func TestGuard_NextLoi_XoaKey(t *testing.T) {
	rdb := requireRedis(t)
	fake := &fakeNext{err: domain.ErrNoPrizeAvailable}
	g := New(fake, rdb, slog.Default())
	ctx := context.Background()
	const userID int64 = 1
	key := t.Name()
	t.Cleanup(func() { rdb.Del(context.Background(), "idem:"+key) })
	lan1, err := g.DrawAndTake(ctx, userID, "tet-2026", key)

	if !errors.Is(err, domain.ErrNoPrizeAvailable) {
		t.Fatalf("lần 1: err = %v; want ErrNoPrizeAvailable", err)
	}

	if lan1.Replayed {
		t.Error("lần 1: Replayed = true; want false")
	}

	lan2, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
	if !errors.Is(err, domain.ErrNoPrizeAvailable) {
		t.Fatalf("lần 1: err = %v; want ErrNoPrizeAvailable", err)
	}

	if lan2.Replayed {
		t.Error("lần 2: Replayed = true; want false")
	}
	if fake.calls.Load() != 2 {
		t.Errorf("next bị gọi %d lần; want 2", fake.calls.Load())
	}
	if rdb.Exists(ctx, "idem:"+key).Val() != 0 {
		t.Error("Key exist; want key del")
	}
}

func TestGuard_KeyCuaNguoiKhac(t *testing.T) {
	rdb := requireRedis(t)
	fake := &fakeNext{res: domain.DrawResult{Prize: &domain.Prize{ID: 3, Name: "Sticker"}}}
	g := New(fake, rdb, slog.Default())
	ctx := context.Background()
	const userID int64 = 1
	const userID2 int64 = 3
	key := t.Name()
	t.Cleanup(func() { rdb.Del(context.Background(), "idem:"+key) })
	lan1, err := g.DrawAndTake(ctx, userID, "tet-2026", key)

	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	if lan1.Prize == nil {
		t.Fatal("lần 1: Prize = nil; want một giải")
	}

	if lan1.Replayed {
		t.Error("lần 1: Replayed = true; want false")
	}

	lan2, err := g.DrawAndTake(ctx, userID2, "tet-2026", key)
	if !errors.Is(err, domain.ErrKeyConflict) {
		t.Fatalf("lần 2: err = %v; want ErrKeyConflict", err)
	}

	if lan2.Replayed {
		t.Error("lần 2: Replayed = true ; want false")
	}
	if fake.calls.Load() != 1 {
		t.Errorf("next bị gọi %d lần; want 1", fake.calls.Load())
	}
}

func TestGuard_NhieuGoroutineCungKey(t *testing.T) {
	rdb := requireRedis(t)
	fake := &fakeNext{res: domain.DrawResult{Prize: &domain.Prize{ID: 3, Name: "Sticker"}}}
	g := New(fake, rdb, slog.Default())
	ctx := context.Background()
	const userID int64 = 1
	key := t.Name()
	t.Cleanup(func() { rdb.Del(context.Background(), "idem:"+key) })
	const rounds = 10
	var wg sync.WaitGroup
	var moi atomic.Int64
	var replay atomic.Int64
	var dangXuLy atomic.Int64
	start := make(chan struct{})
	for range rounds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := g.DrawAndTake(ctx, userID, "tet-2026", key)
			switch {
			case err == nil && !res.Replayed:
				moi.Add(1)
			case err == nil && res.Replayed:
				replay.Add(1)
			case errors.Is(err, domain.ErrInProgress):
				dangXuLy.Add(1)
			default:
				t.Errorf("lỗi không mong đợi: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if n := moi.Load(); n != 1 {
		t.Errorf("phát ra %d giải; want 1", n)
	}

	if r, p := replay.Load(), dangXuLy.Load(); r+p != rounds-1 {
		t.Errorf("replay + in_progress = %d + %d = %d; want %d", r, p, r+p, rounds-1)
	}

	if fake.calls.Load() != 1 {
		t.Errorf("replay = %d; want 1", fake.calls.Load())
	}
}
