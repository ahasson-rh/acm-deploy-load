package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Validator validates image accessibility
type Validator struct {
	timeout time.Duration
}

// NewValidator creates a new image validator
func NewValidator(timeoutSecs int) *Validator {
	if timeoutSecs <= 0 {
		timeoutSecs = 8
	}
	return &Validator{
		timeout: time.Duration(timeoutSecs) * time.Second,
	}
}

// ValidateAccessibility checks if an image can be accessed
func (v *Validator) ValidateAccessibility(ctx context.Context, pullSpec string) bool {
	// Try skopeo first
	if v.validateWithSkopeo(ctx, pullSpec) {
		return true
	}

	// Fall back to HTTP check
	return v.validateWithHTTP(ctx, pullSpec)
}

// validateWithSkopeo uses skopeo inspect to validate accessibility
func (v *Validator) validateWithSkopeo(ctx context.Context, pullSpec string) bool {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "skopeo", "inspect", "--raw", fmt.Sprintf("docker://%s", pullSpec))
	err := cmd.Run()
	return err == nil
}

// validateWithHTTP validates image using Quay.io v2 API
func (v *Validator) validateWithHTTP(ctx context.Context, pullSpec string) bool {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Extract repo path and digest from pull spec
	// Format: quay.io/repo/image@sha256:digest
	parts := strings.Split(pullSpec, "@")
	if len(parts) != 2 {
		return false
	}

	imagePart := parts[0]
	digest := parts[1]

	// Extract registry and repo path
	registryParts := strings.Split(imagePart, "/")
	if len(registryParts) < 2 {
		return false
	}

	repoPath := strings.Join(registryParts[1:], "/")

	// Get auth token
	authURL := fmt.Sprintf("https://quay.io/v2/auth?service=quay.io&scope=repository:%s:pull", repoPath)
	req, _ := http.NewRequestWithContext(ctx, "GET", authURL, nil)
	req.Header.Set("User-Agent", "workload-image-curator/1.0")

	client := &http.Client{Timeout: v.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&authResp)

	if authResp.Token == "" {
		return false
	}

	// Check manifest availability
	manifestURL := fmt.Sprintf("https://quay.io/v2/%s/manifests/%s", repoPath, digest)
	req, _ = http.NewRequestWithContext(ctx, "HEAD", manifestURL, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authResp.Token))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")
	req.Header.Set("User-Agent", "workload-image-curator/1.0")

	resp, err = client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}
