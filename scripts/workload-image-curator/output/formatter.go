package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/acm-deploy-load/workload-image-curator/categorizer"
	"github.com/acm-deploy-load/workload-image-curator/models"
)

// Formatter handles output generation
type Formatter struct {
	prefix         string
	outputSize     bool
	categorizer    *categorizer.Categorizer
}

// NewFormatter creates a new output formatter
func NewFormatter(prefix string) *Formatter {
	if prefix == "" {
		prefix = "deployable_operator_images"
	}
	return &Formatter{
		prefix:      prefix,
		outputSize:  false,
		categorizer: categorizer.NewCategorizer(categorizer.DefaultThresholds()),
	}
}

// NewFormatterWithSize creates a formatter that includes size information
func NewFormatterWithSize(prefix string) *Formatter {
	f := NewFormatter(prefix)
	f.outputSize = true
	return f
}

// WriteJSON writes images to JSON file
func (f *Formatter) WriteJSON(images []*models.OperatorImage, filepath string) error {
	if filepath == "" {
		timestamp := time.Now().Format("20060102_150405")
		filepath = fmt.Sprintf("%s_%s.json", f.prefix, timestamp)
	}

	var data []byte
	var err error

	if f.outputSize {
		// Convert to extended output format with size
		extImages := f.toExtendedImages(images)
		data, err = json.MarshalIndent(extImages, "", "  ")
	} else {
		// Original format without size
		data, err = json.MarshalIndent(images, "", "  ")
	}

	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// WriteTXT writes pull specs to text file
func (f *Formatter) WriteTXT(images []*models.OperatorImage, filepath string) error {
	if filepath == "" {
		timestamp := time.Now().Format("20060102_150405")
		filepath = fmt.Sprintf("%s_%s.txt", f.prefix, timestamp)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	return f.writeToWriter(file, images)
}

// WriteStdout writes pull specs to stdout
func (f *Formatter) WriteStdout(images []*models.OperatorImage) error {
	return f.writeToWriter(os.Stdout, images)
}

// writeToWriter writes pull specs to writer
func (f *Formatter) writeToWriter(w io.Writer, images []*models.OperatorImage) error {
	for _, img := range images {
		if f.outputSize && img.Size > 0 {
			category := f.categorizer.CategorizeImage(img)
			sizeStr := f.formatSize(img.Size)
			fmt.Fprintf(w, "%s\t%s\t%s\n", img.QuayImage, sizeStr, category)
		} else {
			fmt.Fprintf(w, "%s\n", img.QuayImage)
		}
	}
	return nil
}

// formatSize converts bytes to human-readable format
func (f *Formatter) formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	// Handle zero or negative sizes
	if bytes <= 0 {
		return "unknown"
	}

	switch {
	case bytes < MB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	}
}

// ExtendedImage is the output format when --output-size is specified
type ExtendedImage struct {
	Operator   string `json:"operator"`
	CSVName    string `json:"csv_name"`
	ShaDigest  string `json:"sha_digest"`
	QuayImage  string `json:"quay_image"`
	Size       int64  `json:"size_bytes,omitempty"`
	SizeStr    string `json:"size,omitempty"`
	Category   string `json:"category,omitempty"`
}

// toExtendedImages converts OperatorImage to ExtendedImage format with size info
func (f *Formatter) toExtendedImages(images []*models.OperatorImage) []ExtendedImage {
	extended := make([]ExtendedImage, 0, len(images))

	for _, img := range images {
		ext := ExtendedImage{
			Operator:  img.Operator,
			CSVName:   img.CSVName,
			ShaDigest: img.ShaDigest,
			QuayImage: img.QuayImage,
		}

		// Include size info if available (Size > 0)
		if img.Size > 0 {
			ext.Size = img.Size
			ext.SizeStr = f.formatSize(img.Size)
			ext.Category = f.categorizer.CategorizeImage(img).String()
		}

		extended = append(extended, ext)
	}

	return extended
}
