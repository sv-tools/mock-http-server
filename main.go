package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"go.yaml.in/yaml/v4"
)

func main() {
	InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		cancel()
	}()

	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	if err := run(ctx, args); err != nil {
		slog.Error("Exiting with error", slog.String("error", err.Error()))
		_ = os.Stderr.Sync() // ensure all output is flushed before exiting
		cancel()
		os.Exit(1) //nolint:gocritic // cancel is called manually
	}
}

func getConfig(args []string) (config *Config, retError error) {
	fs := flag.NewFlagSet("mock-http-server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var port int
	if v, ok := os.LookupEnv("PORT"); ok {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("wrong value %q of env variable PORT: %w", v, err)
		}
		port = p
	}

	configPath := fs.StringP("config", "c", "config.yaml", "config file")
	fsPort := fs.IntP("port", "p", 8080, "http port")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parsing commandline arguments failed: %w", err)
	}

	if fs.Lookup("port").Changed {
		port = *fsPort
	}

	if v, ok := os.LookupEnv("CONFIG"); ok && !fs.Lookup("config").Changed {
		*configPath = v
	}

	f, err := os.Open(*configPath)
	if err != nil {
		return nil, fmt.Errorf("wrong config file %q: %w", *configPath, err)
	}
	defer func(f *os.File) {
		if err := f.Close(); err != nil {
			retError = errors.Join(retError, fmt.Errorf("failed to close config file %q: %w", *configPath, err))
		}
	}(f)

	var conf Config
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&conf); err != nil {
		return nil, fmt.Errorf("decoding config failed: %w", err)
	}
	if port != 0 {
		conf.Port = port
	}
	if conf.Port == 0 {
		conf.Port = *fsPort
	}
	if conf.RequestIDHeader == "" {
		conf.RequestIDHeader = "X-Request-ID"
	}

	return &conf, nil
}

// run executes the main logic. When startServer is false it stops after constructing the server and
// returns immediately.
// Returns (exitCode, serverAddress, error)
func run(
	ctx context.Context,
	args []string,
) (retError error) {
	config, err := getConfig(args)
	if err != nil {
		return err
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
			config.RequestIDHeader,
			responsesWriter(route.Responses),
		))
	}
	if !isRootRegistered {
		mux.HandleFunc("/", StructuredLogger(config.RequestIDHeader, http.NotFound))
	}

	addr := fmt.Sprintf(":%d", config.Port)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 1 * time.Second}
	server.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()
	go func() {
		defer serverCancel()
		<-ctx.Done()
		slog.Info("Shutdown signal received")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		go func() {
			if err := server.Shutdown(shutdownCtx); err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					retError = errors.Join(retError, fmt.Errorf("server shutdown failed: %w", err))
				}
			}
			shutdownCancel()
		}()
		<-shutdownCtx.Done()
		if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
			retError = errors.Join(retError, errors.New("server shutdown timed out"))
		}
	}()

	slog.Info("Starting server", slog.String("addr", fmt.Sprintf("http://localhost:%d", config.Port)))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("starting failed: %w", err)
	}
	<-serverCtx.Done()
	return
}
