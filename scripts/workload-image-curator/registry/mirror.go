package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/acm-deploy-load/workload-image-curator/models"
	"golang.org/x/sync/errgroup"
)

// MirrorInspector inspects a target registry to find already-mirrored images
type MirrorInspector struct {
	registry  string
	inspector *Inspector
	timeout   int
}

// NewMirrorInspector creates a new mirror registry inspector
func NewMirrorInspector(registry string, timeoutSecs int) *MirrorInspector {
	return &MirrorInspector{
		registry:  registry,
		inspector: NewInspector(timeoutSecs),
		timeout:   timeoutSecs,
	}
}

// ExistingImage represents an image already present in the target registry
type ExistingImage struct {
	Image      *models.OperatorImage
	Size       int64
	LayerCount int
}

// FindExistingImages queries the target registry for images matching a given digest list
// Returns a map of digest -> ExistingImage for images that exist in the target registry
func (m *MirrorInspector) FindExistingImages(ctx context.Context, images []*models.OperatorImage) (map[string]*ExistingImage, error) {
	existing := make(map[string]*ExistingImage)
	existingMu := sync.Mutex{}

	// Query target registry for each image in parallel
	var g errgroup.Group
	g.SetLimit(10) // 10 concurrent inspections

	for _, img := range images {
		image := img // Capture for closure

		g.Go(func() error {
			// Construct target image reference (digest-based)
			// For images without org, we assume they're in the destination org
			targetRef := fmt.Sprintf("%s/%s@%s", m.registry, extractImagePath(image.QuayImage), image.ShaDigest)

			// Try to fetch manifest from target registry
			metadata, err := m.inspector.ExtractSize(ctx, targetRef)
			if err != nil {
				// Image doesn't exist in target registry, which is fine
				return nil
			}

			// Image exists in target registry
			existingMu.Lock()
			existing[image.ShaDigest] = &ExistingImage{
				Image:      image,
				Size:       metadata.TotalSize,
				LayerCount: metadata.LayerCount,
			}
			existingMu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return existing, nil
}

// extractImagePath extracts the repo path from a pull spec
// e.g., "quay.io/openshift/operator@sha256:abc" -> "openshift/operator"
func extractImagePath(pullSpec string) string {
	// Remove digest part
	if idx := strings.Index(pullSpec, "@"); idx > 0 {
		pullSpec = pullSpec[:idx]
	}

	// Remove registry and leading slash
	parts := strings.Split(pullSpec, "/")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "/")
	}
	return parts[0]
}
