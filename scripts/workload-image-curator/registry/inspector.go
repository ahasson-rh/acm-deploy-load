package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Inspector extracts size metadata from registry manifests
type Inspector struct {
	timeout time.Duration
}

// NewInspector creates a new image inspector
func NewInspector(timeoutSecs int) *Inspector {
	if timeoutSecs <= 0 {
		timeoutSecs = 8
	}
	return &Inspector{
		timeout: time.Duration(timeoutSecs) * time.Second,
	}
}

// ManifestMetadata contains Docker manifest v2 metadata
type ManifestMetadata struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"layers"`
}

// ImageSizeMetadata holds extracted size information
type ImageSizeMetadata struct {
	TotalSize  int64 // Total manifest size in bytes
	LayerCount int   // Number of layers (config + layers)
}

// ExtractSize retrieves size metadata from a Docker image manifest
// For Docker v2 manifests: sums config size + all layer sizes
func (i *Inspector) ExtractSize(ctx context.Context, pullSpec string) (*ImageSizeMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	// Parse pull spec: registry/repo/image@sha256:digest or registry/repo/image:tag
	parts := strings.Split(pullSpec, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid pull spec format: %s (must include @sha256:digest)", pullSpec)
	}

	imagePart := parts[0]
	digest := parts[1]

	// Extract registry and repo path
	registryParts := strings.Split(imagePart, "/")
	if len(registryParts) < 2 {
		return nil, fmt.Errorf("invalid image format: %s", imagePart)
	}

	registry := registryParts[0]
	repoPath := strings.Join(registryParts[1:], "/")

	// Fetch manifest using Docker v2 API
	manifest, err := i.fetchManifest(ctx, registry, repoPath, digest)
	if err != nil {
		return nil, err
	}

	if manifest == nil {
		return &ImageSizeMetadata{
			TotalSize:  0,
			LayerCount: 0,
		}, nil
	}

	// Calculate total size
	totalSize := manifest.Config.Size
	layerCount := 1 // Config is always present

	for _, layer := range manifest.Layers {
		totalSize += layer.Size
		layerCount++
	}

	return &ImageSizeMetadata{
		TotalSize:  totalSize,
		LayerCount: layerCount,
	}, nil
}

// fetchManifest retrieves Docker v2 manifest from registry
func (i *Inspector) fetchManifest(ctx context.Context, registry, repoPath, digest string) (*ManifestMetadata, error) {
	// Construct manifest URL
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repoPath, digest)

	// Get auth token for private registries
	var authHeader string
	if registry == "quay.io" || registry == "registry.redhat.io" {
		token, err := i.getAuthToken(ctx, registry, repoPath)
		if err == nil && token != "" {
			authHeader = fmt.Sprintf("Bearer %s", token)
		}
	}

	// Fetch manifest
	req, _ := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")
	req.Header.Set("User-Agent", "workload-image-curator/1.0")

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: i.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("manifest fetch failed: HTTP %d", resp.StatusCode)
	}

	var manifest ManifestMetadata
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// getAuthToken fetches bearer token from registry auth service
func (i *Inspector) getAuthToken(ctx context.Context, registry, repoPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	// Use registry-specific auth endpoint
	var authURL string
	switch registry {
	case "quay.io":
		authURL = fmt.Sprintf("https://quay.io/v2/auth?service=quay.io&scope=repository:%s:pull", repoPath)
	case "registry.redhat.io":
		authURL = fmt.Sprintf("https://registry.redhat.io/v2/auth?service=registry.redhat.io&scope=repository:%s:pull", repoPath)
	default:
		// For other registries, skip auth
		return "", nil
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", authURL, nil)
	req.Header.Set("User-Agent", "workload-image-curator/1.0")

	client := &http.Client{Timeout: i.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("auth failed: HTTP %d", resp.StatusCode)
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	return authResp.Token, nil
}
