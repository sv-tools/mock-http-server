package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestRun_ServerIntegration starts the real HTTP server, exercises endpoints, then shuts it down.
func TestRun_ServerIntegration(t *testing.T) {
	t.Parallel()

	// Pick a free port
	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "tcp", ":0")
	if err != nil {
		t.Fatalf("listen :0: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	configContent := `routes:
  - pattern: /hello
    responses:
      - code: 200
        body: Hello
  - pattern: /json
    responses:
      - code: 201
        body: '{"ok":true}'
        is_json: true
`
	if err := os.WriteFile(cfgPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_, _, _ = run([]string{"-c", cfgPath, "-p", strconv.Itoa(port)}, map[string]string{}, io.Discard, true)
		close(done)
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	base := fmt.Sprintf("http://localhost:%d", port)

	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("server did not start in time on %s", base)
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/hello", http.NoBody)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_ = resp.Body.Close()
		break
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/hello", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/hello expected 200 got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/json", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET /json: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("/json expected 201 got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != JsonContentType {
		t.Fatalf("expected application/json got %q", ct)
	}
	_ = resp.Body.Close()

	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/ expected 404 got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Shutdown
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		panic("server failed to shut down")
	}
}
