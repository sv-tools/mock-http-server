package main

import (
	"fmt"
	"log/slog"
	"net/http"
)

type wrapper struct {
	writer http.ResponseWriter
	status int
	bytes  int
}

func (w *wrapper) Header() http.Header {
	return w.writer.Header()
}

func (w *wrapper) WriteHeader(status int) {
	w.status = status
	w.writer.WriteHeader(status)
}

func (w *wrapper) Write(b []byte) (int, error) {
	n, err := w.writer.Write(b)
	w.bytes += n
	return n, err
}

var _ http.ResponseWriter = &wrapper{}

func StructuredLogger(log *slog.Logger, reqIDHeader string, next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		wr := &wrapper{writer: writer}
		next.ServeHTTP(wr, request)

		scheme := "http"
		if request.TLS != nil {
			scheme = "https"
		}

		log.LogAttrs(request.Context(), slog.LevelInfo, "request completed",
			slog.String("http_scheme", scheme),
			slog.String("http_proto", request.Proto),
			slog.String("http_method", request.Method),
			slog.String("remote_addr", request.RemoteAddr),
			slog.String("user_agent", request.UserAgent()),
			slog.String("uri", fmt.Sprintf("%s://%s%s", scheme, request.Host, request.RequestURI)),
			slog.Int("resp_status", wr.status),
			slog.Int("resp_byte_length", wr.bytes),
			slog.String("request_id", request.Header.Get(reqIDHeader)),
		)
	}
}
