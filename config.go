package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"text/template"

	"golang.org/x/exp/slog"
)

type Config struct {
	RequestIDHeader string  `json:"request_id_header,omitempty" yaml:"request_id_header,omitempty"`
	Routes          []Route `json:"routes" yaml:"routes"`
	Port            int     `json:"port,omitempty" yaml:"port,omitempty"`
}

type Route struct {
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
			} else if body := response.Body; body != "" {
				var err error
				data, err = executeTemplate(body, request)
				if err != nil {
					panic(err)
				}
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
					log.ErrorContext(request.Context(), "sending response failed", slog.String("error", err.Error()))
				}
			}
			return
		}
	}
}

func executeTemplate(body string, request *http.Request) ([]byte, error) {
	tmpl, err := template.New("response").Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse template failed: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, request); err != nil {
		return nil, fmt.Errorf("execute template failed: %w", err)
	}
	return buf.Bytes(), nil
}
