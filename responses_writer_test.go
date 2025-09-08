package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to build logger writing into a buffer
func testLogger(buf io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestResponsesWriter_FileAndJSONHeader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fname := filepath.Join(dir, "data.json")
	content := `{"msg":"ok"}`
	if err := os.WriteFile(fname, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	resp := Response{File: fname, Code: 201, IsJSON: true, Headers: http.Header{"X-Test": {"yes"}}}
	rec := httptest.NewRecorder()
	rw := responsesWriter([]Response{resp}, testLogger(io.Discard))
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rw(rec, r)

	if rec.Code != 201 {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json, got %q", got)
	}
	if got := rec.Header().Get("X-Test"); got != "yes" {
		t.Fatalf("expected custom header, got %q", got)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != content {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestResponsesWriter_RepeatLogic(t *testing.T) {
	t.Parallel()

	repeat := 2
	responses := []Response{{Body: "first", Code: 200, Repeat: &repeat}, {Body: "second", Code: 202}}
	rw := responsesWriter(responses, testLogger(io.Discard))

	// first call
	rec1 := httptest.NewRecorder()
	rw(rec1, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec1.Code != 200 || strings.TrimSpace(rec1.Body.String()) != "first" {
		t.Fatalf("first call wrong: %d %q", rec1.Code, rec1.Body.String())
	}
	// second call should still be first response
	rec2 := httptest.NewRecorder()
	rw(rec2, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec2.Code != 200 || strings.TrimSpace(rec2.Body.String()) != "first" {
		t.Fatalf("second call wrong: %d %q", rec2.Code, rec2.Body.String())
	}
	// third call should switch to second response (repeat exhausted)
	rec3 := httptest.NewRecorder()
	rw(rec3, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec3.Code != 202 || strings.TrimSpace(rec3.Body.String()) != "second" {
		t.Fatalf("third call wrong: %d %q", rec3.Code, rec3.Body.String())
	}
}

func TestResponsesWriter_NotFoundWhenExhausted(t *testing.T) {
	t.Parallel()

	rw := responsesWriter([]Response{}, testLogger(io.Discard))
	rec := httptest.NewRecorder()
	rw(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestResponsesWriter_FileReadError(t *testing.T) {
	t.Parallel()

	responses := []Response{{File: "no_such_file", Code: 200}}
	rec := httptest.NewRecorder()
	rw := responsesWriter(responses, testLogger(io.Discard))
	rw(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no_such_file") {
		t.Fatalf("expected error message, got %q", rec.Body.String())
	}
}

func TestResponsesWriter_JSONDoesNotOverrideExistingContentType(t *testing.T) {
	t.Parallel()

	responses := []Response{{Body: "{}", Code: 200, IsJSON: true, Headers: http.Header{"Content-Type": {"text/plain"}}}}
	rec := httptest.NewRecorder()
	rw := responsesWriter(responses, testLogger(io.Discard))
	rw(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if got := rec.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected text/plain kept, got %q", got)
	}
}

func TestExecuteTemplate_Success(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/foo/bar?x=1", http.NoBody)
	got, err := executeTemplate("{{.Method}} {{.URL.Path}}", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "POST /foo/bar" {
		t.Fatalf("unexpected template result %q", string(got))
	}
}

func TestExecuteTemplate_ParseError(t *testing.T) {
	t.Parallel()

	_, err := executeTemplate("{{", httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestStructuredLogger_BasicFields(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := testLogger(&buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handled", "1")
		w.WriteHeader(http.StatusNoContent)
	})

	h := StructuredLogger(logger, "X-Request-ID", next)
	rec := httptest.NewRecorder()
	// Use relative URL so that when we manually set Host, the constructed URI is correct.
	// If we used an absolute URL, Host would be duplicated in the URI
	// (e.g., "http://example.comhttp://example.com/test?q=1").
	req := httptest.NewRequest(http.MethodGet, "/test?q=1", http.NoBody)
	req.Host = "example.com"
	req.Header.Set("X-Request-ID", "abc-123")
	h(rec, req)

	if rec.Code != 204 {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// parse last JSON line
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	last := lines[len(lines)-1]
	var obj map[string]any
	if err := json.Unmarshal([]byte(last), &obj); err != nil {
		t.Fatalf("unmarshal log: %v: %s", err, last)
	}
	if obj["resp_status"].(float64) != 204 {
		t.Fatalf("expected status 204 in log, got %v", obj["resp_status"])
	}
	if obj["resp_byte_length"].(float64) != 0 {
		t.Fatalf("expected byte length 0, got %v", obj["resp_byte_length"])
	}
	if obj["request_id"] != "abc-123" {
		t.Fatalf("expected request id, got %v", obj["request_id"])
	}
	if obj["uri"].(string) != "http://example.com/test?q=1" {
		t.Fatalf("unexpected uri %v", obj["uri"])
	}
}

func TestResponsesWriter_JSONBodySetsContentType(t *testing.T) {
	t.Parallel()

	responses := []Response{{Body: "{\"k\":1}", Code: 200, IsJSON: true}}
	rec := httptest.NewRecorder()
	rw := responsesWriter(responses, testLogger(io.Discard))
	rw(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestResponsesWriter_RepeatZeroSkips(t *testing.T) {
	t.Parallel()

	zero := 0
	responses := []Response{{Body: "skip", Code: 200, Repeat: &zero}, {Body: "use", Code: 201}}
	rec := httptest.NewRecorder()
	rw := responsesWriter(responses, testLogger(io.Discard))
	rw(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if rec.Code != 201 || strings.TrimSpace(rec.Body.String()) != "use" {
		t.Fatalf("expected second response, got %d %q", rec.Code, rec.Body.String())
	}
}
