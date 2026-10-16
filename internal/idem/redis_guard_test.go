package idem

import (
	"context"
	"log/slog"
	"luckydraw/internal/domain"
	"os"
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
