package proxy

import (
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	httpClient  *http.Client
	upstreamURL string
}

func NewClient(upstreamURL string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		upstreamURL: upstreamURL,
	}
}

// Forward proxies a request to the Kubeflow upstream and returns the response.
func (c *Client) Forward(method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.upstreamURL, path)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	// Copy headers
	for key, values := range headers {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	log.WithFields(log.Fields{
		"method": method,
		"url":    url,
	}).Debug("forwarding request to upstream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	return resp, nil
}
