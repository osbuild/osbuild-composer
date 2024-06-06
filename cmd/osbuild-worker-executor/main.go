package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// based on the excellent post
// https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/

func newServer(logger *logrus.Logger, config *Config) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, logger, config)
	var handler http.Handler = mux
	// todo: consider centralize logginer here?
	//handler = loggingMiddleware(handler)
	return handler
}

func run(ctx context.Context, args []string, getenv func(string) string, logger *logrus.Logger) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	config, err := newConfigFromCmdline(args)
	if err != nil {
		return err
	}

	srv := newServer(logger, config)
	httpServer := &http.Server{
		Addr:              net.JoinHostPort(config.Host, config.Port),
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		logger.Printf("listening on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
	}()

	// todo: this seems kinda complicated, why a waitgroup and not
	// do it flat?
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
		}
	}()
	wg.Wait()

	// cleanup
	if err := os.RemoveAll(config.BuildDirBase); err != nil {
		logger.Errorf("cannot cleanup: %v", err)
		return err
	}

	return nil
}

func main() {
	logger := logrus.New()
	ctx := context.Background()
	if err := run(ctx, os.Args, os.Getenv, logger); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
