package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestServeDrainsRequestsBeforeReturning(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release, shuttingDown := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	t.Cleanup(finish)
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "completed")
	})}
	server.RegisterOnShutdown(func() { close(shuttingDown) })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- serve(ctx, server, listener) }()
	response := make(chan error, 1)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		res, requestErr := client.Do(request)
		if requestErr == nil {
			defer func() { _ = res.Body.Close() }()
			var body []byte
			body, requestErr = io.ReadAll(res.Body)
			if requestErr == nil && string(body) != "completed" {
				requestErr = io.ErrUnexpectedEOF
			}
		}
		response <- requestErr
	}()
	<-started
	cancel()
	<-shuttingDown
	select {
	case err = <-served:
		t.Fatalf("serve returned before the active request completed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	finish()
	if err = <-response; err != nil {
		t.Fatal(err)
	}
	if err = <-served; err != nil {
		t.Fatal(err)
	}
}
