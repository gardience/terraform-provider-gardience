package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the Gardience configuration API (/gardience/api/v1) with a configure key.
type Client struct {
	base string
	key  string
	http *http.Client
	ua   string
}

// APIError is a non-2xx answer from Gardience, with its explanation.
type APIError struct {
	Status int
	Detail string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Gardience API returned %d: %s", e.Status, e.Detail)
}

// IsNotFound reports whether err is a 404 from Gardience.
func IsNotFound(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.Status == http.StatusNotFound
	}
	return false
}

func NewClient(baseURL, apiKey, caFile string, insecure bool, version string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("url must look like https://gardience.example.com (got %q)", baseURL)
	}
	if u.Scheme != "https" && !insecure {
		return nil, fmt.Errorf("url must use https (set insecure = true only for a lab)")
	}
	if !strings.HasPrefix(apiKey, "gdn_") {
		return nil, fmt.Errorf("api_key must be a Gardience configure key (gdn_...), made by an admin in Settings > API keys")
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading ca_cert_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca_cert_file has no PEM certificates")
		}
		tlsCfg.RootCAs = pool
	}
	if insecure {
		tlsCfg.InsecureSkipVerify = true // #nosec G402 -- explicit opt-in for labs with a self-signed certificate
	}
	base := u.String()
	if !strings.HasSuffix(base, "/gardience/api/v1") {
		base += "/gardience/api/v1"
	}
	return &Client{base: base, key: apiKey, ua: "terraform-provider-gardience/" + version,
		http: &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg, Proxy: http.ProxyFromEnvironment}}}, nil
}

// Do sends a request; body (if not nil) is sent as JSON; out (if not nil) receives the JSON answer, or the raw text
// when out is a *string.
func (c *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("User-Agent", c.ua)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 5 {
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
			if body != nil {
				b, _ := json.Marshal(body)
				rd = bytes.NewReader(b)
			}
			continue
		}
		if resp.StatusCode >= 300 {
			var e struct {
				Detail any `json:"detail"`
			}
			detail := strings.TrimSpace(string(data))
			if json.Unmarshal(data, &e) == nil && e.Detail != nil {
				detail = fmt.Sprint(e.Detail)
			}
			return &APIError{Status: resp.StatusCode, Detail: detail}
		}
		if out == nil {
			return nil
		}
		if s, ok := out.(*string); ok {
			*s = string(data)
			return nil
		}
		return json.Unmarshal(data, out)
	}
}

// Wait polls check every interval until it reports done, it fails, or timeout passes.
func Wait(ctx context.Context, timeout, interval time.Duration, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		done, err := check()
		if err != nil || done {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
