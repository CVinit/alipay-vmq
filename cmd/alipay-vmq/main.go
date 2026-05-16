package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alipay-vmq/internal/alipay"
	"alipay-vmq/internal/config"
	"alipay-vmq/internal/handler"
	"alipay-vmq/internal/poller"
	"alipay-vmq/internal/store"
	"alipay-vmq/internal/vmq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	var st store.Store
	switch cfg.DatabaseDriver {
	case "postgres":
		st, err = store.NewPostgres(cfg.DatabaseURL)
	default:
		st, err = store.NewSQLite(cfg.DatabaseURL)
	}
	if err != nil {
		slog.Error("init store", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := st.Init(ctx); err != nil {
		slog.Error("init database", "error", err)
		os.Exit(1)
	}

	ac, err := alipay.NewClient(cfg.AlipayAppID, cfg.AlipayPrivateKey, cfg.AlipayPublicKey, cfg.AlipaySandbox)
	if err != nil {
		slog.Error("init alipay client", "error", err)
		os.Exit(1)
	}

	vc := vmq.NewClient(cfg.VMQBaseURL, cfg.VMQKey, cfg.VMQDeviceKey)

	srv := handler.NewServer(cfg, st, ac, vc)

	p := poller.New(st, srv, cfg.PollInterval)
	go p.Run(ctx)

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpSrv.Shutdown(shutdownCtx)
}
