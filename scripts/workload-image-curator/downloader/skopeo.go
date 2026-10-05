package downloader

import (
	"fmt"
	"os/exec"
	"strings"
)

// SkopeoRunner wraps skopeo copy operations
type SkopeoRunner struct {
	dryRun bool
}

// NewSkopeoRunner creates a new skopeo wrapper
func NewSkopeoRunner(dryRun bool) *SkopeoRunner {
	return &SkopeoRunner{dryRun: dryRun}
}

// Copy mirrors an image from source to destination
func (s *SkopeoRunner) Copy(sourceImage, destImage string) error {
	if s.dryRun {
		return nil // Skip actual copy in dry-run mode
	}

	cmd := exec.Command("skopeo", "copy",
		fmt.Sprintf("docker://%s", sourceImage),
		fmt.Sprintf("docker://%s", destImage),
	)

	// Capture output for error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse skopeo error for better messaging
		errMsg := string(output)
		if strings.Contains(errMsg, "image not found") {
			return fmt.Errorf("source image not found: %s", sourceImage)
		}
		if strings.Contains(errMsg, "permission denied") {
			return fmt.Errorf("permission denied accessing registries")
		}
		return fmt.Errorf("skopeo copy failed: %w (output: %s)", err, errMsg)
	}

	return nil
}

// IsAvailable checks if skopeo is available in PATH
func IsAvailable() bool {
	_, err := exec.LookPath("skopeo")
	return err == nil
}
