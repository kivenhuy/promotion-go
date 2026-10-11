package domain

import (
	"testing"
)

func TestTake(t *testing.T) {
	p, err := NewPrize(1, "Iphone", 2, 10)

	if err != nil {
		t.Fatalf("NewPrize() lỗi :%v; want nil", err)
	}

	if err := p.Take(); err != nil {
		t.Fatalf("Take() lần 1 lỗi : %v; want nil", err)
	}

	if p.Quantity != 1 {
		t.Errorf("Sau Take() lần 1: Quantity = %d; want 1", p.Quantity)
	}

	if err := p.Take(); err != nil {
		t.Fatalf("Take() lần 2 lỗi : %v; want nil", err)
	}

	if p.Quantity != 0 {
		t.Errorf("Sau Take() lần 2: Quantity = %d; want 0", p.Quantity)
	}

	if err := p.Take(); err == nil {
		t.Fatalf("Take() lần 3 lỗi : %v; want lỗi vì đã hết hàng", err)
	}

	if p.Quantity != 0 {
		t.Errorf("Sau Take() lần 2: Quantity = %d; want 0 (Không được âm)", p.Quantity)
	}
}
