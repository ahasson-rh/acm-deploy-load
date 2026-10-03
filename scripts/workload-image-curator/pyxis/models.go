package pyxis

// Package represents an operator package from Pyxis API
type Package struct {
	Name        string `json:"package_name"`
	ID          string `json:"_id"`
	Association string `json:"association"`
	Source      string `json:"source"`
}

// PackageResponse is the API response for packages endpoint
type PackageResponse struct {
	Data  []Package `json:"data"`
	Total int       `json:"total"`
	Page  int       `json:"page"`
}

// Bundle represents an operator bundle from Pyxis API
type Bundle struct {
	BundlePath              string `json:"bundle_path"`
	BundlePathDigest        string `json:"bundle_path_digest"`
	Digest                  string `json:"digest"`
	DockerImageDigest       string `json:"docker_image_digest"`
	ManifestSchema2Digest   string `json:"manifest_schema2_digest"`
	BundleImage             string `json:"bundle_image"`
	Image                   string `json:"image"`
	CSVName                 string `json:"csv_name"`
	RelatedImages           []RelatedImage `json:"related_images"`
	ID                      string `json:"_id"`
}

// RelatedImage represents related image in bundle
type RelatedImage struct {
	Digest     string `json:"digest"`
	Image      string `json:"image"`
	ImageSize  int64  `json:"image_size"`
	LayerCount int    `json:"layer_count"`
}

// BundleResponse is the API response for bundles endpoint
type BundleResponse struct {
	Data  []Bundle `json:"data"`
	Total int      `json:"total"`
	Page  int      `json:"page"`
}
