package models

// OperatorImage represents metadata for an operator container image
type OperatorImage struct {
	Operator   string `json:"operator"`
	CSVName    string `json:"csv_name"`
	ShaDigest  string `json:"sha_digest"`
	QuayImage  string `json:"quay_image"`
	Size       int64  `json:"-"` // Size in bytes, not in JSON output
	LayerCount int    `json:"-"` // For reporting, not in JSON output
}

// ImageCategory represents size category of an image
type ImageCategory int

const (
	CategorySmall ImageCategory = iota
	CategoryMedium
	CategoryLarge
)

func (c ImageCategory) String() string {
	switch c {
	case CategorySmall:
		return "small"
	case CategoryMedium:
		return "medium"
	case CategoryLarge:
		return "large"
	default:
		return "unknown"
	}
}
