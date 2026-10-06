package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/acm-deploy-load/workload-image-curator/models"
	"github.com/spf13/cobra"
)

// Config holds all runtime configuration
type Config struct {
	// Image Selection
	Strategy *models.SelectionStrategy

	// Size Thresholds (bytes)
	SmallThreshold int64
	LargeThreshold int64

	// Destination Registry
	DestRegistry string
	DestOrg      string
	PullSecretPath string

	// Mirroring
	IgnoreExisting bool
	DryRun         bool
	AssumeYes      bool

	// Concurrency
	Workers       int
	InspectWorkers int
	RateLimit     float64

	// Validation
	SkipValidation    bool
	ValidationTimeout int

	// Output
	Stdout       bool
	NoFiles      bool
	OutputJSON   string
	OutputTXT    string
	OutputPrefix string
	OutputSize   bool
	Verbose      bool
	Quiet        bool

	// Source Registry API
	SourceRegistryURL string
}

// LoadFromFlags loads configuration from cobra command flags
func LoadFromFlags(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{
		SmallThreshold: 52428800,   // 50MB default
		LargeThreshold: 209715200,  // 200MB default
		PullSecretPath: "/opt/registry/pull-secret-bastion.txt",
		OutputPrefix:   "deployable_operator_images",
		Workers:        5,
		InspectWorkers: 10,
		RateLimit:      10.0,
		ValidationTimeout: 8,
		SourceRegistryURL: os.Getenv("SOURCE_REGISTRY_URL"),
	}


	// Parse flags from command
	var err error

	// Selection strategy
	count, _ := cmd.Flags().GetInt("count")
	strategy, _ := cmd.Flags().GetString("strategy")

	cfg.Strategy = parseStrategy(count, strategy)

	// Thresholds
	if val, err := cmd.Flags().GetInt64("size-small-threshold"); err == nil && val > 0 {
		cfg.SmallThreshold = val
	}
	if val, err := cmd.Flags().GetInt64("size-large-threshold"); err == nil && val > 0 {
		cfg.LargeThreshold = val
	}

	// Destination Registry
	if val, err := cmd.Flags().GetString("dest-registry"); err == nil && val != "" {
		cfg.DestRegistry = val
	}
	if val, err := cmd.Flags().GetString("dest-org"); err == nil && val != "" {
		cfg.DestOrg = val
	}
	if val, err := cmd.Flags().GetString("pull-secret"); err == nil && val != "" {
		cfg.PullSecretPath = val
	}

	// Flags
	cfg.IgnoreExisting, _ = cmd.Flags().GetBool("ignore-existing")
	cfg.DryRun, _ = cmd.Flags().GetBool("dry-run")
	cfg.AssumeYes, _ = cmd.Flags().GetBool("assume-yes")

	// Concurrency
	if val, err := cmd.Flags().GetInt("workers"); err == nil && val > 0 {
		cfg.Workers = val
	}
	if val, err := cmd.Flags().GetInt("inspect-workers"); err == nil && val > 0 {
		cfg.InspectWorkers = val
	}
	if val, err := cmd.Flags().GetFloat64("rate-limit"); err == nil && val > 0 {
		cfg.RateLimit = val
	}

	// Validation
	cfg.SkipValidation, _ = cmd.Flags().GetBool("skip-validation")
	if val, err := cmd.Flags().GetInt("validation-timeout"); err == nil && val > 0 {
		cfg.ValidationTimeout = val
	}

	// Output
	cfg.Stdout, _ = cmd.Flags().GetBool("stdout")
	cfg.NoFiles, _ = cmd.Flags().GetBool("no-files")
	if val, err := cmd.Flags().GetString("output-json"); err == nil && val != "" {
		cfg.OutputJSON = val
	}
	if val, err := cmd.Flags().GetString("output-txt"); err == nil && val != "" {
		cfg.OutputTXT = val
	}
	if val, err := cmd.Flags().GetString("output-prefix"); err == nil && val != "" {
		cfg.OutputPrefix = val
	}
	cfg.OutputSize, _ = cmd.Flags().GetBool("output-size")
	cfg.Verbose, _ = cmd.Flags().GetBool("verbose")
	cfg.Quiet, _ = cmd.Flags().GetBool("quiet")

	// Source Registry API URL
	if val, err := cmd.Flags().GetString("source-registry-url"); err == nil && val != "" {
		cfg.SourceRegistryURL = val
	}

	if cfg.SourceRegistryURL == "" {
		cfg.SourceRegistryURL = "https://catalog.redhat.com/api/containers/v1"
	}

	return cfg, err
}

// parseStrategy parses the strategy string and returns SelectionStrategy
func parseStrategy(count int, strategy string) *models.SelectionStrategy {
	if strategy == "" {
		// Random mode
		if count == 0 {
			count = 50 // default
		}
		return &models.SelectionStrategy{
			RandomMode: true,
			Count:      count,
		}
	}

	// Size-based mode - parse "small:10,medium:30,large:10"
	small, medium, large := 0, 0, 0
	_, _ = parseDistribution(strategy, &small, &medium, &large)

	return &models.SelectionStrategy{
		RandomMode:  false,
		SmallCount:  small,
		MediumCount: medium,
		LargeCount:  large,
	}
}

// parseDistribution parses "small:X,medium:Y,large:Z" format
func parseDistribution(s string, small, medium, large *int) (bool, error) {
	parts := make(map[string]int)

	// Split by comma
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split by colon
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return false, fmt.Errorf("invalid distribution format: %s (expected 'key:value')", pair)
		}

		key := strings.TrimSpace(kv[0])
		val, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			return false, fmt.Errorf("invalid count for %s: %v", key, err)
		}

		parts[key] = val
	}

	if val, ok := parts["small"]; ok {
		*small = val
	}
	if val, ok := parts["medium"]; ok {
		*medium = val
	}
	if val, ok := parts["large"]; ok {
		*large = val
	}

	return true, nil
}
