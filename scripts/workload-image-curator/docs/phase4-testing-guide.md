# Phase 4: Registry Pre-Assessment - Testing Guide

## Overview

Phase 4 implements registry pre-assessment functionality, allowing the workload image curator to:
- Query target registries to detect already-mirrored images
- Categorize existing images by size
- Calculate remaining images needed to satisfy the strategy
- Skip redundant downloads when strategy is already satisfied
- Provide user confirmation before proceeding with mirroring

## What Changed

### New Packages
- **`registry/mirror.go`** - Target registry inspection (digest lookup, size extraction)
- **`strategy/planner.go`** - Pre-assessment planning and selection logic
- **`strategy/selector.go`** - Image selection utilities

### Updated Files
- **`main.go`** - Integrated pre-assessment into Phase A.5 workflow
- **`models/strategy.go`** - Added convenience getter methods (Small(), Medium(), Large())
- **`config/config.go`** - Already supports `--ignore-existing` and `--assume-yes` flags

## Feature Details

### 1. Target Registry Inspection (registry/mirror.go)

**MirrorInspector** queries a destination registry to find images that already exist:
- Accepts target registry URL and timeout configuration
- Concurrently inspects up to 10 images in parallel
- Uses the same Inspector (manifest parsing) as source registry
- Returns map of digest → ExistingImage for images found

**Digest-based matching:**
```
Source: quay.io/openshift/operator@sha256:abc123...
Target: dest-registry.local/openshift/operator@sha256:abc123...
```

### 2. Pre-Assessment Planning (strategy/planner.go)

**Planner** orchestrates the assessment and selection process:
- Counts existing images by size category (Small/Medium/Large)
- Calculates remaining needed per category to satisfy strategy
- Filters out already-mirrored images from selection
- Prioritizes smaller images within each category for faster downloads
- Detects when strategy is already satisfied (early exit)

**Remaining Calculation Logic:**
- **Random mode**: Distributes remaining evenly across size categories
- **Size-based mode**: Per-category needs (e.g., need 5 more small, 10 more medium)

**PreAssessmentResult** contains:
- `ExistingCount` - Total images already present
- `ExistingBySize` - Breakdown by size category
- `SelectedCount` - Images selected for download
- `SelectedBySize` - Selected breakdown by size
- `TotalDownloadSize` - Bytes to download
- `SkipDownload` - True if strategy satisfied

### 3. Integration into Main Workflow (main.go)

Pre-assessment runs as **Phase A.5** when:
- NOT in dry-run mode (`--dry-run` disables it)
- Target registry is specified (`--dest-registry`)
- User hasn't bypassed it (`--ignore-existing` disables it)

**Flow:**
```
Phase A: Image Selection
  ↓
Phase A.5: Pre-Assessment (NEW)
  ├─ Query target registry for existing images
  ├─ Categorize existing by size
  ├─ Calculate remaining needed
  ├─ Filter selection to only needed images
  ├─ Print assessment report
  └─ Ask user confirmation (unless -y)
  ↓
(Output files, if any)
  ↓
Phase B: Mirror selected images
```

## Testing Procedures

### Test 1: Pre-Assessment with No Existing Images

**Setup:**
- Empty or non-existent target registry

**Command:**
```bash
./workload-image-curator \
  --strategy "small:5,medium:5,large:2" \
  --dest-registry localhost:5000 \
  --dry-run \
  -y
```

**Expected Results:**
- Pre-assessment finds 0 existing images
- Selected count = 12 (5+5+2)
- No early exit
- Output shows "Remaining Needed: 12 images"

### Test 2: Pre-Assessment with Partial Existing Images

**Setup:**
- Mirror 5 images to target registry first:
  ```bash
  # First run
  ./workload-image-curator \
    -c 5 \
    --dest-registry localhost:5000 \
    -y
  ```

**Command (second run with same strategy):**
```bash
./workload-image-curator \
  --strategy "small:3,medium:5,large:2" \
  --dest-registry localhost:5000 \
  --dry-run \
  -y
```

**Expected Results:**
- Pre-assessment finds previously mirrored images
- Categorizes them by size
- Calculates remaining needed (may be 0 if strategy already satisfied)
- Selection filtered to exclude existing images
- Report shows "Already Present: N images"

### Test 3: Early Exit (Strategy Already Satisfied)

**Setup:**
1. Mirror 10 images to registry:
   ```bash
   ./workload-image-curator -c 10 --dest-registry localhost:5000 -y
   ```

2. Run with same or smaller count:

**Command:**
```bash
./workload-image-curator \
  -c 8 \
  --dest-registry localhost:5000 \
  -y
```

**Expected Results:**
- Pre-assessment detects existing count ≥ strategy count
- Sets `SkipDownload = true`
- Function returns nil (no download)
- Log shows "Strategy already satisfied, no download needed"
- Exit code 0 (success)

### Test 4: User Confirmation Prompt

**Command (without -y flag):**
```bash
./workload-image-curator \
  --strategy "small:2,medium:2,large:1" \
  --dest-registry localhost:5000
```

**Expected Behavior:**
- Pre-assessment completes
- Prints report to stderr
- Prompts "Proceed with download of N images? (y/n)"
- Waits for user input
- If 'y': proceed to mirror
- If 'n': exit with error "download cancelled by user"

### Test 5: Ignore Existing Flag

**Command (with --ignore-existing):**
```bash
./workload-image-curator \
  -c 5 \
  --dest-registry localhost:5000 \
  --ignore-existing \
  --dry-run \
  -y
```

**Expected Results:**
- Pre-assessment is SKIPPED
- All 5 selected images will be "mirrored" (in dry-run, no-op)
- No "Already Present" report
- No user confirmation prompt

### Test 6: Size-Based Distribution with Existing

**Setup:**
- Pre-populate registry with 3 small and 2 medium images

**Command:**
```bash
./workload-image-curator \
  --strategy "small:5,medium:5,large:3" \
  --dest-registry localhost:5000 \
  -v \
  -y
```

**Expected Results:**
- Pre-assessment finds 3 small + 2 medium existing
- Remaining needed: 2 small, 3 medium, 3 large
- Selection includes only remaining images
- Report shows breakdown by category:
  ```
  Already Present: 5 images
    - small: 3
    - medium: 2
  
  Remaining Needed: 8 images
    - small: 2
    - medium: 3
    - large: 3
  ```

### Test 7: Dry-Run Skips Pre-Assessment

**Command (with --dry-run):**
```bash
./workload-image-curator \
  --strategy "small:3,medium:3,large:2" \
  --dest-registry localhost:5000 \
  --dry-run \
  -y
```

**Expected Results:**
- Pre-assessment is SKIPPED (dry-run takes precedence)
- No registry queries
- Selection returns all 8 images as-is
- Log shows "Strategy already satisfied..." (from no-op phase B)
- No HTTP/skopeo calls

## Acceptance Criteria Verification

### ✓ Correctly identifies existing images by digest
- [ ] Pre-assessment matches images in target by digest
- [ ] No false positives or misses for same digest
- [ ] Test with multiple registries (quay.io, registry.redhat.io)

### ✓ Skips already-present images in selection
- [ ] Existing images excluded from selected list
- [ ] Selected count = needed count (not total count)
- [ ] Manifests not re-downloaded for existing images

### ✓ Exits early when strategy already satisfied
- [ ] `SkipDownload = true` when existing ≥ strategy
- [ ] No Phase B (mirror) execution
- [ ] User sees confirmation and success

### ✓ Reports pre-assessment results accurately
- [ ] "Already Present" count is correct
- [ ] "Remaining Needed" calculation is accurate
- [ ] Size breakdown (small/medium/large) is correct
- [ ] Total download size estimate is reasonable

## Integration Testing

### Full Workflow Test (with real registry)

```bash
# 1. First run: mirror baseline images
./workload-image-curator \
  --strategy "small:10,medium:20,large:5" \
  --dest-registry bastion.local:5000 \
  --dest-org operator-images \
  -y

# 2. Second run: should detect existing, mirror additional
./workload-image-curator \
  --strategy "small:15,medium:30,large:10" \
  --dest-registry bastion.local:5000 \
  --dest-org operator-images \
  -y

# 3. Third run: should detect strategy satisfied, skip
./workload-image-curator \
  --strategy "small:15,medium:30,large:10" \
  --dest-registry bastion.local:5000 \
  --dest-org operator-images \
  -y
```

**Expected Flow:**
1. Run 1: Mirrors 35 images total
2. Run 2: Detects 35 existing, mirrors additional 20 (to reach 55 total)
3. Run 3: Detects 55 existing ≥ 55 needed, skips download

## Known Limitations

1. **Network Timeouts**: If target registry is unavailable, pre-assessment fails gracefully and proceeds with full mirror
2. **Partial Matches**: Images may exist but with different tags/digests - only digest matches are detected
3. **Size Estimation**: Pre-assessment shows estimated download size; actual may vary (network overhead, compression)
4. **No Resume**: Partial downloads not resumable - full re-mirror needed on failure

## Debugging

Enable verbose logging to debug pre-assessment:
```bash
./workload-image-curator \
  --strategy "small:5,medium:5,large:2" \
  --dest-registry localhost:5000 \
  -v \
  -y 2>&1 | grep -E "(Pre-run|Assessment|Already|Remaining)"
```

Check assessment result fields:
- `ExistingCount`: Number of images found in target
- `SelectedCount`: Number selected for download
- `TotalDownloadSize`: Estimated download size in bytes
- `SkipDownload`: true if strategy satisfied

## Performance Notes

- **Concurrent Inspection**: Up to 10 parallel manifest queries to target registry
- **Lookup Time**: Typically ~100-200ms per image (DNS + HTTP + JSON parse)
- **Impact**: Full pre-assessment adds ~10-30 seconds for 100 images
- **Optimization**: Pre-assessment is skipped with `--dry-run` or `--ignore-existing`
