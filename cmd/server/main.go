package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"luckydraw/internal/api"
	"luckydraw/internal/store"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres@localhost:5432/luckydraw?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("mở DB: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)                 // tối đa 25 kết nối cùng lúc
	db.SetMaxIdleConns(25)                 // giữ sẵn 25 kết nối rảnh
	db.SetConnMaxLifetime(5 * time.Minute) // đóng và mở lại sau 5 phút
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Fatalf("không kết nối được DB: %v", err)

	}
	st := store.NewPostgresStore(db)
	h := api.NewHandler(st, logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"

	}
	addr := ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", addr) // tách ra field
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server lỗi: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info("Nhận tính hiệu dừng , các lượt chờ đang chạy ....")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown lỗi",
			"erorr", err)
	}

	logger.Info("đã tắt sạch")

}
