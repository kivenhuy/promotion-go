package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"luckydraw/internal/domain"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDB dùng chung cho mọi integration test trong package này.
// nil nghĩa là không kết nối được Postgres — các test sẽ tự skip.
var testDB *sql.DB

// TestMain chạy MỘT lần trước và sau toàn bộ test của package.
// Đây là chỗ duy nhất mở và đóng kết nối, thay vì mở lại ở từng test.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres@localhost:5432/luckydraw?sslmode=disable"
	}

	if db, err := sql.Open("pgx", dsn); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := db.PingContext(ctx); err == nil {
			testDB = db
		} else {
			db.Close()
		}
		cancel()
	}

	code := m.Run() // chạy toàn bộ test

	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// requireDB trả về kết nối, hoặc skip test khi máy không có Postgres.
// Nhờ vậy `go test ./...` vẫn xanh trên máy chưa cài DB.
func requireDB(t *testing.T) *sql.DB {
	t.Helper()
	if testDB == nil {
		if os.Getenv("CI") != "" {
			t.Fatal("running on CI but Postgres is unreachable")
		}
		t.Skip("không kết nối được Postgres — bỏ qua integration test")
	}
	return testDB
}

// seedProgram tạo một chương trình RIÊNG cho test đang chạy (đặt tên theo
// t.Name() nên hai test không bao giờ giẫm chân nhau) và tự dọn khi test xong.
func seedProgram(t *testing.T, db *sql.DB, totalQuantity, maxPerUser int) string {
	t.Helper()

	programID := "test_" + t.Name()
	ctx := context.Background()

	dọnSạch := func() {
		for _, q := range []string{
			`DELETE FROM idempotency_record WHERE program_id = $1`,
			`DELETE FROM draw_log          WHERE program_id = $1`,
			`DELETE FROM user_quota        WHERE program_id = $1`,
			`DELETE FROM gift_package      WHERE program_id = $1`,
		} {
			if _, err := db.ExecContext(ctx, q, programID); err != nil {
				t.Errorf("dọn dữ liệu test: %v", err)
			}
		}
	}

	dọnSạch() // xoá tàn dư của lần chạy trước nếu test lần trước bị kill

	_, err := db.ExecContext(ctx, `
		INSERT INTO gift_package
			(program_id, package_id, name, weight, total_quantity, win_quantity, max_per_user)
		VALUES ($1, 1, 'Phần thưởng test', 10, $2, 0, $3)`,
		programID, totalQuantity, maxPerUser)
	if err != nil {
		t.Fatalf("seed gift_package: %v", err)
	}

	t.Cleanup(dọnSạch)
	return programID
}

// winQuantity đọc số lượng đã phát của chương trình — dùng để khẳng định
// kho thay đổi đúng như mong đợi.
func winQuantity(t *testing.T, db *sql.DB, programID string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(),
		`SELECT win_quantity FROM gift_package WHERE program_id = $1`, programID).Scan(&n)
	if err != nil {
		t.Fatalf("đọc win_quantity: %v", err)
	}
	return n
}

// Gửi hai lần cùng một Idempotency-Key: lần hai phải là bản sao của lần một,
// và kho chỉ được trừ MỘT lần.
func TestDrawAndTake_CungKeyChiTruKhoMotLan(t *testing.T) {
	db := requireDB(t)
	programID := seedProgram(t, db, 100, 5)
	st := NewPostgresStore(db)

	ctx := context.Background()
	const userID int64 = 1
	const key = "key-giong-nhau"

	lan1, err := st.DrawAndTake(ctx, userID, programID, key)
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	if lan1.Replayed {
		t.Error("lần 1: Replayed = true; want false")
	}
	if lan1.Prize == nil {
		t.Fatal("lần 1: Prize = nil; want một giải")
	}

	lan2, err := st.DrawAndTake(ctx, userID, programID, key)
	if err != nil {
		t.Fatalf("lần 2: %v", err)
	}
	if !lan2.Replayed {
		t.Error("lần 2: Replayed = false; want true")
	}
	if lan2.Prize.ID != lan1.Prize.ID {
		t.Errorf("lần 2 trả prize %d; want %d (phải giống lần 1)", lan2.Prize.ID, lan1.Prize.ID)
	}

	if got := winQuantity(t, db, programID); got != 1 {
		t.Errorf("win_quantity = %d; want 1 (hai request cùng key chỉ được tính một lượt)", got)
	}
}

func TestDrawAndTake_KhoMotSuatNhieuNguoiQuay(t *testing.T) {
	db := requireDB(t)
	programID := seedProgram(t, db, 1, 5)
	st := NewPostgresStore(db)
	ctx := context.Background()
	const rounds = 10
	var thanhCong atomic.Int64
	var hetQua atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range rounds {
		userID := int64(1000 + i)
		key := fmt.Sprintf("key-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := st.DrawAndTake(ctx, userID, programID, key)

			switch {
			case err == nil:
				thanhCong.Add(1)
			case errors.Is(err, domain.ErrNoPrizeAvailable):
				hetQua.Add(1)
			default:
				t.Errorf("lỗi không mong đợi: %v", err)
			}
		}()

	}
	close(start)
	wg.Wait()
	if n := thanhCong.Load(); n != 1 {
		t.Errorf("phát ra %d giải; want 1", n)
	}

	if got := winQuantity(t, db, programID); got != 1 {
		t.Errorf("win_quantity = %d; want 1", got)
	}

	var soDongQuota int
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM user_quota WHERE program_id = $1`, programID).Scan(&soDongQuota)
	if err != nil {
		t.Fatalf("đếm user_quota: %v", err)
	}
	if soDongQuota != 1 {
		t.Errorf("user_quota có %d dòng; want 1 (9 transaction thất bại phải rollback sạch)", soDongQuota)
	}
}

func TestDrawAndTake_HetLuot(t *testing.T) {
	db := requireDB(t)
	programID := seedProgram(t, db, 100, 2)
	st := NewPostgresStore(db)
	ctx := context.Background()

	const userID int64 = 1
	const key = "key-1"
	const key2 = "key-2"
	const key3 = "key-3"

	if _, err := st.DrawAndTake(ctx, userID, programID, key); err != nil {
		t.Fatalf("lần 1: %v", err)
	}

	if _, err := st.DrawAndTake(ctx, userID, programID, key2); err != nil {
		t.Fatalf("lần 2: %v", err)
	}

	if _, err := st.DrawAndTake(ctx, userID, programID, key3); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Errorf("lần 3: err = %v; want ErrQuotaExceeded", err)

	}

	if got := winQuantity(t, db, programID); got != 2 {
		t.Errorf("win_quantity = %d; want 2 (chỉ nhận được 2 quà vì win_quantity chỉ là 2)", got)
	}

	var soDongQuota int
	err := db.QueryRowContext(ctx, `SELECT used FROM user_quota WHERE program_id = $1`, programID).Scan(&soDongQuota)
	if err != nil {
		t.Fatalf("đếm user_quota: %v", err)
	}
	if soDongQuota != 2 {
		t.Errorf("user_quota có %d dòng; want 2", soDongQuota)
	}
}
