package domain

import (
	"errors"
	"math/rand/v2"
)

type DrawResult struct {
	Prize    *Prize
	Replayed bool
}

// Sentinel error: khai báo 1 lần ở package level để caller so sánh được.
var ErrNoPrizeAvailable = errors.New("no prize available")
var ErrQuotaExceeded = errors.New("You don`t have any turn")
var ErrKeyConflict = errors.New("Key không trùng khớp")

func Draw(prizes []*Prize) (*Prize, error) {
	// Vòng 1: tính tổng trọng số của các giải CÒN HÀNG.
	totalWeight := 0
	for _, p := range prizes {
		if !p.Available() {
			continue // bỏ qua giải hết hàng, không phải bỏ cuộc
		}
		totalWeight += p.Weight
	}

	if totalWeight == 0 {
		return nil, ErrNoPrizeAvailable
	}

	// Chọn một điểm ngẫu nhiên trên trục [0, totalWeight)
	pick := rand.IntN(totalWeight)

	// Vòng 2: cộng dồn đúng tập giải như vòng 1, tìm ô chứa điểm đã chọn.
	cumulative := 0
	for _, p := range prizes {
		if !p.Available() {
			continue // BẮT BUỘC giống hệt vòng 1, nếu không tổng sẽ lệch
		}
		cumulative += p.Weight
		if pick < cumulative {
			return p, nil
		}
	}

	// Không bao giờ tới đây (pick < totalWeight nên luôn có giải trúng),
	// nhưng Go bắt buộc mọi nhánh phải return.
	return nil, ErrNoPrizeAvailable
}
