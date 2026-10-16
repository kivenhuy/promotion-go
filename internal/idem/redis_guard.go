package idem

import (
	"context"
	"encoding/json"
	"log/slog"
	"luckydraw/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisTimeout = 50 * time.Millisecond

// Drawer
type Drawer interface {
	DrawAndTake(ctx context.Context, userID int64, programID, idemKey string) (domain.DrawResult, error)
}

type cached struct {
	UserID    int64  `json:"user_id"`
	PackageID int    `json:"package_id"`
	Name      string `json:"name"`
}

type Guard struct {
	next Drawer
	rdb  *redis.Client
	log  *slog.Logger
}

func New(next Drawer, rdb *redis.Client, lg *slog.Logger) *Guard {
	return &Guard{next: next, rdb: rdb, log: lg}
}

func (g *Guard) setNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	rc, cancel := context.WithTimeout(ctx,redisTimeout)
	defer cancel()
	return g.rdb.SetNX(rc, key, val, ttl).Result()
}

func (g *Guard) get(ctx context.Context, key string) (string, error) {
	rc, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return g.rdb.Get(rc, key).Result()
}

func (g *Guard) set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	rc, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	return g.rdb.Set(rc, key, val, ttl).Err()
}

func (g *Guard) del(ctx context.Context, key string) error {
	rc, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return g.rdb.Del(rc, key).Err()
}

func (g *Guard) DrawAndTake(ctx context.Context, userID int64, programID, idemKey string) (domain.DrawResult, error) {
	key := "idem:" + idemKey
	first, err := g.setNX(ctx, key, "processing", 10*time.Second)
	if err != nil {
		g.log.Warn("redis setnx", "err", err)
		return g.next.DrawAndTake(ctx, userID, programID, idemKey) // Redis hỏng → như không có Redis
	}

	if !first {
		value, err := g.get(ctx, key)
		if err != nil { // gồm cả redis.Nil: key vừa hết hạn
			return g.next.DrawAndTake(ctx, userID, programID, idemKey)
		}
		if value == "processing" {
			return domain.DrawResult{}, domain.ErrInProgress
		}
		var c cached
		if err := json.Unmarshal([]byte(value), &c); err != nil {
			return g.next.DrawAndTake(ctx, userID, programID, idemKey) // cache hỏng → DB quyết
		}
		if c.UserID != userID {
			return domain.DrawResult{}, domain.ErrKeyConflict
		}
		return domain.DrawResult{Prize: &domain.Prize{ID: c.PackageID, Name: c.Name}, Replayed: true}, nil
	}

	res, err := g.next.DrawAndTake(ctx, userID, programID, idemKey)
	if err != nil {
		g.del(ctx, key) // lỗi Del bỏ qua, TTL 10s tự dọn
		return domain.DrawResult{}, err
	}
	b, _ := json.Marshal(cached{UserID: userID, PackageID: res.Prize.ID, Name: res.Prize.Name})
	if err := g.set(ctx, key, b, 48*time.Hour); err != nil {
		g.log.Warn("redis set result", "err", err) // DB đã commit, KHÔNG được trả lỗi
	}
	return res, nil
}
