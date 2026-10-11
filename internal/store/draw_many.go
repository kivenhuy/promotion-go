package store

import (
	"context"
	"sync"

	"luckydraw/internal/domain"
)

type drawer interface {
	DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (*domain.Prize, error)
}

// DrawMany quay n lượt song song và trả về các giải trúng được.
// Lượt nào gặp hết hàng thì bỏ qua, không đưa vào kết quả.
func DrawMany(st drawer, programID string, n int) []*domain.Prize {
	won, _ := DrawManyContext(context.Background(), st, "", n)
	return won
}

// DrawManyContext quay n lượt song song, dừng sớm khi ctx bị cancel.
// Trả về các giải đã quay được kèm lý do dừng; err là nil khi chạy trọn n lượt.
func DrawManyContext(ctx context.Context, st drawer, programID string, n int) ([]*domain.Prize, error) {
	// Buffer bằng n: mọi goroutine đều gửi được ngay, send không bao giờ block.
	result := make(chan *domain.Prize, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Đã cancel thì không quay nữa — tránh trừ kho cho một lượt
			// mà không ai còn chờ kết quả.
			select {
			case <-ctx.Done():
				return
			default:
			}

			prize, err := st.DrawAndTake(ctx, int64(i+1), programID, "")
			if err != nil {
				return // hết hàng
			}

			// Tới đây kho ĐÃ bị trừ, nên prize bắt buộc phải đi tới nơi.
			// Send thẳng, không select: buffer đã đủ chỗ nên không bao giờ block.
			result <- prize
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	won := make([]*domain.Prize, 0, n)
	for p := range result {
		won = append(won, p)
	}

	return won, ctx.Err()
}

func DrawManyPool(ctx context.Context, st drawer, programID string, n, workers int) ([]*domain.Prize, error) {
	result := make(chan *domain.Prize, n)
	jobCh := make(chan int)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {

				// Đã cancel thì không quay nữa — tránh trừ kho cho một lượt
				// mà không ai còn chờ kết quả.
				select {
				case <-ctx.Done():
					return
				default:
				}

				prize, err := st.DrawAndTake(ctx, int64(job), programID, "")
				if err != nil {
					continue
				}

				// Tới đây kho ĐÃ bị trừ, nên prize bắt buộc phải đi tới nơi.
				// Send thẳng, không select: buffer đã đủ chỗ nên không bao giờ block.
				result <- prize
			}
		}()

	}

	go func() {
		defer close(jobCh) // đóng khi rót xong HOẶC khi cancel
		for i := range n {
			select {
			case jobCh <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(result)
	}()

	won := make([]*domain.Prize, 0, n)
	for p := range result {
		won = append(won, p)
	}

	return won, ctx.Err()
}
