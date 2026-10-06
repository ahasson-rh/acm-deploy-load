package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

func TestWriteJSON_Format(t *testing.T) {
	images := []*models.OperatorImage{
		{
			Operator:  "test-operator",
			CSVName:   "test-operator.v1.0.0",
			ShaDigest: "sha256:abc123",
			QuayImage: "quay.io/test/operator@sha256:abc123",
		},
	}

	tmpdir := t.TempDir()
	filepath := filepath.Join(tmpdir, "test.json")

	formatter := NewFormatter("test")
	err := formatter.WriteJSON(images, filepath)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Verify JSON structure
	data, _ := os.ReadFile(filepath)
	var result []map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 image, got %d", len(result))
	}

	// Check required fields
	requiredFields := []string{"operator", "csv_name", "sha_digest", "quay_image"}
	for _, field := range requiredFields {
		if _, ok := result[0][field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestWriteTXT_Format(t *testing.T) {
	images := []*models.OperatorImage{
		{
			Operator:  "test-operator-1",
			CSVName:   "test-operator-1.v1.0.0",
			ShaDigest: "sha256:abc123",
			QuayImage: "quay.io/test/operator-1@sha256:abc123",
		},
		{
			Operator:  "test-operator-2",
			CSVName:   "test-operator-2.v2.0.0",
			ShaDigest: "sha256:def456",
			QuayImage: "quay.io/test/operator-2@sha256:def456",
		},
	}

	tmpdir := t.TempDir()
	filepath := filepath.Join(tmpdir, "test.txt")

	formatter := NewFormatter("test")
	err := formatter.WriteTXT(images, filepath)
	if err != nil {
		t.Fatalf("WriteTXT failed: %v", err)
	}

	// Verify TXT format (one pull spec per line)
	data, _ := os.ReadFile(filepath)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	if len(lines) != len(images) {
		t.Errorf("expected %d lines, got %d", len(images), len(lines))
	}

	for i, line := range lines {
		if string(line) != images[i].QuayImage {
			t.Errorf("line %d: got %q, want %q", i, string(line), images[i].QuayImage)
		}
	}
}

func TestWriteStdout_Format(t *testing.T) {
	images := []*models.OperatorImage{
		{
			QuayImage: "quay.io/test/operator@sha256:abc123",
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter("test")
	err := formatter.writeToWriter(&buf, images)
	if err != nil {
		t.Fatalf("writeToWriter failed: %v", err)
	}

	got := buf.String()
	want := "quay.io/test/operator@sha256:abc123\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONOutputMatchesPythonSchema(t *testing.T) {
	// Test that JSON output exactly matches Python implementation schema
	image := &models.OperatorImage{
		Operator:  "local-storage-operator",
		CSVName:   "local-storage-operator.v4.15.0",
		ShaDigest: "sha256:abc123def456",
		QuayImage: "quay.io/openshift/local-storage-operator@sha256:abc123def456",
	}

	data, _ := json.MarshalIndent([]*models.OperatorImage{image}, "", "  ")

	// Expected Python output format
	var decoded []map[string]interface{}
	json.Unmarshal(data, &decoded)

	// Verify field order and names match Python
	expectedFields := []string{"operator", "csv_name", "sha_digest", "quay_image"}
	for _, field := range expectedFields {
		if _, ok := decoded[0][field]; !ok {
			t.Errorf("missing field: %s", field)
		}
	}

	// Verify no extra fields leaked into JSON
	if len(decoded[0]) != 4 {
		t.Errorf("expected 4 fields, got %d", len(decoded[0]))
	}
}

func TestWriteJSON_WithSize(t *testing.T) {
	images := []*models.OperatorImage{
		{
			Operator:   "test-operator",
			CSVName:    "test-operator.v1.0.0",
			ShaDigest:  "sha256:abc123",
			QuayImage:  "quay.io/test/operator@sha256:abc123",
			Size:       100 * 1024 * 1024, // 100MB
			LayerCount: 5,
		},
	}

	tmpdir := t.TempDir()
	filepath := filepath.Join(tmpdir, "test-size.json")

	formatter := NewFormatterWithSize("test")
	err := formatter.WriteJSON(images, filepath)
	if err != nil {
		t.Fatalf("WriteJSON with size failed: %v", err)
	}

	// Verify JSON structure includes size fields
	data, _ := os.ReadFile(filepath)
	var result []map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Should have size_bytes, size, and category
	if _, ok := result[0]["size_bytes"]; !ok {
		t.Error("missing size_bytes field")
	}
	if _, ok := result[0]["size"]; !ok {
		t.Error("missing size field")
	}
	if _, ok := result[0]["category"]; !ok {
		t.Error("missing category field")
	}

	// Verify values
	if result[0]["size_bytes"] != float64(100*1024*1024) {
		t.Errorf("size_bytes: got %v, want %v", result[0]["size_bytes"], float64(100*1024*1024))
	}
	if result[0]["category"] != "medium" {
		t.Errorf("category: got %v, want medium", result[0]["category"])
	}
}

func TestWriteTXT_WithSize(t *testing.T) {
	images := []*models.OperatorImage{
		{
			Operator:   "small-op",
			CSVName:    "small-op.v1.0.0",
			ShaDigest:  "sha256:small",
			QuayImage:  "quay.io/test/small@sha256:small",
			Size:       10 * 1024 * 1024, // 10MB
			LayerCount: 3,
		},
		{
			Operator:   "large-op",
			CSVName:    "large-op.v1.0.0",
			ShaDigest:  "sha256:large",
			QuayImage:  "quay.io/test/large@sha256:large",
			Size:       500 * 1024 * 1024, // 500MB
			LayerCount: 25,
		},
	}

	tmpdir := t.TempDir()
	filepath := filepath.Join(tmpdir, "test-size.txt")

	formatter := NewFormatterWithSize("test")
	err := formatter.WriteTXT(images, filepath)
	if err != nil {
		t.Fatalf("WriteTXT with size failed: %v", err)
	}

	// Verify TXT format (pull spec, size, category per line)
	data, _ := os.ReadFile(filepath)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Check format: should be tab-separated
	line1Parts := bytes.Split(lines[0], []byte("\t"))
	if len(line1Parts) != 3 {
		t.Errorf("expected 3 tab-separated fields, got %d", len(line1Parts))
	}

	if string(line1Parts[0]) != images[0].QuayImage {
		t.Errorf("pull spec mismatch: got %s, want %s", line1Parts[0], images[0].QuayImage)
	}

	// Second line should have 500MB
	line2Parts := bytes.Split(lines[1], []byte("\t"))
	if !bytes.Contains(line2Parts[1], []byte("500")) {
		t.Errorf("expected 500 in size field, got %s", line2Parts[1])
	}
}

func TestFormatSize(t *testing.T) {
	formatter := NewFormatter("test")

	tests := []struct {
		bytes    int64
		contains string // Just check if the result contains this substring
	}{
		{512, "KB"},
		{1024 * 1024, "MB"},
		{10 * 1024 * 1024, "MB"},
		{1024 * 1024 * 1024, "GB"},
		{0, "unknown"},        // Handle zero
		{-1, "unknown"},       // Handle negative
		{-108398, "unknown"},  // Handle negative from user report
	}

	for _, tt := range tests {
		result := formatter.formatSize(tt.bytes)
		if !bytes.Contains([]byte(result), []byte(tt.contains)) {
			t.Errorf("formatSize(%d): got %q, expected to contain %q", tt.bytes, result, tt.contains)
		}
	}
}

func TestWriteStdout_WithSize(t *testing.T) {
	images := []*models.OperatorImage{
		{
			Operator:   "test-op",
			QuayImage:  "quay.io/test/operator@sha256:abc123",
			Size:       100 * 1024 * 1024,
			LayerCount: 5,
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatterWithSize("test")
	err := formatter.writeToWriter(&buf, images)
	if err != nil {
		t.Fatalf("writeToWriter with size failed: %v", err)
	}

	got := buf.String()
	// Should contain pull spec, size, and category (tab-separated)
	if !bytes.Contains([]byte(got), []byte("quay.io/test/operator@sha256:abc123")) {
		t.Error("output missing pull spec")
	}
	if !bytes.Contains([]byte(got), []byte("100")) {
		t.Error("output missing size indicator")
	}
	if !bytes.Contains([]byte(got), []byte("medium")) {
		t.Error("output missing category")
	}
}

func TestOutputSize_WithUnknownSizes(t *testing.T) {
	// Test that images with unknown size (Size <= 0) are output without size info
	// --output-size is just a presentation flag, not a filter
	images := []*models.OperatorImage{
		{
			Operator:   "good-image",
			QuayImage:  "quay.io/test/good@sha256:abc123",
			Size:       100 * 1024 * 1024,
			LayerCount: 5,
		},
		{
			Operator:   "bad-image-zero",
			QuayImage:  "quay.io/test/bad-zero@sha256:def456",
			Size:       0, // Unknown size
			LayerCount: 0,
		},
		{
			Operator:   "bad-image-negative",
			QuayImage:  "quay.io/test/bad-neg@sha256:ghi789",
			Size:       -1, // Invalid size
			LayerCount: 0,
		},
	}

	// Test JSON output
	tmpdir := t.TempDir()
	jsonPath := filepath.Join(tmpdir, "test.json")

	formatter := NewFormatterWithSize("test")
	err := formatter.WriteJSON(images, jsonPath)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	data, _ := os.ReadFile(jsonPath)
	var result []map[string]interface{}
	json.Unmarshal(data, &result)

	// Should have all 3 images (no filtering)
	if len(result) != 3 {
		t.Errorf("expected 3 images, got %d", len(result))
	}

	// Good image should have size fields
	if _, ok := result[0]["size_bytes"]; !ok {
		t.Error("good image should have size_bytes")
	}

	// Bad images should NOT have size fields (Size <= 0)
	if _, ok := result[1]["size_bytes"]; ok && result[1]["size_bytes"] != nil {
		t.Error("bad-zero image should not have size_bytes")
	}
	if _, ok := result[2]["size_bytes"]; ok && result[2]["size_bytes"] != nil {
		t.Error("bad-neg image should not have size_bytes")
	}

	// Test TXT output
	txtPath := filepath.Join(tmpdir, "test.txt")
	err = formatter.WriteTXT(images, txtPath)
	if err != nil {
		t.Fatalf("WriteTXT failed: %v", err)
	}

	txtData, _ := os.ReadFile(txtPath)
	lines := bytes.Split(bytes.TrimSpace(txtData), []byte("\n"))

	// Should have all 3 lines (no filtering)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// First line should have size info (tab-separated)
	if !bytes.Contains(lines[0], []byte("\t")) {
		t.Error("first line should have size info (tab-separated)")
	}

	// Other lines should be plain pull specs (no tabs)
	if bytes.Contains(lines[1], []byte("\t")) {
		t.Error("second line should NOT have size info (Size=0)")
	}
	if bytes.Contains(lines[2], []byte("\t")) {
		t.Error("third line should NOT have size info (Size<0)")
	}
}

func TestNoFilter_WithoutSizeFlag(t *testing.T) {
	// When --output-size is NOT set, all images should be included regardless of size
	images := []*models.OperatorImage{
		{
			Operator:  "good-image",
			QuayImage: "quay.io/test/good@sha256:abc123",
			Size:      100 * 1024 * 1024,
		},
		{
			Operator:  "bad-image",
			QuayImage: "quay.io/test/bad@sha256:def456",
			Size:      0, // Unknown size
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter("test") // Without size flag
	err := formatter.writeToWriter(&buf, images)
	if err != nil {
		t.Fatalf("writeToWriter without size failed: %v", err)
	}

	output := buf.String()
	lines := bytes.Split(bytes.TrimSpace([]byte(output)), []byte("\n"))

	// Should have both images (no filtering)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines without size flag, got %d", len(lines))
	}
}
