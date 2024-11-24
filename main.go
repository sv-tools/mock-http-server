package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	conf := flag.StringP("config", "c", "config.yaml", "config file")
	port := flag.IntP("port", "p", 8080, "http port")
	flag.Parse()

	if v := os.Getenv("CONFIG"); v != "" && !flag.Lookup("config").Changed {
		conf = &v
	}
	if v := os.Getenv("PORT"); v != "" && !flag.Lookup("port").Changed {
		p, err := strconv.Atoi(v)
		if err != nil {
			log.Error(fmt.Sprintf("wrong value %q of env variable PORT", v), err)
			os.Exit(1)
		}
		port = &p
	}

	f, err := os.Open(*conf)
	if err != nil {
		log.Error(fmt.Sprintf("wrong config file %q", *conf), err)
		os.Exit(1)
	}

	var config Config
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&config); err != nil {
		log.Error("decoding config failed", err)
		os.Exit(1)
	}
	if config.Port != 0 {
		port = &config.Port
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
		mux.HandleFunc(
			route.Pattern,
			StructuredLogger(log, config.RequestIDHeader, responsesWriter(route.Responses, log)),
		)
	}
	if !isRootRegistered {
		mux.HandleFunc("/", StructuredLogger(log, config.RequestIDHeader, http.NotFound))
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           mux,
		ReadHeaderTimeout: 1 * time.Second,
	}
	serverCtx, serverStopCtx := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, shutdownCancelCtx := context.WithTimeout(serverCtx, 30*time.Second)
		defer shutdownCancelCtx()

		go func() {
			<-shutdownCtx.Done()
			log.Info("graceful shutdown")
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Error("graceful shutdown timed out.. forcing exit.", nil)
				os.Exit(1)
			}
		}()

		// Trigger graceful shutdown
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", err)
		}
		serverStopCtx()
	}()

	// Run the server
	log.Info(fmt.Sprintf("Listen on http://localhost:%d", *port))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("starting failed", err)
		os.Exit(1)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}
