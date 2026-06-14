package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.json", "Path to config file")
	interactive := flag.Bool("i", false, "Start interactive CLI mode")
	noHTTP := flag.Bool("no-http", false, "Skip HTTP server (use with -i)")
	flag.Parse()

	// Setup structured logging — suppress in interactive mode unless debug
	logLevel := slog.LevelInfo
	if *interactive {
		logLevel = slog.LevelError // quiet in CLI mode
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))

	slog.Info("goclaw channel bridge starting")

	// Load config
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create bridge
	bridge := NewBridge(cfg)

	// Connect to goclaw with retry
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := connectWithRetry(ctx, bridge); err != nil {
		slog.Error("failed to connect to goclaw", "error", err)
		os.Exit(1)
	}

	// Start HTTP server (unless skipped)
	var httpServer *http.Server
	if !*noHTTP {
		handler := NewHandler(bridge, cfg)
		mux := http.NewServeMux()
		handler.RegisterRoutes(mux)
		corsMux := corsMiddleware(mux)

		httpServer = &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      corsMux,
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 5 * time.Minute,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			if !*interactive {
				fmt.Printf("\n  HTTP API: http://localhost%s\n\n", cfg.ListenAddr)
			}
			slog.Info("HTTP server starting", "addr", cfg.ListenAddr)
			htErr := httpServer.ListenAndServe()
			if htErr != nil && htErr != http.ErrServerClosed {
				if *interactive {
					slog.Warn("HTTP server failed (non-fatal in interactive mode)", "addr", cfg.ListenAddr, "error", htErr)
				} else {
					slog.Error("HTTP server error", "error", htErr)
					cancel()
				}
			}
		}()
	}

	// Interactive CLI mode
	if *interactive {
		cli := NewCLI(bridge, cfg)
		cli.Run() // blocks until /quit or EOF
		// Shutdown
		if httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			httpServer.Shutdown(shutdownCtx)
		}
		bridge.Disconnect()
		return
	}

	// Non-interactive: wait for shutdown signal
	printBanner(cfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig)
	case <-ctx.Done():
		slog.Info("context cancelled")
	}

	// Graceful shutdown
	slog.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}

	bridge.Disconnect()
	slog.Info("bridge stopped")
}

// connectWithRetry tries to connect to goclaw with exponential backoff.
func connectWithRetry(ctx context.Context, bridge *Bridge) error {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := bridge.Connect(ctx); err != nil {
			delay := time.Duration(i+1) * 2 * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			slog.Warn("connection failed, retrying...",
				"attempt", i+1,
				"max_retries", maxRetries,
				"delay", delay,
				"error", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to connect after %d retries", maxRetries)
}

// corsMiddleware adds CORS headers for browser access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// printBanner prints startup info.
func printBanner(cfg *Config) {
	fmt.Println(`
  ╔══════════════════════════════════════════════════╗
  ║       GoClaw Channel Bridge v1.0.0               ║
  ║  HTTP ↔ WebSocket bridge for GoClaw Gateway      ║
  ╚══════════════════════════════════════════════════╝
`)
	fmt.Printf("  Goclaw URL:    %s\n", cfg.GoclawURL)
	fmt.Printf("  Agent Key:     %s\n", cfg.AgentKey)
	fmt.Printf("  User ID:       %s\n", cfg.UserID)
	fmt.Printf("  Listen Addr:   %s\n", cfg.ListenAddr)
	fmt.Printf("  Session Mode:  %s\n", cfg.SessionMode)
	fmt.Println()
}
