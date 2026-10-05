package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

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
	rootCmd.Flags().String("dest-registry", "", "Destination registry for mirroring")
	rootCmd.Flags().String("dest-org", "", "Destination organization/namespace")
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

	// Setup graceful shutdown on Ctrl+C
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		if !cfg.Quiet {
			logf("Received signal: %v, shutting down gracefully...", sig)
		}
		cancel()
	}()

	// Phase A: Image Selection
	images, err := selectImages(ctx, cfg)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("image selection cancelled")
		}
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

	// Dynamically fetch packages based on strategy size
	// Estimate: ~1.5 images per package, so fetch with 20% scale factor plus buffer
	targetCount := cfg.Strategy.Total()
	scaleFactor := 1.2   // 20% overhead
	buffer := 2
	packageCount := int(math.Ceil(float64(targetCount)*scaleFactor)) + buffer
	if packageCount > 100 {
		packageCount = 100 // Cap at 100
	}

	// Fetch operator packages
	packages, err := fetchAllPackages(ctx, srcClient, packageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages: %w", err)
	}

	if !cfg.Quiet {
		logf("Found %d operator packages (fetched %d, need ~%d)", len(packages), packageCount, targetCount)
	}

	// Fetch bundles and extract images (concurrent)
	allImages := make([]*models.OperatorImage, 0, targetCount*2)
	allImagesMu := sync.Mutex{}

	// Use errgroup for simple concurrent processing
	var g errgroup.Group
	g.SetLimit(10) // 10 concurrent bundle fetches

	// Determine logging frequency based on total packages
	logFrequency := 10
	if len(packages) <= 10 {
		logFrequency = 2
	}

	// Enqueue bundle fetch jobs for each package
	for i, pkg := range packages {
		// Early exit check
		allImagesMu.Lock()
		if len(allImages) >= targetCount*3 {
			allImagesMu.Unlock()
			break
		}
		allImagesMu.Unlock()

		pkgName := pkg.Name // Capture for closure

		g.Go(func() error {
			bundles, err := srcClient.FetchBundles(ctx, pkgName, 10)
			if err != nil {
				if !cfg.Quiet {
					logf("Warning: Failed to fetch bundles for %s: %v", pkgName, err)
				}
				return nil // Non-fatal error
			}

			images := make([]*models.OperatorImage, 0)
			for _, bundle := range bundles.Data {
				// Use bundle_path as the image (main bundle image)
				if bundle.BundlePath != "" {
					// Generate deterministic mock size for categorization
					hash := 0
					for _, c := range bundle.BundlePath {
						hash = hash*31 + int(c)
					}
					mockSize := int64((hash%300 + 10) * 1000000) // 10-309 MB range

					img := &models.OperatorImage{
						Operator:   pkgName,
						CSVName:    bundle.CSVName,
						ShaDigest:  bundle.BundlePathDigest,
						QuayImage:  bundle.BundlePath,
						Size:       mockSize,
						LayerCount: (hash % 20) + 5, // 5-24 layers
					}
					images = append(images, img)
				}
			}

			allImagesMu.Lock()
			allImages = append(allImages, images...)
			allImagesMu.Unlock()

			return nil
		})

		if i%logFrequency == 0 && !cfg.Quiet {
			logf("Queued packages for processing: %d/%d", i+1, len(packages))
		}
	}

	// Wait for all bundle fetches to complete
	if err := g.Wait(); err != nil {
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("bundle fetch cancelled")
		}
		if !cfg.Quiet {
			logf("Warning: Some bundle fetches failed: %v", err)
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

	// Validate image accessibility (concurrent)
	if !cfg.DryRun && !cfg.SkipValidation {
		validator := registry.NewValidator(cfg.ValidationTimeout)
		var vg errgroup.Group
		vg.SetLimit(cfg.InspectWorkers)

		for _, img := range selected {
			image := img // Capture for closure

			vg.Go(func() error {
				accessible := validator.ValidateAccessibility(ctx, image.QuayImage)
				if !accessible && !cfg.Quiet {
					logf("Warning: Image not accessible: %s", image.QuayImage)
				}
				return nil
			})
		}

		if err := vg.Wait(); err != nil {
			if ctx.Err() == context.Canceled {
				return nil, fmt.Errorf("image validation cancelled")
			}
		}

		if !cfg.Quiet {
			logf("Validated %d images", len(selected))
		}
	}

	return selected, nil
}

// fetchAllPackages fetches operator packages from source registry API
func fetchAllPackages(ctx context.Context, client *pyxis.Client, count int) ([]pyxis.Package, error) {
	// Fetch requested number of packages
	resp, err := client.FetchPackages(ctx, 0, count)
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
