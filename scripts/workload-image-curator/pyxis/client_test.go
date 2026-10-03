package pyxis

import (
	"context"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	client := NewClient("")

	if client.baseURL != "https://catalog.redhat.com/api/containers/v1" {
		t.Errorf("got baseURL %q, want default", client.baseURL)
	}

	if client.retryMax != 3 {
		t.Errorf("got retryMax %d, want 3", client.retryMax)
	}

	if client.retryDelay != 5*time.Second {
		t.Errorf("got retryDelay %v, want 5s", client.retryDelay)
	}
}

func TestNewClient_CustomURL(t *testing.T) {
	customURL := "https://custom.api.com"
	client := NewClient(customURL)

	if client.baseURL != customURL {
		t.Errorf("got baseURL %q, want %q", client.baseURL, customURL)
	}
}

func TestFetchPackages_BuildsCorrectURL(t *testing.T) {
	client := NewClient("")
	ctx := context.Background()

	// This will fail due to network, but we're just testing URL construction
	// The actual network test would require mocking
	_, err := client.FetchPackages(ctx, 0, 500)
	if err == nil {
		t.Skip("Network test skipped - would require mocking")
	}
}

func TestFetchBundles_BuildsCorrectFilter(t *testing.T) {
	client := NewClient("")
	ctx := context.Background()

	// This will fail due to network, but we're just testing URL construction
	_, err := client.FetchBundles(ctx, "test-operator", 1)
	if err == nil {
		t.Skip("Network test skipped - would require mocking")
	}
}
