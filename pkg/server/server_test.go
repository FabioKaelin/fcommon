package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestGracefulShutdownDrainsRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release; w.Write([]byte("done")) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(listener.Addr().String(), handler)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, server, listener) }()
	response := make(chan error, 1)
	go func() {
		r, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			r.Body.Close()
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not enter")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("shutdown did not drain in-flight request")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if server.ReadHeaderTimeout <= 0 || server.MaxHeaderBytes > 32768 {
		t.Fatal("unbounded headers")
	}
}
