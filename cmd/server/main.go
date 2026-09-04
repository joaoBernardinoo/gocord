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

	"ipv6call/internal/config"
	"ipv6call/internal/httpapp"
	"ipv6call/internal/rooms"
	"ipv6call/internal/signaling"
)

func main() {
	logLevel := new(slog.LevelVar)
	if os.Getenv("DEBUG_SIGNALING") == "1" || os.Getenv("DEBUG_SIGNALING") == "true" {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	roomManager := rooms.NewManager(cfg.RoomTTL, cfg.EmptyRoomGrace, cfg.MaxRooms)
	signalingHandler := signaling.NewHandler(roomManager)
	app, err := httpapp.New(cfg, roomManager, signalingHandler)
	if err != nil {
		slog.Error("initialize HTTP app", "error", err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		slog.Error("listen failed", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				roomManager.Cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP shutdown failed", "error", err)
		}
	}()

	slog.Info("signaling server listening", "addr", listener.Addr())
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve failed", "error", err)
		os.Exit(1)
	}
}
