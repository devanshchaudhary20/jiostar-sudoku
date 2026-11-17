// Command server runs the Sudoku API over HTTP.
//
//	go run ./cmd/server             # listens on :8080
//	PORT=9000 go run ./cmd/server   # listens on :9000
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jiostar/sudoku/internal/app"
	"jiostar/sudoku/internal/httpapi"
)

const defaultPort = "8080"

func main() {
	logger := log.New(os.Stdout, "sudoku ", log.LstdFlags)

	ctrl, err := app.New(time.Now().UnixNano())
	if err != nil {
		logger.Fatalf("startup: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewHandler(ctrl, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Shut down gracefully on Ctrl+C / SIGTERM so in-flight moves finish.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Printf("listening on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown: %v", err)
	}
}
