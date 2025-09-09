package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"go.yaml.in/yaml/v4"
)

func main() {
	code, _, _ := run(os.Args[1:], getEnvMap(), os.Stderr, true)
	if code != 0 {
		os.Exit(code)
	}
}

func getEnvMap() map[string]string {
	m := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			m[kv[:idx]] = kv[idx+1:]
		}
	}
	return m
}

// run executes the main logic. When startServer is false it stops after constructing the server and
// returns immediately.
// Returns (exitCode, serverAddress, error)
func run( //nolint:gocritic // no need for named return values here, it's clear enough as is.
	args []string,
	env map[string]string,
	stderr io.Writer,
	startServer bool,
) (int, string, error) {
	fs := flag.NewFlagSet("mock-http-server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	conf := fs.StringP("config", "c", "config.yaml", "config file")
	port := fs.IntP("port", "p", 8080, "http port")
	if err := fs.Parse(args); err != nil {
		return 1, "", err
	}

	if v, ok := env["CONFIG"]; ok && !fs.Lookup("config").Changed {
		*conf = v
	}
	if v, ok := env["PORT"]; ok && !fs.Lookup("port").Changed {
		p, err := strconv.Atoi(v)
		if err != nil {
			return 1, "", fmt.Errorf("wrong value %q of env variable PORT: %w", v, err)
		}
		*port = p
	}

	log := slog.New(slog.NewJSONHandler(stderr, nil))

	f, err := os.Open(*conf)
	if err != nil {
		return 1, "", fmt.Errorf("wrong config file %q: %w", *conf, err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Error("closing config file failed", slog.String("error", err.Error()))
		}
	}(f)

	var config Config
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&config); err != nil {
		return 1, "", fmt.Errorf("decoding config failed: %w", err)
	}
	if config.Port != 0 {
		*port = config.Port
	}
	if config.RequestIDHeader == "" {
		config.RequestIDHeader = "X-Request-ID"
	}

	mux := http.NewServeMux()
	var isRootRegistered bool
	for _, route := range config.Routes {
		if route.Pattern == "" {
			route.Pattern = "/"
		}
		if route.Pattern == "/" {
			isRootRegistered = true
		}
		mux.HandleFunc(route.Pattern, StructuredLogger(
			log,
			config.RequestIDHeader,
			responsesWriter(route.Responses, log),
		))
	}
	if !isRootRegistered {
		mux.HandleFunc("/", StructuredLogger(log, config.RequestIDHeader, http.NotFound))
	}

	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 1 * time.Second}

	if !startServer {
		return 0, addr, nil
	}

	serverCtx, serverStopCtx := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		shutdownCtx, shutdownCancelCtx := context.WithTimeout(serverCtx, 30*time.Second)
		defer shutdownCancelCtx()
		go func() {
			<-shutdownCtx.Done()
			log.Info("graceful shutdown")
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Error("graceful shutdown timed out.. forcing exit.")
				os.Exit(1)
			}
		}()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		}
		serverStopCtx()
	}()

	log.Info(fmt.Sprintf("Listen on http://localhost:%d", *port))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return 1, addr, fmt.Errorf("starting failed: %w", err)
	}
	<-serverCtx.Done()
	return 0, addr, nil
}
