package authcallback

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestCallbackServer_SuccessfulCallback(t *testing.T) {
	srv, err := NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	defer srv.Close()

	url := srv.URL()
	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	// Simulate IdP callback in background.
	errCh := make(chan error, 1)
	go func() {
		resp, err := http.Get(url + "?code=abc123&state=xyz")
		if err != nil {
			errCh <- err
			return
		}
		resp.Body.Close()
		errCh <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := srv.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if result.Params["code"] != "abc123" {
		t.Errorf("expected code=abc123, got %q", result.Params["code"])
	}
	if result.Params["state"] != "xyz" {
		t.Errorf("expected state=xyz, got %q", result.Params["state"])
	}

	// Wait for the HTTP request goroutine to finish before test exits.
	if err := <-errCh; err != nil {
		t.Errorf("GET callback: %v", err)
	}
}

func TestCallbackServer_ContextCancellation(t *testing.T) {
	srv, err := NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = srv.Wait(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestCallbackServer_CloseBeforeCallback(t *testing.T) {
	srv, err := NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}

	url := srv.URL()
	srv.Close()

	// Requests after close should fail.
	_, err = http.Get(url + "?code=test")
	if err == nil {
		t.Fatal("expected error after server close")
	}
}

func TestCallbackServer_BindsToLocalhost(t *testing.T) {
	srv, err := NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	defer srv.Close()

	tcpAddr, ok := srv.listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected *net.TCPAddr, got %T", srv.listener.Addr())
	}
	if !tcpAddr.IP.IsLoopback() {
		t.Errorf("expected loopback address, got %s", tcpAddr.IP)
	}
}
