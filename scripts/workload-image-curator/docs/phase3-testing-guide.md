# Phase 3: Size-Based Categorization - Testing Guide

## Overview
Phase 3 implements intelligent image selection based on size categorization. This enables size-based distribution strategies (e.g., "10 small, 30 medium, 10 large images") for more predictable test scenarios and performance baselines.

## What Changed

**New Packages:**
- `categorizer/` - Size-based image categorization engine
- `registry/inspector.go` - Docker manifest inspection for size extraction

**New Types:**
- `SizeThresholds` - Configuration for size boundaries (bytes)
- `Categorizer` - Main categorization engine with strategy selection
- `Inspector` - Extracts size metadata from Docker registry manifests
- `ManifestMetadata` - Docker v2 manifest structure
- `ImageSizeMetadata` - Extracted size information
- `CategorizedImages` - Images grouped by size
- `CategoryStats` - Per-category statistics

**Modified:**
- `models.OperatorImage` - Added `Size` and `LayerCount` fields (non-exported)

## Testing Procedures

### Unit Tests

Run all categorizer and inspector tests:
```bash
cd scripts/workload-image-curator
go test -v ./categorizer ./registry
```

**Test Coverage:**
- Threshold management (default, custom, boundary validation)
- Image categorization by size
- Size distribution selection
- Category filtering
- Size metadata extraction
- Timeout handling

### Test Cases

#### 1. Threshold Configuration
**Verify:** Default and custom size thresholds work correctly

```bash
# Unit tests validate:
# - Default: Small < 50MB, Medium 50-200MB, Large > 200MB
# - Custom thresholds can be applied
# - Invalid configurations fall back to defaults
# - Threshold ordering is maintained (small < large)
```

#### 2. Image Categorization
**Verify:** Images are correctly classified by size

```bash
# Test images at boundary conditions:
# - 10MB → small
# - 50MB (threshold) → medium
# - 30MB → small
# - 100MB → medium
# - 200MB (threshold) → large
# - 500MB → large
```

**Test:** Run unit tests:
```bash
go test -v -run TestCategorizeImage ./categorizer
```

#### 3. Distribution Selection
**Verify:** Selection algorithm picks the right count from each category

```bash
# Strategy: small:2, medium:1, large:1
# Input: 3 small, 2 medium, 2 large (6 total)
# Output: Should select exactly 2 small, 1 medium, 1 large

go test -v -run TestSelectByDistribution ./categorizer
```

**Key Behavior:**
- Prioritizes smaller images within each category (faster downloads)
- Gracefully handles insufficient images in any category
- Returns all available images if strategy cannot be fully satisfied

#### 4. Size Extraction (Inspector)
**Verify:** Manifest parsing and size calculation

```bash
go test -v -run TestImageSizeMetadata_Calculation ./registry
```

**Validation:**
- Config blob size + all layer sizes = total manifest size
- Layer count includes config blob (1) + number of layers
- HTTP timeout defaults to 8 seconds

#### 5. Sorting Behavior
**Verify:** Images are sorted smallest-first for efficiency

```go
// From test:
// categorized.Small should be sorted: [10MB, 30MB]
// categorized.Medium should be sorted: [100MB, 150MB]
// categorized.Large should be sorted: [300MB, 500MB]
```

Run test:
```bash
go test -v -run TestCategorize ./categorizer
```

## Acceptance Criteria Verification

### ✓ Accurate Size Extraction (within 1%)
- **Test Method:** Extract manifest size and compare to calculated total
- **Pass Condition:** Calculated size matches manifest structure
- **Command:** `go test -v -run TestImageSizeMetadata_Calculation ./registry`

### ✓ Correct Categorization with Default and Custom Thresholds
- **Test Method:** Categorize images at exact threshold boundaries
- **Pass Condition:** All boundary cases correctly classified
- **Command:** `go test -v -run "TestCategorizeImage|TestCustomThresholds" ./categorizer`

### ✓ Distribution Strategy Enforced
- **Test Method:** Request specific distribution and verify output
- **Pass Condition:** Selected images match requested distribution
- **Command:** `go test -v -run TestSelectByDistribution ./categorizer`

## Known Limitations

1. **Size Extraction:** Currently assumes Docker v2 manifest format. Images using older formats may fail extraction.
2. **Registry Authentication:** Supports Quay.io and Red Hat registries with token-based auth. Private registries may require additional configuration.
3. **Network:** Requires network access to registry and manifest endpoints. Air-gapped environments not supported without local manifest cache.

## Integration with Existing Code

The Phase 3 implementation:
- Extends `models.OperatorImage` with size metadata (non-exported fields)
- Adds new `categorizer` package with `Categorizer` type
- Adds new `registry/inspector.go` with `Inspector` type
- Does not modify existing validation or mirroring logic
- Maintains backward compatibility with current CLI flags and output formats

## Manual Testing (Integration)

When Phase 3 is integrated with main workflow:

```bash
# Test categorization in context
# (After Phase 4: Registry Pre-Assessment is implemented)

workload-image-curator \
  --strategy "small:5,medium:10,large:5" \
  --size-small-threshold 52428800 \
  --size-large-threshold 209715200 \
  --dry-run \
  --verbose
```

Expected behavior:
1. Fetch metadata from Pyxis
2. Inspect image sizes from registries
3. Categorize by configured thresholds
4. Select images matching distribution
5. Display categorized results (in dry-run, no download)

## Performance Notes

- **Inspection Phase:** Parallel requests (default 10 workers) to registry manifests
- **Categorization:** O(n log n) due to sorting within categories
- **Memory:** Linear in number of images (negligible for 50-500 images)
- **Network:** One HTTP request per image for manifest inspection

## Troubleshooting

### Test Failures

**Import errors:**
```
package acm-deploy-load/models is not in std
```
**Fix:** Use full module path: `github.com/acm-deploy-load/workload-image-curator/models`

**HTTP errors in ExtractSize:**
```
http: server gave HTTP response to HTTPS client
```
**Note:** Unit tests mock manifest structure; actual HTTP tests require real registry or proper test server setup.

### Integration Issues

If size data is unavailable for an image:
- Inspection fails → `Size = 0`
- Categorized as `CategorySmall` (since 0 < threshold)
- Warning logged during categorization

## Files Modified/Created

**New:**
- `scripts/workload-image-curator/categorizer/size.go`
- `scripts/workload-image-curator/categorizer/size_test.go`
- `scripts/workload-image-curator/categorizer/thresholds.go`
- `scripts/workload-image-curator/categorizer/thresholds_test.go`
- `scripts/workload-image-curator/registry/inspector.go`
- `scripts/workload-image-curator/registry/inspector_test.go`
- `scripts/workload-image-curator/docs/phase3-testing-guide.md`

**Modified:**
- `scripts/workload-image-curator/models/image.go` (added `Size`, `LayerCount` fields)

## Next Steps

Phase 4 will integrate categorization with the pre-run assessment workflow:
- Query target registry for existing images
- Categorize existing images by size
- Calculate remaining needed per category
- Select new images to satisfy distribution strategy
- Support `--ignore-existing` flag to bypass this logic
