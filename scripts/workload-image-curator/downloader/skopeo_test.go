package downloader

import (
	"testing"
)

func TestNewSkopeoRunner(t *testing.T) {
	runner := NewSkopeoRunner(false)
	if runner == nil {
		t.Error("NewSkopeoRunner returned nil")
	}
	if runner.dryRun != false {
		t.Errorf("Expected dryRun=false, got %v", runner.dryRun)
	}
}

func TestSkopeoRunner_DryRun(t *testing.T) {
	runner := NewSkopeoRunner(true)

	// Dry-run should always succeed without executing skopeo
	err := runner.Copy("source:latest", "dest:latest")
	if err != nil {
		t.Errorf("Expected dry-run to succeed, got error: %v", err)
	}
}

func TestSkopeoRunner_IsAvailable(t *testing.T) {
	// Just test that the function doesn't panic
	available := IsAvailable()
	t.Logf("skopeo available: %v", available)
	// Don't assert, as skopeo may not be installed in test environment
}

func TestSkopeoRunner_InvalidSource(t *testing.T) {
	if !IsAvailable() {
		t.Skip("skopeo not available, skipping integration test")
	}

	runner := NewSkopeoRunner(false)

	// Try to copy from non-existent image (will fail)
	err := runner.Copy("nonexistent.registry.com/nonexistent:latest", "local:test")
	if err == nil {
		t.Error("Expected error for non-existent image, got nil")
	}
	t.Logf("Got expected error: %v", err)
}
