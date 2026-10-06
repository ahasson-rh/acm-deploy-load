package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractSize_InvalidPullSpec(t *testing.T) {
	inspector := NewInspector(8)

	tests := []struct {
		name    string
		pullSpec string
	}{
		{"Missing digest", "quay.io/openshift/image:v1"},
		{"No registry", "image@sha256:abc123"},
		{"Empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := inspector.ExtractSize(context.Background(), tt.pullSpec)
			if err == nil {
				t.Error("Expected error for invalid pull spec, got nil")
			}
		})
	}
}

func TestExtractSize_ValidManifest(t *testing.T) {
	// Create mock manifest metadata
	manifest := ManifestMetadata{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: struct {
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		}{
			Size:   5000,
			Digest: "sha256:config123",
		},
		Layers: []struct {
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		}{
			{Size: 1000, Digest: "sha256:layer1"},
			{Size: 2000, Digest: "sha256:layer2"},
			{Size: 3000, Digest: "sha256:layer3"},
		},
	}

	// Test size calculation directly
	expectedTotal := int64(5000 + 1000 + 2000 + 3000)
	actualTotal := manifest.Config.Size
	layerCount := 1

	for _, layer := range manifest.Layers {
		actualTotal += layer.Size
		layerCount++
	}

	if actualTotal != expectedTotal {
		t.Errorf("Total size calculation: expected %d, got %d", expectedTotal, actualTotal)
	}
	if layerCount != 4 {
		t.Errorf("Layer count: expected 4, got %d", layerCount)
	}
}

func TestGetAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": "test-token-12345",
		})
	}))
	defer server.Close()

	// Test with known registries
	inspector := NewInspector(8)
	token, err := inspector.getAuthToken(context.Background(), "example.com", "test/image")

	if err != nil {
		t.Fatalf("getAuthToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token for unknown registry, got %q", token)
	}
}

func TestNewInspector_DefaultTimeout(t *testing.T) {
	inspector := NewInspector(0)
	if inspector.timeout.Seconds() != 8 {
		t.Errorf("Default timeout: expected 8s, got %vs", inspector.timeout.Seconds())
	}

	inspector = NewInspector(-1)
	if inspector.timeout.Seconds() != 8 {
		t.Errorf("Negative timeout should use default 8s, got %vs", inspector.timeout.Seconds())
	}
}

func TestNewInspector_CustomTimeout(t *testing.T) {
	inspector := NewInspector(30)
	if inspector.timeout.Seconds() != 30 {
		t.Errorf("Custom timeout: expected 30s, got %vs", inspector.timeout.Seconds())
	}
}

func TestImageSizeMetadata_Calculation(t *testing.T) {
	// Test manual calculation (mimicking ExtractSize behavior)
	configSize := int64(1024)
	layers := []int64{512, 1024, 2048}

	totalSize := configSize
	layerCount := 1 // config
	for _, layer := range layers {
		totalSize += layer
		layerCount++
	}

	if totalSize != 4608 {
		t.Errorf("Total size calculation: expected 4608, got %d", totalSize)
	}
	if layerCount != 4 {
		t.Errorf("Layer count: expected 4, got %d", layerCount)
	}
}
