package store

import (
	"context"
	"luckydraw/internal/domain"
	"sync"
)

type PrizeStore struct {
	mu     sync.Mutex // bảo vệ prizes
	prizes []*domain.Prize
}

// NewPrizeStore tạo store giữ tham chiếu tới danh sách prize truyền vào.
// Store không sao chép danh sách, nên mọi thay đổi tồn kho đều tác động
// lên chính các Prize gốc mà caller đang nắm
func NewPrizeStore(prizes []*domain.Prize) *PrizeStore {
	return &PrizeStore{prizes: prizes}
}

// DrawAndTake quay một giải theo trọng số rồi trừ tồn kho, an toàn khi
// nhiều goroutine gọi cùng lúc. Trả về domain.ErrNoPrizeAvailable khi
// mọi giải đã hết hàng.
func (s *PrizeStore) DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (*domain.Prize, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prize, err := domain.Draw(s.prizes)
	if err != nil {
		return nil, err
	}

	if err := prize.Take(); err != nil {
		return nil, err
	}

	return prize, nil
}
