package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/acm-deploy-load/workload-image-curator/config"
	"github.com/acm-deploy-load/workload-image-curator/models"
	"github.com/acm-deploy-load/workload-image-curator/output"
	"github.com/acm-deploy-load/workload-image-curator/pyxis"
	"github.com/acm-deploy-load/workload-image-curator/registry"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "workload-image-curator",
	Short: "Curate and mirror operator container images for ACS testing",
	Long: `Workload Image Curator fetches operator images from Red Hat Pyxis API,
validates accessibility, and mirrors them to a target registry.

Supports both random selection (-c COUNT) and size-based distribution (--strategy).`,
	RunE: runCmd,
}

func init() {
	// Image selection (mutually exclusive)
	rootCmd.Flags().IntP("count", "c", 50, "Total images for random selection (default: 50)")
	rootCmd.Flags().String("strategy", "", "Size distribution: small:10,medium:30,large:10")

	// Size thresholds
	rootCmd.Flags().Int64("size-small-threshold", 52428800, "Small/medium boundary in bytes (default: 50MB)")
	rootCmd.Flags().Int64("size-large-threshold", 209715200, "Medium/large boundary in bytes (default: 200MB)")

	// Registry & mirroring
	rootCmd.Flags().String("target-registry", "", "Target registry for mirroring")
	rootCmd.Flags().String("target-org", "", "Target organization/namespace")
	rootCmd.Flags().String("pull-secret", "/opt/registry/pull-secret-bastion.txt", "Pull secret file path")
	rootCmd.Flags().Bool("ignore-existing", false, "Force mirror all, skip pre-run assessment")
	rootCmd.Flags().Bool("dry-run", false, "Fetch metadata only, skip validation and mirroring")

	// Concurrency
	rootCmd.Flags().IntP("workers", "w", 5, "Concurrent download workers")
	rootCmd.Flags().Int("inspect-workers", 10, "Concurrent inspection workers")
	rootCmd.Flags().Float64("rate-limit", 10.0, "Max requests/sec collectively across all workers")

	// Validation
	rootCmd.Flags().Bool("skip-validation", false, "Skip image accessibility checks")
	rootCmd.Flags().Int("validation-timeout", 8, "Timeout for validation checks in seconds")
	rootCmd.Flags().BoolP("assume-yes", "y", false, "Assume yes to user prompts")

	// Output
	rootCmd.Flags().BoolP("stdout", "s", false, "Output pull specs to stdout")
	rootCmd.Flags().Bool("no-files", false, "Skip file output")
	rootCmd.Flags().String("output-json", "", "JSON output file path")
	rootCmd.Flags().String("output-txt", "", "TXT output file path")
	rootCmd.Flags().String("output-prefix", "deployable_operator_images", "Filename prefix for auto-generated names")

	// API
	rootCmd.Flags().String("source-registry-url", "", "Source registry API base URL (default: https://catalog.redhat.com/api/containers/v1)")

	// Logging
	rootCmd.Flags().BoolP("verbose", "v", false, "Verbose logging")
	rootCmd.Flags().BoolP("quiet", "q", false, "Suppress progress output")
}

func runCmd(cmd *cobra.Command, args []string) error {
	// Load configuration from flags
	cfg, err := config.LoadFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if !cfg.Quiet {
		logf("Starting Workload Image Curator")
		logf("Selection mode: %s", selectionModeName(cfg.Strategy))
		logf("Target count: %d", cfg.Strategy.Total())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Phase A: Image Selection
	images, err := selectImages(ctx, cfg)
	if err != nil {
		return fmt.Errorf("image selection failed: %w", err)
	}

	if !cfg.Quiet {
		logf("Selected %d images for processing", len(images))
	}

	if len(images) == 0 {
		return fmt.Errorf("no images selected")
	}

	// Output: Generate results
	formatter := output.NewFormatter(cfg.OutputPrefix)

	// Write stdout if requested
	if cfg.Stdout {
		if err := formatter.WriteStdout(images); err != nil {
			return fmt.Errorf("failed to write stdout: %w", err)
		}
	}

	// Write files if requested
	if !cfg.NoFiles {
		jsonFile := cfg.OutputJSON
		if jsonFile == "" {
			jsonFile = fmt.Sprintf("%s_%s.json", cfg.OutputPrefix, time.Now().Format("20060102_150405"))
		}
		txtFile := cfg.OutputTXT
		if txtFile == "" {
			txtFile = fmt.Sprintf("%s_%s.txt", cfg.OutputPrefix, time.Now().Format("20060102_150405"))
		}

		if err := formatter.WriteJSON(images, jsonFile); err != nil {
			return fmt.Errorf("failed to write JSON: %w", err)
		}
		if err := formatter.WriteTXT(images, txtFile); err != nil {
			return fmt.Errorf("failed to write TXT: %w", err)
		}
		if !cfg.Quiet {
			logf("Output files written: %s %s", jsonFile, txtFile)
		}
	}

	if !cfg.Quiet {
		logf("Phase 1: Image selection and validation completed")
	}

	return nil
}

// selectImages performs Phase A: fetches and validates images
func selectImages(ctx context.Context, cfg *config.Config) ([]*models.OperatorImage, error) {
	if !cfg.Quiet {
		logf("Fetching packages from source registry: %s", cfg.SourceRegistryURL)
	}

	// Create source registry client
	srcClient := pyxis.NewClient(cfg.SourceRegistryURL)

	// Fetch operator packages
	packages, err := fetchAllPackages(ctx, srcClient, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages: %w", err)
	}

	if !cfg.Quiet {
		logf("Found %d operator packages", len(packages))
	}

	// Fetch bundles and extract images
	// Phase 1: Stop early when we have enough images
	targetCount := cfg.Strategy.Total()
	allImages := make([]*models.OperatorImage, 0, targetCount*2)

	for i, pkg := range packages {
		// Early exit if we have enough images
		if len(allImages) >= targetCount*3 {
			break
		}

		if i%10 == 0 && !cfg.Quiet {
			logf("Fetching bundles for package %d/%d (have %d images so far)", i+1, len(packages), len(allImages))
		}

		bundles, err := srcClient.FetchBundles(ctx, pkg.Name, 10)
		if err != nil {
			if !cfg.Quiet {
				logf("Warning: Failed to fetch bundles for %s: %v", pkg.Name, err)
			}
			continue
		}

		for _, bundle := range bundles.Data {
			// Use bundle_path as the image (main bundle image)
			if bundle.BundlePath != "" {
				// Generate deterministic mock size for categorization
				// Phase 2 will fetch actual image sizes
				hash := 0
				for _, c := range bundle.BundlePath {
					hash = hash*31 + int(c)
				}
				mockSize := int64((hash%300 + 10) * 1000000) // 10-309 MB range

				img := &models.OperatorImage{
					Operator:   pkg.Name,
					CSVName:    bundle.CSVName,
					ShaDigest:  bundle.BundlePathDigest,
					QuayImage:  bundle.BundlePath,
					Size:       mockSize,
					LayerCount: (hash % 20) + 5, // 5-24 layers
				}
				allImages = append(allImages, img)
			}
		}
	}

	if !cfg.Quiet {
		logf("Fetched %d total images from %d packages", len(allImages), len(packages))
	}

	if len(allImages) == 0 {
		return nil, fmt.Errorf("no images found in source registry")
	}

	// Select images based on strategy
	selected := selectByStrategy(allImages, cfg.Strategy)

	if !cfg.Quiet && !cfg.SkipValidation {
		logf("Validating image accessibility (%d images)", len(selected))
	}

	// Validate image accessibility (Phase 1: sequential, Phase 2: concurrent)
	if !cfg.DryRun && !cfg.SkipValidation {
		validator := registry.NewValidator(cfg.ValidationTimeout)
		for i, img := range selected {
			accessible := validator.ValidateAccessibility(ctx, img.QuayImage)
			if !cfg.Quiet && i%10 == 0 {
				logf("Validated %d/%d images", i, len(selected))
			}
			if !accessible && !cfg.Quiet {
				logf("Warning: Image not accessible: %s", img.QuayImage)
			}
		}
	}

	return selected, nil
}

// fetchAllPackages fetches operator packages from source registry API
// Phase 1 only fetches first 50 packages for performance
func fetchAllPackages(ctx context.Context, client *pyxis.Client, cfg *config.Config) ([]pyxis.Package, error) {
	// Fetch first page with 50 packages
	resp, err := client.FetchPackages(ctx, 0, 50)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// selectByStrategy selects images based on selection strategy
func selectByStrategy(allImages []*models.OperatorImage, strategy *models.SelectionStrategy) []*models.OperatorImage {
	if strategy.RandomMode {
		// Random mode: return first N images (Phase 2 will randomize)
		count := strategy.Count
		if count > len(allImages) {
			count = len(allImages)
		}
		return allImages[:count]
	}

	// Size-based mode: categorize and select by size
	var small, medium, large []*models.OperatorImage

	for _, img := range allImages {
		// Size thresholds are in global config, using defaults from Phase 1
		smallThreshold := int64(52428800)   // 50MB
		largeThreshold := int64(209715200)  // 200MB

		if img.Size < smallThreshold {
			small = append(small, img)
		} else if img.Size < largeThreshold {
			medium = append(medium, img)
		} else {
			large = append(large, img)
		}
	}

	selected := make([]*models.OperatorImage, 0)
	selected = append(selected, small[:min(len(small), strategy.SmallCount)]...)
	selected = append(selected, medium[:min(len(medium), strategy.MediumCount)]...)
	selected = append(selected, large[:min(len(large), strategy.LargeCount)]...)

	return selected
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


// selectionModeName returns a string describing the selection mode
func selectionModeName(strategy *models.SelectionStrategy) string {
	if strategy.RandomMode {
		return fmt.Sprintf("random (%d images)", strategy.Count)
	}
	return fmt.Sprintf("size-based (small:%d, medium:%d, large:%d)",
		strategy.SmallCount, strategy.MediumCount, strategy.LargeCount)
}

// logf logs to stderr with timestamp
func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
