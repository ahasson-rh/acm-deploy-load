package pyxis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a Pyxis API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	retryMax   int
	retryDelay time.Duration
}

// NewClient creates a new Pyxis API client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://catalog.redhat.com/api/containers/v1"
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryMax:   3,
		retryDelay: 5 * time.Second,
	}
}

// FetchPackages fetches operator packages from Pyxis API
func (c *Client) FetchPackages(ctx context.Context, page int, pageSize int) (*PackageResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	url := fmt.Sprintf("%s/operators/packages?%s", c.baseURL, params.Encode())
	return c.doWithRetry(ctx, url, &PackageResponse{})
}

// FetchBundles fetches bundles for an operator package
func (c *Client) FetchBundles(ctx context.Context, packageName string, pageSize int) (*BundleResponse, error) {
	params := url.Values{}
	params.Set("filter", fmt.Sprintf(`package=="%s"`, packageName))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	url := fmt.Sprintf("%s/operators/bundles?%s", c.baseURL, params.Encode())
	return c.doWithRetry(ctx, url, &BundleResponse{})
}

// doWithRetry executes a request with exponential backoff retry logic
func (c *Client) doWithRetry(ctx context.Context, url string, v interface{}) (interface{}, error) {
	var lastErr error
	backoff := c.retryDelay

	for attempt := 0; attempt < c.retryMax; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
				backoff = time.Duration(float64(backoff) * 2.0)
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		err = json.Unmarshal(body, v)
		if err != nil {
			lastErr = err
			continue
		}

		return v, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
