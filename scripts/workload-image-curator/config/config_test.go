package config

import (
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/models"
	"github.com/spf13/cobra"
)

func TestParseStrategy_RandomMode(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		strategy string
		want     models.SelectionStrategy
	}{
		{
			name:  "default count",
			count: 0,
			want: models.SelectionStrategy{
				RandomMode: true,
				Count:      50,
			},
		},
		{
			name:  "custom count",
			count: 100,
			want: models.SelectionStrategy{
				RandomMode: true,
				Count:      100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStrategy(tt.count, "")
			if got.RandomMode != tt.want.RandomMode || got.Count != tt.want.Count {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStrategy_SizeMode(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		want     models.SelectionStrategy
	}{
		{
			name:     "basic distribution",
			strategy: "small:10,medium:30,large:10",
			want: models.SelectionStrategy{
				RandomMode: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStrategy(0, tt.strategy)
			if got.RandomMode != tt.want.RandomMode {
				t.Errorf("got RandomMode %v, want %v", got.RandomMode, tt.want.RandomMode)
			}
		})
	}
}

func TestLoadFromFlags_Defaults(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().IntP("count", "c", 50, "")
	cmd.Flags().String("strategy", "", "")
	cmd.Flags().Int64("size-small-threshold", 52428800, "")
	cmd.Flags().Int64("size-large-threshold", 209715200, "")
	cmd.Flags().String("target-registry", "", "")
	cmd.Flags().String("target-org", "", "")
	cmd.Flags().String("pull-secret", "/opt/registry/pull-secret-bastion.txt", "")
	cmd.Flags().Bool("ignore-existing", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().IntP("workers", "w", 5, "")
	cmd.Flags().Int("inspect-workers", 10, "")
	cmd.Flags().Float64("rate-limit", 10.0, "")
	cmd.Flags().Bool("skip-validation", false, "")
	cmd.Flags().Int("validation-timeout", 8, "")
	cmd.Flags().Bool("assume-yes", false, "")
	cmd.Flags().BoolP("stdout", "s", false, "")
	cmd.Flags().Bool("no-files", false, "")
	cmd.Flags().String("output-json", "", "")
	cmd.Flags().String("output-txt", "", "")
	cmd.Flags().String("output-prefix", "deployable_operator_images", "")
	cmd.Flags().BoolP("verbose", "v", false, "")
	cmd.Flags().BoolP("quiet", "q", false, "")

	cfg, err := LoadFromFlags(cmd)
	if err != nil {
		t.Fatalf("LoadFromFlags failed: %v", err)
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"SmallThreshold", cfg.SmallThreshold, int64(52428800)},
		{"LargeThreshold", cfg.LargeThreshold, int64(209715200)},
		{"PullSecretPath", cfg.PullSecretPath, "/opt/registry/pull-secret-bastion.txt"},
		{"Workers", cfg.Workers, 5},
		{"InspectWorkers", cfg.InspectWorkers, 10},
		{"RateLimit", cfg.RateLimit, 10.0},
		{"ValidationTimeout", cfg.ValidationTimeout, 8},
		{"OutputPrefix", cfg.OutputPrefix, "deployable_operator_images"},
		{"IgnoreExisting", cfg.IgnoreExisting, false},
		{"DryRun", cfg.DryRun, false},
		{"Stdout", cfg.Stdout, false},
		{"NoFiles", cfg.NoFiles, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}
