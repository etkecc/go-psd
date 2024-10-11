package psd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Helper function to create mock data
func createMockData(t *testing.T) []byte {
	t.Helper()
	targets := []*Item{
		{Targets: []string{"target1"}, Labels: map[string]string{"domain": "example.com"}},
		{Targets: []string{"target2"}, Labels: map[string]string{"domain": "example.net"}},
	}
	data, _ := json.Marshal(targets)
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
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write(mockData) //nolint:errcheck // test
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
