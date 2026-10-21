package crawler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClient() *Client {
	return New(Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    2 * time.Second,
		MaxSize:    1 << 20,
		MaxRetries: 1,
		Logger:     testLogger(),
	})
}

func TestFetchReturns404ForSpiderPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	response, err := testClient().Fetch(context.Background(), server.URL+"/missing")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusNotFound {
		t.Errorf("status = %d", response.Status)
	}
}

func TestFetchRetriesTransientStatus(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	response, err := testClient().Fetch(context.Background(), server.URL+"/retry")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusOK || string(response.Body) != "ok" {
		t.Errorf("response = %d %q", response.Status, response.Body)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestFetchReportsPersistentBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Just a moment"))
	}))
	defer server.Close()

	_, err := testClient().Fetch(context.Background(), server.URL+"/blocked")
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("error = %T %v, want *BlockedError", err, err)
	}
}

func TestFetchHonorsRobots(t *testing.T) {
	var pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		pageCalls.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	_, err := testClient().Fetch(context.Background(), server.URL+"/private")
	if !errors.Is(err, ErrRobotsDisallowed) {
		t.Fatalf("error = %v, want ErrRobotsDisallowed", err)
	}
	if pageCalls.Load() != 0 {
		t.Errorf("se solicitó una URL prohibida %d veces", pageCalls.Load())
	}
}
