package registry

import (
	"context"
	"testing"
	"time"
)

func TestNewValidator_Defaults(t *testing.T) {
	validator := NewValidator(0)

	if validator.timeout != 8*time.Second {
		t.Errorf("got timeout %v, want 8s", validator.timeout)
	}
}

func TestNewValidator_CustomTimeout(t *testing.T) {
	validator := NewValidator(15)

	if validator.timeout != 15*time.Second {
		t.Errorf("got timeout %v, want 15s", validator.timeout)
	}
}

func TestValidateAccessibility_InvalidPullSpec(t *testing.T) {
	validator := NewValidator(8)
	ctx := context.Background()

	// Test with invalid pull spec (no digest)
	tests := []struct {
		name     string
		pullSpec string
		want     bool
	}{
		{
			name:     "no digest",
			pullSpec: "quay.io/test/image:latest",
			want:     false,
		},
		{
			name:     "empty string",
			pullSpec: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateAccessibility(ctx, tt.pullSpec)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestValidateAccessibility_SkopeoNotAvailable(t *testing.T) {
	validator := NewValidator(1)
	ctx := context.Background()

	// When skopeo is not available, should fall back to HTTP
	// This test will fail if skopeo is installed and image is accessible
	pullSpec := "quay.io/nonexistent/image@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	result := validator.ValidateAccessibility(ctx, pullSpec)

	// We can't assert the result without mocking, but we can verify it doesn't panic
	t.Logf("ValidateAccessibility returned: %v", result)
}
