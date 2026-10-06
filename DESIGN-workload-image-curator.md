# Workload Image Curator - Go Implementation Design

## Executive Summary

This document outlines the design for a GoLang-based utility to replace the existing Python implementation (`ansible/roles/operator-container-images-curator/files/generate_operator_image_list.py`) with enhanced capabilities:

- **Multithreaded downloading** - Parallel skopeo operations with configurable worker pools
- **Size-based download strategy** - Categorize images by size (small/medium/large) with configurable distribution
- **Rate limiting** - Avoid registry throttling while maximizing throughput  
- **Registry pre-assessment** - Skip already-mirrored images, exit early if strategy satisfied
- **Backward compatibility** - Drop-in replacement for existing Ansible role

## Context

The current Python implementation queries the Red Hat Pyxis API for random operator container images, validates accessibility, and generates image lists for ACS (Advanced Cluster Security) scanner testing. Limitations include:

- **Sequential processing** - Single-threaded API queries and validation
- **No size-based selection** - Cannot select images based on size for predictable test scenarios
- **No registry pre-assessment** - Always downloads without checking existing images
- **Limited concurrency** - Ansible calls python script that mirrors images sequentially, batching in Ansible shifts concurrency logic outside the application limiting error recovery and/or missing downloads reconciliation

## Architecture

### Directory Structure

```
scripts/
  workload-image-curator/           # Go utility (self-contained)
    main.go                         # Entry point with CLI
    go.mod                          # Go module
    go.sum                          
    Makefile                        # Build automation
    README.md                       
    
    pyxis/                          # Pyxis API client
      client.go                     # HTTP client with retry logic
      models.go                     # API response structures
      pagination.go                 # Concurrent page fetching
    
    registry/                       # Registry operations
      inspector.go                  # Image metadata extraction (size, layers)
      validator.go                  # Accessibility checks (skopeo + HTTP)
      mirror.go                     # Target registry inspection
      auth.go                       # Pull secret handling
    
    categorizer/                    # Image categorization
      size.go                       # Size classification logic
      thresholds.go                 # Size threshold configuration
      distribution.go               # Size distribution strategy
    
    downloader/                     # Concurrent mirroring
      worker_pool.go                # Worker pool orchestration
      skopeo.go                     # Skopeo wrapper
      retry.go                      # Exponential backoff retry
      progress.go                   # Real-time progress reporting
    
    strategy/                       # Download planning
      planner.go                    # Pre-run assessment
      selector.go                   # Image selection algorithm
    
    output/                         # Output generation
      formatter.go                  # JSON + TXT formatters
      logger.go                     # Structured logging (stderr)
    
    config/                         # Configuration
      config.go                     # Configuration management
      env.go                        # Environment variable mapping
    
    models/                         # Shared types
      image.go                      # Image metadata structures
      strategy.go                   # Strategy configuration
    
    test/                           # Tests
      integration/
      unit/
```

After building, the binary is stored in `scripts/workload-image-curator` for testing. The binary is also also copied to the ansible operator-container-images-curator role.

### Data Flow

```
1. API Query Phase
   ├─ Fetch operator packages from Pyxis (paginated, 10 concurrent workers)
   ├─ For each package, fetch latest bundle metadata
   ├─ Extract digest and construct pull spec
   └─ Enqueue for inspection

2. Inspection Phase (10 parallel workers, skipped in --dry-run)
   ├─ Validate image accessibility (skopeo/HTTP fallback)
   ├─ Extract size metadata from registry manifest
   ├─ Collect layer count
   └─ Build OperatorImage records with size data

3. Categorization Phase
   ├─ Calculate size thresholds (default: 50MB/200MB, or custom via flags)
   ├─ Classify images by size: Small/Medium/Large
   ├─ Extract layer count for each image (for reporting)
   └─ Sort within size categories (smallest first for faster downloads)

4. Pre-Run Assessment Phase
   ├─ Query target registry for existing images (digest-based)
   ├─ Filter out already-mirrored images
   ├─ Categorize existing images by size
   ├─ Calculate remaining needed per size category
   ├─ Select images to satisfy size strategy
   ├─ Exit early with summary if strategy already satisfied
   └─ Get user input confirmation to proceed unless -y or --assume-yes is specified

5. Mirror Phase (5 parallel workers, configurable, skipped in --dry-run)
   ├─ Each worker executes `skopeo copy` to mirror from source (quay.io) to target registry
   ├─ Command: `skopeo copy docker://quay.io/repo/image@sha256:digest docker://dest-registry/org/image@sha256:digest`
   ├─ Retry logic: exponential backoff, max 3 attempts
   ├─ Rate limiting via token bucket (default: 10 req/s collectively)
   ├─ Real-time progress reporting to stderr
   └─ Aggregate results and errors

6. Output Phase
   ├─ Generate JSON metadata file (matches Python format)
   ├─ Generate TXT pull spec list (one per line)
   ├─ Print summary to stderr
   └─ Exit with appropriate code
```

### Concurrency Model

**Worker Pools:**
- **API Query Pool** - 10 workers for concurrent Pyxis bundle queries
- **Inspection Pool** - 10 workers for parallel image validation and size extraction
- **Download Pool** - 5 workers (configurable) for concurrent skopeo copy operations

**Rate Limiting:**
- Token bucket algorithm (`golang.org/x/time/rate`)
- Default: 10 requests/second **collectively across all workers**
- Configurable via `--rate-limit` flag
- Example: `--rate-limit 10` with 5 workers = ~2 req/s per worker (shared pool)

**Synchronization:**
- Channels for job distribution and result aggregation
- `context.Context` for cancellation propagation (Ctrl+C support)
- `sync.WaitGroup` for worker lifecycle management

## Utility Workflow

**Standard Mode (Default):**
1. **Phase A - Image Selection:** 
   - Fetch metadata from Pyxis API (bundle names, digests, sizes)
   - Validate image accessibility (HTTP checks to source registry [quay.io, registry.redhat.io, etc])
   - Categorize by size
   - Select images based on strategy
   - Get user confirmation to proceed (unless `-y | --assume-yes` specified)
2. **Phase B - Image Mirroring:** 
   - Mirror selected images to target registry using `skopeo copy`

**Dry Run Mode (`--dry-run`):**
- Fetches metadata from Pyxis API only
- **Skips** image accessibility validation (no HTTP calls)
- **Skips** Phase B (no mirroring with skopeo)
- Outputs report showing what would be selected/mirrored with warning that numbers may differ slightly when run in default mode due to accessibility validation
- Useful for previewing selection strategy without validation overhead

---

## Key Features

### 1. Two Image Selection Modes

**Mode A: Random Selection (`-c 50`)**
- Fetch N random images from Pyxis
- No size-based filtering
- Simple baseline for testing

**Mode B: Size-Based Selection (`--strategy "small:10,medium:30,large:10"`)**
- Fetch and categorize by size
- Select specific distribution
- Default thresholds: Small < 50MB, Medium 50-200MB, Large > 200MB
- Optional custom thresholds via `--size-small-threshold` and `--size-large-threshold`

### 2. Always-On Mirroring (with Dry-Run Option)

Images are **always mirrored** to target registry using `skopeo copy` (unless `--dry-run` specified):

```bash
# Full workflow: Fetch + validate + mirror
workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --dest-registry bastion:5000 \
  --dest-org operator-containers \
  --stdout

# Dry run: Only fetch metadata, skip validation and mirroring
workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --dest-registry bastion:5000 \
  --dest-org operator-containers \
  --dry-run \
  --stdout
  # Output: What would be selected, but no HTTP checks to source registry or skopeo calls
```

### 2. Registry Pre-Assessment

Before downloading, the tool:
1. Queries target registry for existing images by digest
2. Categorizes existing images by size
3. Calculates remaining needed per size category
4. Selects only images needed to satisfy size strategy
5. Exits with summary if strategy already satisfied and still generates a list of images per output flags

**Example Output:**
```
Pre-run Assessment:
  Already Present: 15 images (3 small, 8 medium, 4 large)
  
  Remaining Needed: 35 images
    Strategy: small:7, medium:22, large:6
  
  Total Download Size: 4.2 GB
  
Proceeding with download...
```

### 3. Retry Logic

**Exponential backoff** matching existing patterns:
- Max attempts: 3
- Initial delay: 5 seconds
- Backoff multiplier: 2.0
- Max delay: 60 seconds

**Retryable errors:** Network timeouts, 5xx responses, temporary failures, rate limits  
**Non-retryable errors:** 401 Unauthorized, 404 Not Found, invalid digest format

### 4. Progress Reporting

Real-time progress to stderr (doesn't interfere with stdout):
```
[45/50] Success: 42 | Failed: 3 | Rate: 2.3 img/s | ETA: 00:02:10
```

Quiet mode (`-q`) suppresses progress for CI/automation.

## CLI Interface

### Core Flags

```
# Basic options
-c, --count INT              Total images (default: 50)
--strategy STRING            Distribution: "small:10,medium:30,large:10"

# Size thresholds (bytes, optional - only used with --strategy)
--size-small-threshold INT   Small/medium boundary (default: 52428800 bytes / 50MB)
--size-large-threshold INT   Medium/large boundary (default: 209715200 bytes / 200MB)

# Concurrency
-w, --workers INT            Download workers (default: 5)
--rate-limit FLOAT           Max requests/sec collectively across all workers (default: 10)
--inspect-workers INT        Inspection workers (default: 10)

# Registry & Mirroring
--dest-registry STRING       Destination registry for mirroring (required for mirroring phase)
--dest-org STRING            Destination organization/namespace
--pull-secret FILE           Pull secret path (default: /opt/registry/pull-secret-bastion.txt)
--ignore-existing            Ignore existing images in target, force mirror all (skip pre-run assessment)
--dry-run                    Fetch metadata only, skip validation checks and mirroring (no HTTP/skopeo calls)

# Validation
--skip-validation            Skip accessibility checks
--validation-timeout SEC     Timeout for validation (default: 8)
-y, --assume-yes             Assume yes to user prompt asking to proceed with download

# Output
-s, --stdout                 Output image pull specs to stdout (for Ansible capture)
                             Works with any other output combination (files, etc.)
--output-json FILE           JSON output path (omit to skip JSON output)
--output-txt FILE            TXT output path (omit to skip TXT output)
--output-prefix STRING       Filename prefix for auto-generated names (default: deployable_operator_images)
--no-files                   Skip all file output (only stdout if -s specified)
-v, --verbose                Verbose logging
-q, --quiet                  Suppress progress
```

### Environment Variables

Matches Python implementation for backward compatibility:

```
PYXIS_BASE_URL               Pyxis API endpoint
TARGET_REGISTRY              Default target registry
OUTPUT_PREFIX                Output filename prefix
OUTPUT_TXT_FILE              Explicit TXT file path
OUTPUT_JSON_FILE             Explicit JSON file path
REGISTRY_PULL_SECRET         Pull secret path
CURATOR_WORKERS              Default worker count
CURATOR_RATE_LIMIT           Default rate limit
```

### Usage Examples

**Random selection (no size strategy):**
```bash
# Download 50 random images, any size
workload-image-curator -c 50

# Output to stdout for Ansible capture
workload-image-curator -c 50 --stdout

# Both stdout and JSON file
workload-image-curator -c 50 --stdout --output-json images.json
```

**Size-based strategy:**
```bash
# Download 50 total: 10 small, 30 medium, 10 large
workload-image-curator --strategy "small:10,medium:30,large:10"

# With stdout for Ansible
workload-image-curator --strategy "small:10,medium:30,large:10" --stdout

# All outputs: stdout + JSON + TXT files
workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --stdout \
  --output-json metadata.json \
  --output-txt pullspecs.txt
```

**Output Combinations:**
```bash
# Only stdout (no files)
workload-image-curator -c 50 --stdout --no-files

# Files with auto-generated names (from --output-prefix)
workload-image-curator -c 50 --output-prefix "test-run"
# Creates: test-run_<timestamp>.json and test-run_<timestamp>.txt

# Stdout + files
workload-image-curator -c 50 --stdout --output-prefix "test-run"
```

**With pre-assessment:**
```bash
workload-image-curator \
  --dest-registry bastion.example.com:5000 \
  --dest-org operator-containers \
  --strategy "small:20,medium:20,large:10"
```

## Output Format

### JSON (matches Python)

```json
[
  {
    "operator": "local-storage-operator",
    "csv_name": "local-storage-operator.v4.15.0",
    "sha_digest": "sha256:abc123...",
    "quay_image": "quay.io/openshift/local-storage-operator@sha256:abc123..."
  }
]
```

### TXT (matches Python)

```
quay.io/openshift/local-storage-operator@sha256:abc123...
quay.io/openshift/logging-operator@sha256:def456...
```

### Summary Report (stderr)

```
Operator Image Curator - Summary Report
========================================

API Query:
  - Packages fetched: 500
  - Bundles queried: 125
  - Images validated: 100
  - Validation failures: 25

Categorization By Size:
  - Small images (<50MB): 35 (total: 1.2 GB, avg: 8 layers)
  - Medium images (50-200MB): 45 (total: 5.4 GB, avg: 22 layers)
  - Large images (>200MB): 20 (total: 6.8 GB, avg: 45 layers)

Pre-run Assessment:
  - Already present in target: 15 images
  - Selected for mirroring: 35 images (needed to satisfy strategy)
  - Failed to mirror: 0 images
  - Not selected: 50 images (beyond strategy requirements)

Mirror Performance:
  - Total image size: 4.2 GB
  - Duration: 15m 32s
  - Average rate: 2.3 images/s
  - Bandwidth: 4.5 MB/s

Output:
  - JSON metadata: deployable_operator_images_20261001_143022.json
  - TXT pull specs: deployable_operator_images_20261001_143022.txt
  - Images processed: 35 (selected for mirroring)
```

## Ansible Integration

### Updated Role Task

The Go binary can be used as a drop-in replacement:

**Current (Python):**
```yaml
- name: Generate operator image list
  shell: |
    {{ playbook_dir }}/roles/operator-container-images-curator/files/generate_operator_image_list.py \
      -c {{ acs_operator_containers_target_count }} \
      -y \
      --stdout --no-files
  register: operator_images
```

**New (Go):**
```yaml
- name: Generate workload image list
  shell: |
    {{ playbook_dir }}/../scripts/workload-image-curator \
      -c {{ acs_operator_containers_target_count }} \
      -y \
      --stdout --no-files
  register: operator_images
```

**With size strategy:**
```yaml
- name: Generate workload image list with size distribution
  shell: |
    {{ playbook_dir }}/../scripts/workload-image-curator \
      --strategy "{{ acs_image_size_distribution | default('small:10,medium:30,large:10') }}" \
      --workers {{ acs_mirror_workers | default(5) }} \
      --dest-registry {{ acs_internal_registry }} \
      --dest-org {{ acs_org_name }} \
      --assume-yes \
      --stdout --no-files
  register: operator_images
```

### New Ansible Variables

Add to `ansible/roles/operator-container-images-curator/defaults/main.yml`:

```yaml
# New variables for Go implementation
acs_image_size_distribution: "small:10,medium:30,large:10"
acs_size_small_threshold: 52428800   # 50MB (optional, uses default if not set)
acs_size_large_threshold: 209715200  # 200MB (optional, uses default if not set)
acs_mirror_workers: 5
acs_rate_limit: 10.0
acs_ignore_existing: false            # false = use pre-run assessment (default), true = force mirror all
```

## Implementation Phases

### Phase Requirements

**For each phase:**
- Implementation summary must be written to `scripts/workload-image-curator/docs/phase{N}-testing-guide.md`
- Summary should be concise (no detailed code changes, focus on what changed and why)
- Include testing procedures, acceptance criteria verification, and known limitations

---

### Phase 1: Core Functionality
**Goal:** Feature parity with Python implementation

**Documentation:**
- [x] Testing guide: `scripts/workload-image-curator/docs/phase1-testing-guide.md`

**Deliverables:**
- [x] Go module initialization
- [x] Pyxis API client with retry logic
- [x] Image validation (skopeo + HTTP fallback)
- [x] Sequential processing workflow
- [x] JSON + TXT output (exact Python format)
- [x] Environment variable support
- [x] CLI flags matching Python
- [x] Compatibility tests
- [x] README with installation and usage

**Acceptance Criteria:**
- Produces identical output to Python for same inputs
- All Python CLI flags and env vars supported
- Exit codes match Python behavior

**Critical Files:**
- `scripts/workload-image-curator/main.go`
- `scripts/workload-image-curator/pyxis/client.go`
- `scripts/workload-image-curator/registry/validator.go`
- `scripts/workload-image-curator/output/formatter.go`

### Phase 2: Concurrency & Performance
**Goal:** Add parallel processing for speed improvements

**Documentation:**
- [x] Testing guide: `scripts/workload-image-curator/docs/phase2-testing-guide.md`

**Deliverables:**
- [x] Worker pool for API queries (10 concurrent)
- [x] Worker pool for image inspection (10 concurrent)
- [x] Worker pool for downloads (5 concurrent, configurable via --workers flag)
- [x] Rate limiting (token bucket)
- [x] Progress reporting for API and inspection workers (real-time, stderr with adaptive frequency)
- [x] Per-worker progress reporting for download goroutines (track each worker's image/status)
- [x] Cancellation support (Ctrl+C graceful shutdown)
- [x] Performance benchmarks vs Python
- [x] Skopeo wrapper with error handling and fallbacks

**Acceptance Criteria:**
- 5-10x faster than Python for 50+ images
- No race conditions (verified with `-race` flag)
- Graceful shutdown on SIGINT/SIGTERM

**Critical Files:**
- `scripts/workload-image-curator/downloader/worker_pool.go`
- `scripts/workload-image-curator/downloader/progress.go`
- `scripts/workload-image-curator/pyxis/pagination.go`

### Phase 3: Size-Based Categorization
**Goal:** Intelligent image selection based on size

**Deliverables:**
- [x] Size metadata extraction from registry manifests
- [x] Categorization engine (Small/Medium/Large)
- [x] Absolute threshold mode (bytes)
- [x] Distribution strategy configuration
- [x] Image selection algorithm
- [x] Unit tests for categorization logic

**Documentation:**
- [x] Testing guide: `scripts/workload-image-curator/docs/phase3-testing-guide.md`

**Acceptance Criteria:**
- Accurate size extraction (within 1% of actual)
- Correct categorization with default and custom thresholds
- Distribution strategy enforced

**Critical Files:**
- `scripts/workload-image-curator/registry/inspector.go`
- `scripts/workload-image-curator/categorizer/size.go`
- `scripts/workload-image-curator/categorizer/thresholds.go`

### Phase 4: Registry Pre-Assessment
**Goal:** Skip redundant downloads, optimize workflow

**Deliverables:**
- [ ] Target registry inspection (digest lookup)
- [ ] Existing image categorization
- [ ] Remaining calculation per category
- [ ] Image selection to satisfy remaining strategy
- [ ] Early exit if strategy satisfied
- [ ] Summary report for pre-assessment results

**Documentation:**
- [ ] Testing guide: `scripts/workload-image-curator/docs/phase4-testing-guide.md`

**Acceptance Criteria:**
- Correctly identifies existing images by digest
- Skips already-present images
- Exits early when no downloads needed

**Critical Files:**
- `scripts/workload-image-curator/registry/mirror.go`
- `scripts/workload-image-curator/strategy/planner.go`
- `scripts/workload-image-curator/strategy/selector.go`

### Phase 5: Hardening & Documentation
**Goal:** Production readiness and team enablement

**Deliverables:**
- [ ] Comprehensive error handling
- [ ] Structured logging (configurable verbosity)
- [ ] Memory optimization
- [ ] Makefile (build, test, install targets)
- [ ] Integration tests (end-to-end)
- [ ] Ansible role update documentation
- [ ] Migration guide (Python → Go)
- [ ] CLI reference documentation
- [ ] Performance tuning guide

**Documentation:**
- [ ] Testing guide: `scripts/workload-image-curator/docs/phase5-testing-guide.md`

**Acceptance Criteria:**
- No memory leaks (verified with profiling)
- All error paths tested
- Documentation complete
- Ansible team trained on new tool

### Phase 6: Advanced Features (Future)
**Possible enhancements:**
- [ ] Multi-architecture image support
- [ ] Image filtering by labels/annotations
- [ ] Custom validation hooks
- [ ] Prometheus metrics export
- [ ] Resume capability (checkpoint/restart)
- [ ] Container image packaging (UBI-based)

## Build & Installation

```bash
# Development
cd scripts/workload-image-curator
make build          # Creates ./workload-image-curator binary
make test           # Run unit tests
make install        # Installs to ../workload-image-curator

# Usage
../workload-image-curator --count 10 --stdout
```

## Dependencies

### Go Packages
- `golang.org/x/time/rate` - Rate limiting
- `golang.org/x/sync/errgroup` - Error group management
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management

### External Tools
- `skopeo` - Image operations (must be in PATH)

### Runtime Requirements
- Go 1.21+ for building
- Linux x86_64 (primary target)
- Network access to Pyxis API and Quay.io
- Optional: Pull secret at `/opt/registry/pull-secret-bastion.txt`

## Testing Strategy

### Unit Tests
- API client retry logic
- Categorization algorithms
- Threshold calculation (absolute/percentile)
- Worker pool lifecycle
- Rate limiting behavior

### Integration Tests
- Real Pyxis API queries (rate-limited)
- Skopeo inspect against public registries
- Size extraction accuracy
- End-to-end workflow with test registry

### Compatibility Tests
- JSON output schema match with Python
- TXT format match with Python
- Environment variable behavior
- CLI flag equivalence
- Exit code compatibility

### Performance Tests
- Load test with 500+ images
- Memory profiling
- Concurrency stress test
- Rate limiting verification

## Success Metrics

- **Performance:** 5-10x faster than Python for 50+ images
- **Reliability:** 99% success rate for accessible images
- **Efficiency:** Pre-assessment reduces redundant downloads by 50%+
- **Compatibility:** 100% output format compatibility with Python
- **Adoption:** Ansible team successfully uses Go tool within 1 sprint

## Verification

End-to-end verification after implementation:

```bash
# 1. Basic functionality
operator-image-curator -c 10 --stdout | wc -l  # Should output 10

# 2. Size-based strategy
operator-image-curator \
  --strategy "small:3,medium:5,large:2" \
  -v \
  --output-prefix test-run

# 3. Pre-assessment (run twice)
operator-image-curator --strategy "small:5,medium:5,large:0" --dest-registry bastion:5000
operator-image-curator --strategy "small:5,medium:5,large:0" --dest-registry bastion:5000
# Second run should detect existing images

# 4. Ansible integration
cd ansible
ansible-playbook -i inventory/cloud30.local curate-images-for-acs-testing.yml \
  --extra-vars "acs_image_distribution=small:10,medium:20,large:5"
```

## Files to Update

**New files:**
- `scripts/workload-image-curator/` - All Go source code
- `DESIGN-workload-image-curator.md` - This document

**Files to update (Phase 5):**
- `ansible/roles/operator-container-images-curator/tasks/main.yml` - Update to use Go binary
- `ansible/roles/operator-container-images-curator/defaults/main.yml` - Add new variables
- `docs/acs-operator-workload-image-curator.md` - Update documentation

**No changes to Python implementation** - Keep for backward compatibility and comparison testing.
