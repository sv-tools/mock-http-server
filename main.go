package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

func main() {
	conf := flag.StringP("config", "c", "config.yaml", "config file")
	port := flag.IntP("port", "p", 8080, "http port")
	flag.Parse()

	if v := os.Getenv("CONFIG"); v != "" && !flag.Lookup("config").Changed {
		conf = &v
	}
	if v := os.Getenv("PORT"); v != "" && !flag.Lookup("port").Changed {
		p, err := strconv.Atoi(v)
		if err != nil {
			log.Fatal(err)
		}
		port = &p
	}

	f, err := os.Open(*conf)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&config); err != nil {
		panic(err)
	}
	if config.Port != 0 {
		port = &config.Port
	}

	if config.RequestIDHeader != "" {
		middleware.RequestIDHeader = config.RequestIDHeader
	}
	r := chi.NewRouter().With(middleware.Logger, middleware.RequestID)
	for _, route := range config.Routes {
		if route.Method == "" {
			route.Method = "GET"
		} else {
			chi.RegisterMethod(route.Method)
		}
		if route.Pattern == "" {
			route.Pattern = "/"
		}
		r.MethodFunc(route.Method, route.Pattern, responsesWriter(route.Responses))
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           r,
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
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		// Trigger graceful shutdown
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()

	// Run the server
	log.Printf("Listen on http://localhost:%d\n", *port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}
