package psd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	// DefaultUserAgent is the default user agent used by the client
	DefaultUserAgent = "Go-PSD-client/v0.0.0"
	// DefaultMaxRetries is the default number of retries for failed requests
	DefaultMaxRetries = 3
)

// Client is the client to interact with the Prometheus Service Discovery HTTP API
type Client struct {
	url        *url.URL
	cache      *expirable.LRU[string, cacheValue]
	client     *http.Client
	login      string
	password   string
	userAgent  string
	maxRetries int
}

type cacheValue struct {
	data     []byte
	etag     string
	cachedAt string
}

// NewClient returns a new PSD client
func NewClient(baseURL, login, password string, userAgentOverride ...string) *Client {
	uri, err := url.Parse(baseURL)
	if err != nil || login == "" || password == "" {
		return &Client{client: http.DefaultClient}
	}

	cache := expirable.NewLRU[string, cacheValue](1000, nil, 5*time.Minute)
	userAgent := DefaultUserAgent
	if len(userAgentOverride) > 0 {
		userAgent = userAgentOverride[0]
	}

	return &Client{
		url:        uri,
		cache:      cache,
		client:     http.DefaultClient,
		login:      login,
		password:   password,
		userAgent:  userAgent,
		maxRetries: DefaultMaxRetries,
	}
}

// do executes an HTTP request with per-attempt timeout and retry handling.
func (p *Client) do(ctx context.Context, makeReq func(context.Context) (*http.Request, error), currentAttempt ...int) (*http.Response, error) {
	attempt := 0
	if len(currentAttempt) > 0 {
		attempt = currentAttempt[0]
	}

	childCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := makeReq(childCtx)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req) //nolint:gosec // The URL is built from a trusted source
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil
		}
		return nil, err
	}

	if resp.StatusCode == http.StatusGone { // not found, to distinguish from reverse proxy 404 error
		defer resp.Body.Close()
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
		defer resp.Body.Close()
		if attempt < p.maxRetries {
			time.Sleep(1 * time.Second * time.Duration(attempt+1)) // Exponential backoff
			return p.do(ctx, makeReq, attempt+1)
		}
		err = fmt.Errorf("HTTP error: %s", resp.Status)
		return nil, err
	}

	return resp, nil
}

// GetWithContext returns the list of targets for the given identifier using the given context
func (p *Client) GetWithContext(ctx context.Context, identifier string, jobOverride ...string) (Items, error) {
	if p.url == nil {
		return nil, nil
	}
	cloned := *p.url
	job := "node"
	if len(jobOverride) > 0 {
		job = jobOverride[0]
	}
	uri := cloned.JoinPath("/" + job + "/" + identifier)
	urlTarget := uri.String()
	cachedData, cached := p.cache.Get(urlTarget)

	headers := http.Header{}
	if cached {
		headers.Set("If-Modified-Since", cachedData.cachedAt)
		headers.Set("If-None-Match", cachedData.etag)
	}

	headers.Set("User-Agent", p.userAgent)
	makeReq := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlTarget, http.NoBody)
		if err != nil {
			return nil, err
		}
		req.Header = headers.Clone()
		req.SetBasicAuth(p.login, p.password)
		return req, nil
	}
	resp, err := p.do(ctx, makeReq)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, nil
	}

	defer resp.Body.Close()
	var datab []byte
	if resp.StatusCode == http.StatusNotModified && cached {
		datab = cachedData.data
	}

	if datab == nil {
		datab, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		lastModified := resp.Header.Get("Last-Modified")
		if lastModified == "" {
			lastModified = time.Now().Format(http.TimeFormat)
		}
		p.cache.Add(urlTarget, cacheValue{
			data:     datab,
			etag:     resp.Header.Get("ETag"),
			cachedAt: lastModified,
		})
	}
	var psd []*Item
	err = json.Unmarshal(datab, &psd)
	if err != nil {
		return nil, err
	}

	return psd, nil
}

// Get returns the list of targets for the given identifier
func (p *Client) Get(identifier string) (Items, error) {
	return p.GetWithContext(context.Background(), identifier)
}

// GetRaw returns the raw data for the given identifier, without using the cache or parsing the response.
// Do not use this method unless you need the raw data, as it does not parse the response
func (p *Client) GetRaw(ctx context.Context, identifier string, jobOverride ...string) ([]byte, error) {
	if p.url == nil {
		return nil, nil
	}
	cloned := *p.url
	job := "node"
	if len(jobOverride) > 0 {
		job = jobOverride[0]
	}
	uri := cloned.JoinPath("/" + job + "/" + identifier)
	urlTarget := uri.String()

	headers := http.Header{}
	headers.Set("User-Agent", p.userAgent)
	makeReq := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlTarget, http.NoBody)
		if err != nil {
			return nil, err
		}
		req.Header = headers.Clone()
		req.SetBasicAuth(p.login, p.password)
		return req, nil
	}
	resp, err := p.do(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
