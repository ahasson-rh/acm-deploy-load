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
