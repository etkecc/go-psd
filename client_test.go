package psd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

// Helper function to create mock data
func createMockData(t *testing.T) []byte {
	t.Helper()
	targets := []*Item{
		{Targets: []string{"target1"}, Labels: map[string]string{"domain": "example.com"}},
		{Targets: []string{"target2"}, Labels: map[string]string{"domain": "example.net"}},
	}
	data, err := json.Marshal(targets)
	if err != nil {
		t.Fatalf("Failed to marshal mock data: %v", err)
	}
	return data
}

func createBenchmarkData(b *testing.B) []byte {
	b.Helper()
	const itemsCount = 50
	items := make([]*Item, 0, itemsCount)
	for i := 0; i < itemsCount; i++ {
		domain := fmt.Sprintf("example-%d.com", i)
		items = append(items, &Item{
			Targets: []string{
				domain + ":443",
				domain + ":8448",
				domain + "/.well-known/matrix/client",
				domain + "/_matrix/federation/v1/version",
			},
			Labels: map[string]string{
				"domain": domain,
				"site":   "bench",
				"index":  strconv.Itoa(i),
			},
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		b.Fatalf("Failed to marshal benchmark data: %v", err)
	}
	return data
}

// Test NewClient creation
func TestNewClient(t *testing.T) {
	client := NewClient("http://example.com", "user", "password")
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.url == nil || client.cache == nil {
		t.Fatal("Expected valid URL and cache in client")
	}
	if client.login != "user" || client.password != "password" {
		t.Fatal("Expected correct login and password")
	}
}

// Test GetWithContext with a valid response and cache miss
func TestGetWithContext_CacheMiss(t *testing.T) {
	mockData := createMockData(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("Expected User-Agent header to be set")
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "password" {
			t.Fatal("Expected Authorization header to be set with valid Basic Auth")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Expected Content-Type application/json, got %q", got)
		}
		if got := r.Header.Get("If-Modified-Since"); got != "" {
			t.Fatalf("Expected no If-Modified-Since on cache miss, got %q", got)
		}
		if got := r.Header.Get("If-None-Match"); got != "" {
			t.Fatalf("Expected no If-None-Match on cache miss, got %q", got)
		}
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write(mockData)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Perform the request
	targets, err := client.Get("test-id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(targets))
	}
	if targets[0].Targets[0] != "target1" || targets[1].Targets[0] != "target2" {
		t.Fatal("Unexpected target names")
	}
}

// Test GetWithContext with a valid response and cache hit
func TestGetWithContext_CacheHit(t *testing.T) {
	mockData := createMockData(t)
	etag := `"mock-etag"`
	lastModified := time.Now().Format(http.TimeFormat)

	// Create a test server that will return a 304 Not Modified
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("Expected User-Agent header to be set")
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "password" {
			t.Fatal("Expected Authorization header to be set with valid Basic Auth")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Expected Content-Type application/json, got %q", got)
		}
		if r.Header.Get("If-None-Match") == etag && r.Header.Get("If-Modified-Since") == lastModified {
			w.WriteHeader(http.StatusNotModified)
		} else {
			t.Fatal("Headers not set correctly for cached request")
		}
	}))
	defer ts.Close()

	// Create client and pre-populate cache
	client := NewClient(ts.URL, "user", "password")
	client.cache.Add(ts.URL+"/node/test-id", cacheValue{
		data:     mockData,
		etag:     etag,
		cachedAt: lastModified,
	})

	// Perform the request
	targets, err := client.Get("test-id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(targets))
	}
	if targets[0].Targets[0] != "target1" || targets[1].Targets[0] != "target2" {
		t.Fatal("Unexpected target names")
	}
}

// Test GetWithContext with a 404 error response
func TestGetWithContext_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")
	_, err := client.Get("test-id")
	if err != nil {
		t.Fatalf("Expected no error on 410 status, got %v", err)
	}
}

// Test GetWithContext with a server error
func TestGetWithContext_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")
	_, err := client.Get("test-id")
	if err == nil || !strings.Contains(err.Error(), "HTTP error: 500 Internal Server Error") {
		t.Fatalf("Expected HTTP 500 error, got %v", err)
	}
}

// Test invalid URL handling in NewClient
func TestNewClient_InvalidURL(t *testing.T) {
	client := NewClient("::://invalid-url", "user", "password")
	if client.url != nil {
		t.Fatal("Expected nil URL for invalid baseURL")
	}
}

func TestGetDomain(t *testing.T) {
	target := &Item{Targets: []string{"target1"}, Labels: map[string]string{"domain": "example.com"}}
	if target.GetDomain() != "example.com" {
		t.Fatal("Expected domain to be example.com")
	}
}

func TestGetWithContext_RetriesAndPreservesHeadersAndAuth(t *testing.T) {
	var calls int32
	failUntil := int32(2)
	userAgent := "test-ua"
	var headerErr atomic.Value

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

		if got := r.Header.Get("User-Agent"); got != userAgent {
			if headerErr.Load() == nil {
				headerErr.Store("User-Agent mismatch: got " + got + ", want " + userAgent)
			}
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			if headerErr.Load() == nil {
				headerErr.Store("Content-Type mismatch: got " + got + ", want application/json")
			}
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "password" {
			if headerErr.Load() == nil {
				headerErr.Store("BasicAuth mismatch: got (" + user + ", " + pass + "), ok=" + strconv.FormatBool(ok))
			}
		}

		if atomic.LoadInt32(&calls) <= failUntil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write(createMockData(t))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password", userAgent)
	client.maxRetries = int(failUntil)

	_, err := client.Get("test-id")
	if err != nil {
		t.Fatalf("Expected no error after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != failUntil+1 {
		t.Fatalf("Expected %d calls, got %d", failUntil+1, got)
	}
	if msg := headerErr.Load(); msg != nil {
		t.Fatal(msg)
	}
}

func TestGetWithContext_RetryLimitExceeded(t *testing.T) {
	var calls int32
	userAgent := "test-ua"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password", userAgent)
	client.maxRetries = 1

	_, err := client.Get("test-id")
	if err == nil || !strings.Contains(err.Error(), "HTTP error: 500 Internal Server Error") {
		t.Fatalf("Expected HTTP 500 error after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != int32(client.maxRetries)+1 {
		t.Fatalf("Expected %d calls, got %d", client.maxRetries+1, got)
	}
}

func TestGetWithContext_ContextCanceledReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("Expected User-Agent header to be set")
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "password" {
			t.Fatal("Expected Authorization header to be set with valid Basic Auth")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Expected Content-Type application/json, got %q", got)
		}
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write(createMockData(t))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetWithContext(ctx, "test-id")
	if err == nil {
		t.Fatal("Expected error for canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Expected context.Canceled, got %v", err)
	}
}

func TestGetWithContext_ConcurrentRequestsNoCancellation(t *testing.T) {
	mockData := createMockData(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("Expected User-Agent header to be set")
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "password" {
			t.Error("Expected Authorization header to be set with valid Basic Auth")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %q", got)
		}
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write(mockData)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")

	const total = 100
	var wg sync.WaitGroup
	errCh := make(chan error, total)

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "test-id-" + strconv.Itoa(i)
			_, err := client.GetWithContext(context.Background(), id)
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Expected no errors, got %v", err)
		}
	}
}

func BenchmarkGetWithContext_Parallel(b *testing.B) {
	payload := createBenchmarkData(b)
	lastModified := "Mon, 02 Jan 2006 15:04:05 GMT"
	etag := `"mock-etag"`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")

	var total uint64
	var errVal atomic.Value
	var errOnce sync.Once
	var failed atomic.Bool
	ctx := context.Background()
	id := "bench-id"

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	b.RunParallel(func(pb *testing.PB) {
		var local uint64
		for pb.Next() {
			if failed.Load() {
				break
			}
			_, err := client.GetWithContext(ctx, id)
			if err != nil {
				errOnce.Do(func() {
					errVal.Store(fmt.Errorf("request error: %T: %w", err, err))
				})
				failed.Store(true)
				break
			}
			local++
		}
		if local > 0 {
			atomic.AddUint64(&total, local)
		}
	})
	elapsed := time.Since(start)

	if err := errVal.Load(); err != nil {
		b.Fatalf("Benchmark failed: %v", err)
	}

	rps := float64(total) / elapsed.Seconds()
	b.ReportMetric(rps, "req/s")
}

func BenchmarkGetRaw_Parallel(b *testing.B) {
	payload := createBenchmarkData(b)
	lastModified := "Mon, 02 Jan 2006 15:04:05 GMT"
	etag := `"mock-etag"`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "user", "password")

	var total uint64
	var errVal atomic.Value
	var errOnce sync.Once
	var failed atomic.Bool
	ctx := context.Background()
	id := "bench-id"

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	b.RunParallel(func(pb *testing.PB) {
		var local uint64
		for pb.Next() {
			if failed.Load() {
				break
			}
			_, err := client.GetRaw(ctx, id)
			if err != nil {
				errOnce.Do(func() {
					errVal.Store(fmt.Errorf("request error: %T: %w", err, err))
				})
				failed.Store(true)
				break
			}
			local++
		}
		if local > 0 {
			atomic.AddUint64(&total, local)
		}
	})
	elapsed := time.Since(start)

	if err := errVal.Load(); err != nil {
		b.Fatalf("Benchmark failed: %v", err)
	}

	rps := float64(total) / elapsed.Seconds()
	b.ReportMetric(rps, "req/s")
}
