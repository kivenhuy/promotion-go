package domain

import (
	"errors"
	"testing"
)

func TestDraw_Empty(t *testing.T) {
	prizes := []*Prize{}
	p, err := Draw(prizes)

	if err == nil {
		t.Error("Expected error for empty prizes")
	}
	if p != nil {
		t.Error("Expected nil prize")
	}
}

func TestDraw_NoAvailable(t *testing.T) {
	p1 := &Prize{Name: "P1", Quantity: 0, Weight: 10}
	p2 := &Prize{Name: "P2", Quantity: 0, Weight: 20}
	prizes := []*Prize{p1, p2}

	p, err := Draw(prizes)

	if err == nil || !errors.Is(err, ErrNoPrizeAvailable) {
		t.Errorf("Expected ErrNoPrizeAvailable, got %v", err)
	}
	if p != nil {
		t.Error("Expected nil prize")
	}
}

// mustPrize dựng Prize qua constructor và fail test ngay nếu dữ liệu không hợp lệ.
func mustPrize(t *testing.T, id int, name string, qty, weight int) *Prize {
	t.Helper() // lỗi sẽ được quy về DÒNG GỌI mustPrize, không phải dòng bên trong đây
	p, err := NewPrize(id, name, qty, weight)
	if err != nil {
		t.Fatalf("NewPrize(%d, %q, %d, %d) lỗi: %v", id, name, qty, weight, err)
	}
	return p
}

// Giải hết hàng phải bị LOẠI khỏi vòng quay, không được làm hỏng cả lượt quay.
func TestDraw_SkipsUnavailable(t *testing.T) {
	soldOut := mustPrize(t, 1, "iPhone", 0, 90) // hết hàng nhưng weight rất cao
	inStock := mustPrize(t, 2, "Voucher", 10, 10)
	prizes := []*Prize{soldOut, inStock}

	for i := 0; i < 100; i++ {
		got, err := Draw(prizes)
		if err != nil {
			t.Fatalf("lượt %d: Draw() lỗi %v; want nil", i, err)
		}
		if got.ID != inStock.ID {
			t.Fatalf("lượt %d: Draw() = %q; want %q", i, got.Name, inStock.Name)
		}
	}
}

// Quay nhiều lần thì tỉ lệ trúng phải bám sát trọng số 70/20/10.
func TestDraw_Distribution(t *testing.T) {
	const (
		rounds    = 10000
		tolerance = 0.03 // cho phép lệch 3 điểm phần trăm
	)

	prizes := []*Prize{
		mustPrize(t, 1, "Thường", 1000, 70),
		mustPrize(t, 2, "Hiếm", 1000, 20),
		mustPrize(t, 3, "Cực hiếm", 1000, 10),
	}

	counts := make(map[int]int)
	for i := 0; i < rounds; i++ {
		got, err := Draw(prizes)
		if err != nil {
			t.Fatalf("lượt %d: Draw() lỗi %v; want nil", i, err)
		}
		counts[got.ID]++ // key chưa có vẫn chạy: zero value 0 rồi +1
	}

	totalWeight := 0
	for _, p := range prizes {
		totalWeight += p.Weight
	}

	for _, p := range prizes {
		want := float64(p.Weight) / float64(totalWeight)
		got := float64(counts[p.ID]) / float64(rounds)

		t.Logf("%-10s %5d lượt = %.2f%% (kỳ vọng %.2f%%)", p.Name, counts[p.ID], got*100, want*100)

		if diff := got - want; diff > tolerance || diff < -tolerance {
			t.Errorf("%q trúng %.2f%%; want %.2f%% (lệch tối đa %.0f%%)",
				p.Name, got*100, want*100, tolerance*100)
		}
	}
}
