package registry

import (
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

func TestExtractImagePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "quay.io/openshift/operator@sha256:abc123",
			expected: "openshift/operator",
		},
		{
			input:    "registry.redhat.io/ose-operator@sha256:def456",
			expected: "ose-operator",
		},
		{
			input:    "localhost:5000/myorg/image@sha256:xyz789",
			expected: "myorg/image",
		},
		{
			input:    "image@sha256:test",
			expected: "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractImagePath(tt.input)
			if result != tt.expected {
				t.Errorf("extractImagePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewMirrorInspector(t *testing.T) {
	inspector := NewMirrorInspector("localhost:5000", 10)
	if inspector == nil {
		t.Fatal("NewMirrorInspector returned nil")
	}
	if inspector.registry != "localhost:5000" {
		t.Errorf("registry = %q, want %q", inspector.registry, "localhost:5000")
	}
	if inspector.timeout != 10 {
		t.Errorf("timeout = %d, want 10", inspector.timeout)
	}
}

func TestFindExistingImages_Empty(t *testing.T) {
	inspector := NewMirrorInspector("localhost:5000", 5)
	images := make([]*models.OperatorImage, 0)

	// This will fail to connect (expected for unit test)
	// Just verify it returns empty map for empty input
	if len(images) == 0 {
		existing, _ := inspector.FindExistingImages(nil, images)
		if len(existing) != 0 {
			t.Errorf("FindExistingImages on empty input returned %d results, want 0", len(existing))
		}
	}
}
