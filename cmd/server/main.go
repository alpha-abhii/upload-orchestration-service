package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/alpha-abhii/upload-orchestration-service/internal/config"
	"github.com/alpha-abhii/upload-orchestration-service/internal/handler"
	"github.com/alpha-abhii/upload-orchestration-service/internal/middleware"
	"github.com/alpha-abhii/upload-orchestration-service/internal/service"
	"github.com/alpha-abhii/upload-orchestration-service/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading from system environment")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	s3Store, err := storage.NewS3Store(cfg)
	if err != nil {
		slog.Error("failed to initialize S3 store", "error", err)
		os.Exit(1)
	}

	uploadService := service.NewUploadService(s3Store, cfg)
	uploadHandler := handler.NewUploadHandler(uploadService)

	rateLimiter := middleware.NewRateLimiter(100, time.Minute)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.InjectRequestID)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.MaxBodySize(1 * 1024 * 1024)) // 1MB max JSON body
	r.Use(rateLimiter.Limit)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/upload/initiate", uploadHandler.Initiate)
		r.Post("/upload/presigned-urls", uploadHandler.GetPresignedURLs)
		r.Post("/upload/complete", uploadHandler.Complete)
		r.Delete("/upload/abort", uploadHandler.Abort)
		r.Get("/upload/status", uploadHandler.GetUploadStatus)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		serverErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server error", "error", err)
		os.Exit(1)
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}