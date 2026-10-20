package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"luckydraw/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) DrawAndTake(ctx context.Context, userID int64, programID string, idemKey string) (domain.DrawResult, error) {
	prizes, err := s.availablePrize(ctx, programID)
	if err != nil {
		return domain.DrawResult{}, fmt.Errorf("availablePrize: %w", err)
	}
	prize, err := domain.Draw(prizes)
	if err != nil {
		return domain.DrawResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.DrawResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	const insIdem = `
		INSERT INTO idempotency_record (key, user_id, program_id, package_id)
		VALUES ($1,$2,$3,$4)
	`

	if _, err := tx.ExecContext(ctx, insIdem, idemKey, userID, programID, prize.ID); err != nil {
		var pgERR *pgconn.PgError
		if errors.As(err, &pgERR) && pgERR.Code == "23505" {
			_ = tx.Rollback()
			return s.replay(ctx, idemKey, userID)
		}
		return domain.DrawResult{}, fmt.Errorf("insert idempotency: %w", err)
	}

	const q = `
		INSERT INTO user_quota (user_id, program_id, package_id, used)
		VALUES ($1,$2,$3,1)
		ON CONFLICT (user_id, program_id, package_id)
		DO UPDATE SET used = user_quota.used + 1, updated_at = now ()
		WHERE user_quota.used <$4
	`

	res, err := tx.ExecContext(ctx, q, userID, programID, prize.ID, prize.MaxPerUser)
	if err != nil {
		return domain.DrawResult{}, fmt.Errorf("update user_quota fail: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.DrawResult{}, fmt.Errorf("update user_quota fail: %w", err)
	}
	if n == 0 {
		return domain.DrawResult{}, fmt.Errorf("user %d: %w", userID, domain.ErrQuotaExceeded)
	}

	// Ghi log TRƯỚC khi trừ kho: draw_log không ai tranh, còn gift_package là hot row.
	// Đặt UPDATE gift_package cuối cùng để row lock chỉ kéo dài qua UPDATE + COMMIT.
	// Nếu hết quà, updateStock trả lỗi và defer tx.Rollback() huỷ luôn dòng log này.
	const l = `
		INSERT INTO draw_log (user_id, program_id, package_id, status)
		VALUES ($1,$2,$3,'success')
	`
	if _, err := tx.ExecContext(ctx, l, userID, programID, prize.ID); err != nil {
		return domain.DrawResult{}, fmt.Errorf("create new log fail: %w", err)
	}

	err = s.updateStock(ctx, tx, programID, prize.ID)
	if err != nil {
		return domain.DrawResult{}, fmt.Errorf("Update stock fail: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.DrawResult{}, fmt.Errorf("commit: %w", err)
	}
	return domain.DrawResult{Prize: prize, Replayed: false}, nil
}

func (s *PostgresStore) availablePrize(ctx context.Context, programID string) ([]*domain.Prize, error) {
	const q = `
		SELECT package_id, name, weight, total_quantity - win_quantity AS remaining,max_per_user
		FROM gift_package
		WHERE program_id = $1 AND win_quantity < total_quantity
	`

	rows, err := s.db.QueryContext(ctx, q, programID)
	if err != nil {
		return nil, fmt.Errorf("query gift_package: %w", err)
	}
	defer rows.Close()
	var prizes []*domain.Prize

	for rows.Next() {
		var (
			id           int
			name         string
			weight       int
			remaining    int
			max_per_user int
		)

		if err := rows.Scan(&id, &name, &weight, &remaining, &max_per_user); err != nil {
			return nil, fmt.Errorf("scan gift_package: %w", err)
		}

		prizes = append(prizes, &domain.Prize{
			ID:         id,
			Name:       name,
			Weight:     weight,
			Quantity:   remaining,
			MaxPerUser: max_per_user,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("duyệt gift_package: %w", err)
	}

	return prizes, nil
}

func (s *PostgresStore) updateStock(ctx context.Context, tx *sql.Tx, programID string, packageID int) error {
	const q = `
		Update gift_package 
		SET win_quantity = win_quantity + 1
		WHERE win_quantity < total_quantity AND package_id = $2 AND program_id = $1
	`

	res, err := tx.ExecContext(ctx, q, programID, packageID)
	if err != nil {
		return fmt.Errorf("update gift_package: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update gift_package: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("package %d: %w", packageID, domain.ErrNoPrizeAvailable)
	}
	return nil
}

func (s *PostgresStore) replay(ctx context.Context, idemKey string, userID int64) (domain.DrawResult, error) {
	const replay = `
		SELECT r.user_id , r.package_id , g.name
		FROM idempotency_record r
		JOIN gift_package g
			ON g.program_id = r.program_id AND g.package_id = r.package_id
		WHERE r.key = $1
	`

	var (
		ownerID   int64
		packageID int
		name      string
	)
	err := s.db.QueryRowContext(ctx, replay, idemKey).Scan(&ownerID, &packageID, &name)
	if errors.Is(err, sql.ErrNoRows) {
		// vừa nhận 23505 mà không thấy record — bất thường
	}
	if err != nil {
		// lỗi hạ tầng
	}

	if ownerID != userID {
		return domain.DrawResult{}, fmt.Errorf("key %q: %w", idemKey, domain.ErrKeyConflict)
	}

	return domain.DrawResult{Prize: &domain.Prize{ID: packageID, Name: name}, Replayed: true}, nil

}
