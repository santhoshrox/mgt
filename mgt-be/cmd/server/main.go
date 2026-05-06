package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santhoshrox/mgt-be/internal/ai"
	"github.com/santhoshrox/mgt-be/internal/api"
	"github.com/santhoshrox/mgt-be/internal/auth"
	"github.com/santhoshrox/mgt-be/internal/config"
	"github.com/santhoshrox/mgt-be/internal/core"
	"github.com/santhoshrox/mgt-be/internal/crypto"
	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/grpcserver"
	"github.com/santhoshrox/mgt-be/internal/mergequeue"
	"github.com/santhoshrox/mgt-be/internal/search"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dbConn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db open", "err", err)
		os.Exit(1)
	}
	defer dbConn.Close()
	if err := dbConn.Migrate(ctx); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	sealer, err := crypto.NewSealer(cfg.MasterKey)
	if err != nil {
		slog.Error("sealer", "err", err)
		os.Exit(1)
	}
	signer := auth.NewSessionSigner(cfg.SessionSecret)
	aiClient := ai.New(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel)
	searchClient := search.New(cfg.OpenSearchURL)
	if err := searchClient.EnsureIndex(ctx); err != nil {
		slog.Warn("opensearch ensure index", "err", err)
	}

	worker := mergequeue.New(dbConn, sealer, cfg.WebhookPollFallbackSeconds)
	c := core.New(cfg, dbConn, sealer, signer, aiClient, searchClient, worker)
	srv := api.NewServer(c)

	if cfg.WorkerEnabled {
		go worker.Run(ctx)
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	gsrv, err := grpcserver.New(c)
	if err != nil {
		slog.Error("grpc setup", "err", err)
		os.Exit(1)
	}
	grpcLn, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("grpc listen", "err", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = httpServer.Shutdown(shutdownCtx)
		gsrv.GracefulStop()
	}()

	go func() {
		slog.Info("mgt-be grpc listening", "addr", cfg.GRPCAddr)
		if err := gsrv.Serve(grpcLn); err != nil {
			slog.Error("grpc serve", "err", err)
		}
	}()

	slog.Info("mgt-be listening", "addr", cfg.Addr, "ui", cfg.UIBaseURL)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}
