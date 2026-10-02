package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

// Formatter handles output generation
type Formatter struct {
	prefix string
}

// NewFormatter creates a new output formatter
func NewFormatter(prefix string) *Formatter {
	if prefix == "" {
		prefix = "deployable_operator_images"
	}
	return &Formatter{prefix: prefix}
}

// WriteJSON writes images to JSON file
func (f *Formatter) WriteJSON(images []*models.OperatorImage, filepath string) error {
	if filepath == "" {
		timestamp := time.Now().Format("20060102_150405")
		filepath = fmt.Sprintf("%s_%s.json", f.prefix, timestamp)
	}

	data, err := json.MarshalIndent(images, "", "  ")
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

	for _, img := range images {
		fmt.Fprintf(file, "%s\n", img.QuayImage)
	}

	return nil
}

// WriteStdout writes pull specs to stdout
func (f *Formatter) WriteStdout(images []*models.OperatorImage) error {
	return f.writeToWriter(os.Stdout, images)
}

// writeToWriter writes pull specs to writer
func (f *Formatter) writeToWriter(w io.Writer, images []*models.OperatorImage) error {
	for _, img := range images {
		fmt.Fprintf(w, "%s\n", img.QuayImage)
	}
	return nil
}
