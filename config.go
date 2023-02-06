package main

import (
	"net/http"
	"os"

	"golang.org/x/exp/slog"
)

type Config struct {
	RequestIDHeader string  `json:"request_id_header,omitempty" yaml:"request_id_header,omitempty"`
	Routes          []Route `json:"routes" yaml:"routes"`
	Port            int     `json:"port,omitempty" yaml:"port,omitempty"`
}

type Route struct {
	Method    string     `json:"method,omitempty" yaml:"method,omitempty"`
	Pattern   string     `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Responses []Response `json:"responses" yaml:"responses"`
}

type Response struct {
	Headers http.Header `json:"headers,omitempty" yaml:"headers,omitempty"`
	Repeat  *int        `json:"repeat,omitempty" yaml:"repeat,omitempty"`
	Body    string      `json:"body,omitempty" yaml:"body,omitempty"`
	File    string      `json:"file,omitempty" yaml:"file,omitempty"`
	Code    int         `json:"code,omitempty" yaml:"code,omitempty"`
	IsJSON  bool        `json:"is_json,omitempty" yaml:"is_json,omitempty"`
}

func responsesWriter(responses []Response, log *slog.Logger) http.HandlerFunc {
	var i int
	return func(writer http.ResponseWriter, request *http.Request) {
		for {
			if i > len(responses)-1 {
				http.NotFound(writer, request)
				return
			}
			response := responses[i]
			if response.Repeat != nil {
				if *response.Repeat <= 0 {
					i++
					continue
				}
				*response.Repeat--
			}

			var data []byte
			if response.File != "" {
				var err error
				data, err = os.ReadFile(response.File)
				if err != nil {
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
			} else if len(response.Body) > 0 {
				data = []byte(response.Body)
			}

			for name, header := range response.Headers {
				for _, value := range header {
					writer.Header().Add(name, value)
				}
			}
			if response.IsJSON {
				if writer.Header().Get("Content-Type") == "" {
					writer.Header().Set("Content-Type", "application/json")
				}
			}
			writer.WriteHeader(response.Code)

			if len(data) > 0 {
				if _, err := writer.Write(data); err != nil {
					log.Error("sending response failed", err)
				}
			}
			return
		}
	}
}
