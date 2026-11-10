package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a small JSON HTTP client used by tests and tooling.
type Client struct {
	HTTP     *http.Client
	BaseURL  string
	Identity string
}

// Response is a captured HTTP exchange.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Request    *http.Request
}

// New creates a client with a default timeout.
func New(baseURL string) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
}

// DoJSON performs an HTTP request with optional JSON body and extra headers.
func (c *Client) DoJSON(method, path string, headers map[string]string, body any) (*Response, error) {
	var reader io.Reader
	var rawBody []byte
	if body != nil {
		var err error
		rawBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(rawBody)
	}

	url := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		if path == "" {
			url = c.BaseURL
		} else {
			url = c.BaseURL + "/" + strings.TrimLeft(path, "/")
		}
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Identity != "" {
		req.Header.Set("X-Playground-Identity", c.Identity)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       respBody,
		Request:    req,
	}, nil
}

// Decode unmarshals the response body into dest.
func (r *Response) Decode(dest any) error {
	return json.Unmarshal(r.Body, dest)
}

// PrettyBody returns indented JSON when possible.
func (r *Response) PrettyBody() string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, r.Body, "", "  "); err != nil {
		return string(r.Body)
	}
	return buf.String()
}

// Diagnostics formats request/response for failing assertions.
func (r *Response) Diagnostics() string {
	var b strings.Builder
	b.WriteString("\n========== API DIAGNOSTICS ==========\n")
	if r.Request != nil {
		fmt.Fprintf(&b, "REQUEST: %s %s\n", r.Request.Method, r.Request.URL.String())
		for k, vals := range r.Request.Header {
			v := strings.Join(vals, ", ")
			if strings.EqualFold(k, "Authorization") && len(v) > 20 {
				v = v[:20] + "…"
			}
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	fmt.Fprintf(&b, "RESPONSE: HTTP %d\n", r.StatusCode)
	fmt.Fprintf(&b, "  body: %s\n", r.PrettyBody())
	b.WriteString("=====================================\n")
	return b.String()
}
