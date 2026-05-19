package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"warm-migration-core/internal/api"
	"warm-migration-core/internal/host"
	"warm-migration-core/internal/node"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("warm-migrationd", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	listen := flags.String("listen", "", "HTTP listen address, for example 127.0.0.1:8080")
	showVersion := flags.Bool("version", false, "print version and exit")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintf(os.Stdout, "warm-migrationd %s\n", version)
		return nil
	}

	cfg := node.ConfigFromEnv(os.Getenv)
	if *listen != "" {
		cfg.ListenAddr = *listen
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewServer(api.WithHostService(host.NewLocalService())),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals()...)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stdout, "warm-migrationd listening on %s\n", cfg.ListenAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
